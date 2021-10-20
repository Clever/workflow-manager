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
	"github.com/elastic/go-elasticsearch/v6"
	"github.com/kardianos/osext"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	counter "github.com/Clever/aws-sdk-go-counter"
	"github.com/Clever/workflow-manager/executor"
	"github.com/Clever/workflow-manager/executor/sfncache"
	"github.com/Clever/workflow-manager/gen-go/server"
	dynamodbgen "github.com/Clever/workflow-manager/gen-go/server/db/dynamodb"
	"github.com/Clever/workflow-manager/gen-go/tracing"
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

	// Initialize globals for tracing
	enableTracing := os.Getenv("_TRACING_ENABLED")
	if enableTracing == "true" {
		log.Print("Tracinng is enabled")
		if exp, prov, err := tracing.SetupGlobalTraceProviderAndExporter(context.Background()); err != nil {
			log.Fatalf("failed to setup tracing: %v", err)
		} else {
			// Ensure traces are finalized when exiting
			defer exp.Shutdown(context.Background())
			defer prov.Shutdown(context.Background())
		}
	} else {
		log.Print("tracing is disabled")
	}

	dynamoTransport := tracedTransport("go-aws", "dynamodb", func(operation string, _ *http.Request) string {
		return operation
	})
	svc := dynamodb.New(session.Must(session.NewSessionWithOptions(session.Options{
		// reducing MaxRetries to 2 (from 10) to avoid long backoffs when writes fail
		Config: aws.Config{
			Region:     aws.String(c.DynamoRegion),
			MaxRetries: &dynamoMaxRetries,
			HTTPClient: &http.Client{Transport: dynamoTransport},
		},
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
	sfnTransport := tracedTransport("go-aws", "sfn", func(operation string, _ *http.Request) string {
		return operation
	})
	countedSFNAPI := sfn.New(sfnsess, aws.NewConfig().WithRegion(c.SFNRegion).WithHTTPClient(&http.Client{Transport: sfnTransport}))
	cachedSFNAPI, err := sfncache.New(countedSFNAPI)
	if err != nil {
		log.Fatal(err)
	}

	sqsTransport := tracedTransport("go-aws", "sqs", func(operation string, _ *http.Request) string {
		return operation
	})
	sqsapi := sqs.New(session.New(), aws.NewConfig().WithRegion(c.SQSRegion).WithHTTPClient(&http.Client{Transport: sqsTransport}))

	wfmSFN := executor.NewSFNWorkflowManager(cachedSFNAPI, sqsapi, db, c.SFNRoleARN, c.SFNRegion, c.SFNAccountID, c.SQSQueueURL)

	es, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{c.ESURL},
		Transport: tracedTransport("go-elasticsearch", "elasticsearch", func(_ string, _ *http.Request) string {
			return "elasticsearch-request"
		}),
	})
	if err != nil {
		log.Fatal(err)
	}

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

// This can be used when more detailed instrumentation is not available.
// For example, the go-elasticsearch doesn't have instrumentation, and as of writing, aws-sdk-go doesn't.
// aws-sdk-go-v2 may get instrumentation eventually, but v1 is unlikely to get it.
func tracedTransport(component string, peerService string, spanNamer func(operation string, req *http.Request) string) http.RoundTripper {
	return otelhttp.NewTransport(http.DefaultTransport,
		otelhttp.WithSpanNameFormatter(spanNamer),
		otelhttp.WithSpanOptions(
			trace.WithSpanKind(trace.SpanKindClient),
			trace.WithAttributes(attribute.String("peer.service", peerService), attribute.String("component", component)),
		),
	)
}
