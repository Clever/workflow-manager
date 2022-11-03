package main

import (
	"context"
	_ "embed"
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/sfn"
	"github.com/elastic/go-elasticsearch/v6"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	counter "github.com/Clever/aws-sdk-go-counter"
	"github.com/Clever/kayvee-go/v7/logger"
	"github.com/Clever/workflow-manager/executor"
	"github.com/Clever/workflow-manager/executor/sfncache"
	"github.com/Clever/workflow-manager/gen-go/server"
	dynamodbgen "github.com/Clever/workflow-manager/gen-go/server/db/dynamodb"
	tracing "github.com/Clever/workflow-manager/gen-go/servertracing"
	dynamodbstore "github.com/Clever/workflow-manager/store/dynamodb"
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
	ESURL                           string
	ExecutionEventsStreamARN        string
	CWLogsToKinesisRoleARN          string
}

//go:embed kvconfig.yml
var kvconfig []byte

func setupRouting() {
	if err := logger.SetGlobalRoutingFromBytes(kvconfig); err != nil {
		log.Fatalf("Error setting kvconfig: %s", err)
	}
}

func main() {
	addr := flag.String("addr", ":8080", "Address to listen at")
	flag.Parse()

	c := loadConfig()
	setupRouting()

	// Initialize globals for tracing
	if os.Getenv("_TRACING_ENABLED") == "true" {
		if exp, prov, err := tracing.SetupGlobalTraceProviderAndExporter(context.Background()); err != nil {
			log.Fatalf("failed to setup tracing: %v", err)
		} else {
			// Ensure traces are finalized when exiting
			defer exp.Shutdown(context.Background())
			defer prov.Shutdown(context.Background())
		}
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

	cwlogsTransport := tracedTransport("go-aws", "cwlogs", func(operation string, _ *http.Request) string {
		return operation
	})
	cwlogsapi := cloudwatchlogs.New(session.New(), aws.NewConfig().WithRegion(c.SFNRegion).WithHTTPClient(&http.Client{Transport: cwlogsTransport}))

	wfmSFN := executor.NewSFNWorkflowManager(cachedSFNAPI, cwlogsapi, db, c.SFNRoleARN, c.SFNRegion, c.SFNAccountID, c.ExecutionEventsStreamARN, c.CWLogsToKinesisRoleARN)

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
		DynamoRegion:             mustGetenv("AWS_DYNAMO_REGION"),
		SFNRegion:                mustGetenv("AWS_SFN_REGION"),
		SFNAccountID:             mustGetenv("AWS_SFN_ACCOUNT_ID"),
		SFNRoleARN:               mustGetenv("AWS_SFN_ROLE_ARN"),
		ESURL:                    mustGetenv("ES_URL"),
		ExecutionEventsStreamARN: mustGetenv("AWS_SFN_EXECUTION_EVENTS_STREAM_ARN"),
		CWLogsToKinesisRoleARN:   mustGetenv("AWS_IAM_CWLOGS_TO_KINESIS_ROLE_ARN"),
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
