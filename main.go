package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"path"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/batch"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/sfn"
	"github.com/kardianos/osext"

	"github.com/Clever/kayvee-go/logger"
	"github.com/Clever/workflow-manager/executor"
	"github.com/Clever/workflow-manager/executor/batchclient"
	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/Clever/workflow-manager/gen-go/server"
	dynamodbstore "github.com/Clever/workflow-manager/store/dynamodb"
)

// Config contains the configuration for the workflow-manager app
type Config struct {
	AWSRegion                       string
	DefaultBatchQueue               string
	CustomBatchQueues               map[string]string
	DynamoPrefixStateResources      string
	DynamoPrefixWorkflowDefinitions string
	DynamoPrefixWorkflows           string
	DynamoRegion                    string
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
		Config: aws.Config{Region: aws.String(c.DynamoRegion)},
	})))
	db := dynamodbstore.New(svc, dynamodbstore.TableConfig{
		PrefixStateResources:      c.DynamoPrefixStateResources,
		PrefixWorkflowDefinitions: c.DynamoPrefixWorkflowDefinitions,
		PrefixWorkflows:           c.DynamoPrefixWorkflows,
	})
	batch := batchclient.NewBatchExecutor(batch.New(awsSession(c)), c.DefaultBatchQueue, c.CustomBatchQueues)
	wfmBatch := executor.NewBatchWorkflowManager(batch, db)
	// TODO: config all of these
	sfnRegion := "us-west-2"
	sfnRole := "arn:aws:iam::589690932525:role/raf-test-step-functions"
	sfnAccountID := "589690932525"
	sfnapi := sfn.New(session.New(), aws.NewConfig().WithRegion(sfnRegion))
	wfmSFN := executor.NewSFNWorkflowManager(sfnapi, db, sfnRole, sfnRegion, sfnAccountID)
	wfmMulti := executor.NewMultiWorkflowManager(wfmBatch, map[models.Manager]executor.WorkflowManager{
		models.ManagerBatch:         wfmBatch,
		models.ManagerStepFunctions: wfmSFN,
	})
	h := Handler{
		store:   db,
		manager: wfmMulti,
	}
	s := server.New(h, *addr)

	go executor.PollForPendingWorkflowsAndUpdateStore(context.Background(), wfmMulti, db)

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
	customQueues := os.Getenv("CUSTOM_BATCH_QUEUES")
	customQueuesMap, err := parseCustomQueues(customQueues)
	if err != nil {
		log.Fatal(err)
	}

	return Config{
		AWSRegion:         getEnvVarOrDefault("AWS_REGION", "us-east-1"),
		DefaultBatchQueue: getEnvVarOrDefault("DEFAULT_BATCH_QUEUE", "default"),
		CustomBatchQueues: customQueuesMap,
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
	}
}

func parseCustomQueues(s string) (map[string]string, error) {
	if s == "" {
		return map[string]string{}, nil

	}
	var output map[string]string
	if err := json.Unmarshal([]byte(s), &output); err != nil {
		return map[string]string{}, err
	}
	return output, nil
}

func getEnvVarOrDefault(envVarName, defaultIfEmpty string) string {
	value := os.Getenv(envVarName)
	if value == "" {
		value = defaultIfEmpty
	}

	return value
}
