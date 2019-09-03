package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/sfn"
	"github.com/aws/aws-sdk-go/service/sqs"
	elasticsearch "github.com/elastic/go-elasticsearch/v6"
	"github.com/elastic/go-elasticsearch/v6/esapi"
	"github.com/elastic/go-elasticsearch/v6/estransport"
	"github.com/kardianos/osext"
	otaws "github.com/opentracing-contrib/go-aws-sdk"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"

	counter "github.com/Clever/aws-sdk-go-counter"
	"github.com/Clever/workflow-manager/executor"
	"github.com/Clever/workflow-manager/executor/sfncache"
	"github.com/Clever/workflow-manager/gen-go/server"
	dynamodbgen "github.com/Clever/workflow-manager/gen-go/server/db/dynamodb"
	dynamodbstore "github.com/Clever/workflow-manager/store/dynamodb"
	"gopkg.in/Clever/kayvee-go.v6/logger"
)

var dynamoMaxRetries = 4

// Config contains the configuration for the workflow-manager app
type Config struct {
	DynamoPrefixStateResources      string
	DynamoPrefixWorkflowDefinitions string
	DynamoPrefixWorkflows           string
	DynamoRegion                    string
	SFNRegion                       string
	SFNAccountID                    string
	SFNRoleARN                      string
	SQSRegion                       string
	SQSQueueURL                     string
	ESURL                           string
}

func setupRouting() {
	dir, err := osext.ExecutableFolder()
	if err != nil {
		log.Fatal(err)
	}
	err = logger.SetGlobalRouting(path.Join(dir, "kvconfig.yml"))
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	addr := flag.String("addr", ":8080", "Address to listen at")
	flag.Parse()

	c := loadConfig()
	setupRouting()

	svc := dynamodb.New(session.Must(session.NewSessionWithOptions(session.Options{
		// reducing MaxRetries to 2 (from 10) to avoid long backoffs when writes fail
		Config: aws.Config{Region: aws.String(c.DynamoRegion), MaxRetries: &dynamoMaxRetries},
	})))
	db := dynamodbstore.New(svc, dynamodbstore.TableConfig{
		PrefixStateResources:      c.DynamoPrefixStateResources,
		PrefixWorkflowDefinitions: c.DynamoPrefixWorkflowDefinitions,
		PrefixWorkflows:           c.DynamoPrefixWorkflows,
	})
	var err error
	db.Future, err = dynamodbgen.New(dynamodbgen.Config{
		DynamoDBAPI:   svc,
		DefaultPrefix: c.DynamoPrefixWorkflowDefinitions,
		WorkflowDefinitionTable: dynamodbgen.WorkflowDefinitionTable{
			Prefix: c.DynamoPrefixWorkflowDefinitions,
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	sfnsess := session.New()
	counter := counter.New()
	sfnsess.Handlers.Send.PushFront(counter.SessionHandler)
	countedSFNAPI := sfn.New(sfnsess, aws.NewConfig().WithRegion(c.SFNRegion))
	cachedSFNAPI, err := sfncache.New(countedSFNAPI)
	if err != nil {
		log.Fatal(err)
	}

	sqsapi := sqs.New(session.New(), aws.NewConfig().WithRegion(c.SQSRegion))

	// Add OpenTracing Handlers
	// Note that Dynamo has automatically OpenTracing through wag's dynamo code
	otaws.AddOTHandlers(countedSFNAPI.Client)
	otaws.AddOTHandlers(sqsapi.Client)

	wfmSFN := executor.NewSFNWorkflowManager(cachedSFNAPI, sqsapi, db, c.SFNRoleARN, c.SFNRegion, c.SFNAccountID, c.SQSQueueURL)

	es, err := elasticsearch.NewClient(elasticsearch.Config{Addresses: []string{c.ESURL}})
	if err != nil {
		log.Fatal(err)
	}
	es = tracedClient(es)

	h := Handler{
		store:     db,
		manager:   wfmSFN,
		es:        es,
		deployEnv: mustGetenv("_DEPLOY_ENV"),
	}
	timeout := 5 * time.Second
	s := server.NewWithMiddleware(h, *addr, []func(http.Handler) http.Handler{
		func(handler http.Handler) http.Handler {
			return http.TimeoutHandler(handler, timeout, "Request timed out")
		},
		func(handler http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				newCtx, cancel := context.WithTimeout(r.Context(), timeout)
				defer cancel()
				r = r.WithContext(newCtx)
				handler.ServeHTTP(w, r)
			})
		},
	})

	go executor.PollForPendingWorkflowsAndUpdateStore(context.Background(), wfmSFN, db, sqsapi, c.SQSQueueURL)
	go logSFNCounts(counter)

	if err := s.Serve(); err != nil {
		log.Fatal(err)
	}

	log.Println("workflow-manager exited without error")
}

func awsSession(c Config) *session.Session {
	options := session.Options{
		Config:            aws.Config{Region: aws.String("us-east-1")},
		SharedConfigState: session.SharedConfigEnable,
	}

	return session.Must(session.NewSessionWithOptions(options))
}

func loadConfig() Config {
	return Config{
		DynamoPrefixStateResources: getEnvVarOrDefault(
			"AWS_DYNAMO_PREFIX_STATE_RESOURCES",
			"workflow-manager-test",
		),
		DynamoPrefixWorkflowDefinitions: getEnvVarOrDefault(
			"AWS_DYNAMO_PREFIX_WORKFLOW_DEFINITIONS",
			"workflow-manager-test",
		),
		DynamoPrefixWorkflows: getEnvVarOrDefault(
			"AWS_DYNAMO_PREFIX_WORKFLOWS",
			"workflow-manager-test",
		),
		DynamoRegion: mustGetenv("AWS_DYNAMO_REGION"),
		SFNRegion:    mustGetenv("AWS_SFN_REGION"),
		SFNAccountID: mustGetenv("AWS_SFN_ACCOUNT_ID"),
		SFNRoleARN:   mustGetenv("AWS_SFN_ROLE_ARN"),
		SQSRegion:    mustGetenv("AWS_SQS_REGION"),
		SQSQueueURL:  mustGetenv("AWS_SQS_URL"),
		ESURL:        mustGetenv("ES_URL"),
	}
}

func mustGetenv(envVarName string) string {
	v := os.Getenv(envVarName)
	if v == "" {
		log.Fatalf("%s required", envVarName)
	}
	return v
}

func getEnvVarOrDefault(envVarName, defaultIfEmpty string) string {
	value := os.Getenv(envVarName)
	if value == "" {
		value = defaultIfEmpty
	}

	return value
}

func logSFNCounts(sfnCounter *counter.Counter) {
	ticker := time.NewTicker(30 * time.Second)
	for range ticker.C {
		executor.LogSFNCounts(sfnCounter.Counters())
	}
}

// Below provides a thin wrapper around the elasticsearch Client with opentracing added in
// The Client consists of a Transport object which handles the http requests, and an API object
//   which makes calls to the Transport. We take the old transport, wrap it with the tracing,
//   then build a new API object from it.

type tracedESTransport struct {
	child estransport.Interface
}

func (t tracedESTransport) Perform(req *http.Request) (*http.Response, error) {
	span, ctx := opentracing.StartSpanFromContext(req.Context(), "elasticsearch-request")
	defer span.Finish()

	// These fields mirror the ones in the aws-sdk-go opentracing package
	ext.SpanKindRPCClient.Set(span)
	ext.Component.Set(span, "go-elasticsearch")
	ext.HTTPMethod.Set(span, req.Method)
	ext.HTTPUrl.Set(span, req.URL.String())
	ext.PeerService.Set(span, "elasticsearch")

	req = req.WithContext(ctx)
	resp, err := t.child.Perform(req)

	if err != nil {
		ext.Error.Set(span, true)
	} else {
		ext.HTTPStatusCode.Set(span, uint16(resp.StatusCode))
	}

	return resp, err
}

func tracedClient(client *elasticsearch.Client) *elasticsearch.Client {

	tracedTransport := tracedESTransport{
		child: client.Transport,
	}
	return &elasticsearch.Client{
		API:       esapi.New(tracedTransport),
		Transport: tracedTransport,
	}
}
