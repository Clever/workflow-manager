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
	"github.com/kardianos/osext"

	"github.com/Clever/aws-sdk-go-counter/counter/sfncounter"
	"github.com/Clever/workflow-manager/executor"
	"github.com/Clever/workflow-manager/executor/sfncache"
	"github.com/Clever/workflow-manager/gen-go/server"
	dynamodbgen "github.com/Clever/workflow-manager/gen-go/server/db/dynamodb"
	dynamodbstore "github.com/Clever/workflow-manager/store/dynamodb"
	"gopkg.in/Clever/kayvee-go.v6/logger"
)

var dynamoMaxRetries = 2

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

	sfnapi := sfn.New(session.New(), aws.NewConfig().WithRegion(c.SFNRegion))
	countedSFNAPI := sfncounter.New(sfnapi)
	cachedSFNAPI, err := sfncache.New(countedSFNAPI)
	if err != nil {
		log.Fatal(err)
	}

	sqsapi := sqs.New(session.New(), aws.NewConfig().WithRegion(c.SQSRegion))
	wfmSFN := executor.NewSFNWorkflowManager(cachedSFNAPI, sqsapi, db, c.SFNRoleARN, c.SFNRegion, c.SFNAccountID, c.SQSQueueURL)
	h := Handler{
		store:   db,
		manager: wfmSFN,
	}
	timeout := 10 * time.Second
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
	go logSFNCounts(countedSFNAPI)

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
		DynamoRegion: os.Getenv("AWS_DYNAMO_REGION"),
		SFNRegion:    os.Getenv("AWS_SFN_REGION"),
		SFNAccountID: os.Getenv("AWS_SFN_ACCOUNT_ID"),
		SFNRoleARN:   os.Getenv("AWS_SFN_ROLE_ARN"),
		SQSRegion:    os.Getenv("AWS_SQS_REGION"),
		SQSQueueURL:  os.Getenv("AWS_SQS_URL"),
	}
}

func getEnvVarOrDefault(envVarName, defaultIfEmpty string) string {
	value := os.Getenv(envVarName)
	if value == "" {
		value = defaultIfEmpty
	}

	return value
}

func logSFNCounts(sfnCounter *sfncounter.SFN) {
	ticker := time.NewTicker(30 * time.Second)
	for range ticker.C {
		executor.LogSFNCounts(sfnCounter.Counters())
	}
}
