package main

import (
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
	AWSRegion    string
	BatchQueue   string
	DynamoRegion string
	DynamoPrefix string
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
	batch :=
		batchclient.NewBatchExecutor(batch.New(awsSession(c)), c.BatchQueue)

	wm := WorkflowManager{
		store:   db,
		manager: executor.NewBatchJobManager(batch, db),
	}
	s := server.New(wm, *addr)

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
	queue := os.Getenv("BATCH_QUEUE")
	dynamoPrefix := os.Getenv("AWS_DYNAMO_PREFIX")
	dynamoRegion := os.Getenv("AWS_DYNAMO_REGION")

	if region == "" {
		region = "us-east-1"
	}
	if queue == "" {
		queue = "default"
	}
	if dynamoPrefix == "" {
		dynamoPrefix = "workflow-manager-test"
	}

	return Config{
		AWSRegion:    region,
		BatchQueue:   queue,
		DynamoPrefix: dynamoPrefix,
		DynamoRegion: dynamoRegion,
	}
}
