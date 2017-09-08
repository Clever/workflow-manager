package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"path"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/batch"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/kardianos/osext"

	"github.com/Clever/kayvee-go/logger"
	"github.com/Clever/workflow-manager/executor"
	"github.com/Clever/workflow-manager/executor/batchclient"
	"github.com/Clever/workflow-manager/gen-go/server"
	dynamodbstore "github.com/Clever/workflow-manager/store/dynamodb"
)

// Config contains the configuration for the workflow-manager app
type Config struct {
	AWSRegion         string
	DefaultBatchQueue string
	CustomBatchQueues map[string]string
	DynamoRegion      string
	DynamoPrefix      string
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
	db := dynamodbstore.New(svc, c.DynamoPrefix)
	batch := batchclient.NewBatchExecutor(batch.New(awsSession(c)), c.DefaultBatchQueue, c.CustomBatchQueues)

	ctrl := controller{
		store:   db,
		manager: executor.NewBatchJobManager(batch, db),
	}
	s := server.New(ctrl, *addr)

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
	region := os.Getenv("AWS_REGION")
	defaultQueue := os.Getenv("DEFAULT_BATCH_QUEUE")
	customQueues := os.Getenv("CUSTOM_BATCH_QUEUES")
	dynamoPrefix := os.Getenv("AWS_DYNAMO_PREFIX")
	dynamoRegion := os.Getenv("AWS_DYNAMO_REGION")

	if region == "" {
		region = "us-east-1"
	}

	if defaultQueue == "" {
		defaultQueue = "default"
	}

	customQueuesMap, err := parseCustomQueues(customQueues)
	if err != nil {
		log.Fatal(err)
	}

	if dynamoPrefix == "" {
		dynamoPrefix = "workflow-manager-test"
	}

	return Config{
		AWSRegion:         region,
		DefaultBatchQueue: defaultQueue,
		CustomBatchQueues: customQueuesMap,
		DynamoPrefix:      dynamoPrefix,
		DynamoRegion:      dynamoRegion,
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
