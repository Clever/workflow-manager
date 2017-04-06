package main

import (
	"flag"
	"log"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/batch"

	"github.com/Clever/workflow-manager/executor"
	"github.com/Clever/workflow-manager/executor/batchclient"
	"github.com/Clever/workflow-manager/gen-go/server"
	"github.com/Clever/workflow-manager/store"
)

// Config contains the configuration for the workflow-manager app
type Config struct {
	AWSRegion  string
	BatchQueue string
}

func main() {
	addr := flag.String("addr", ":8080", "Address to listen at")
	flag.Parse()

	c := loadConfig()
	db := store.NewMemoryStore()
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

	if region == "" {
		region = "us-east-1"
	}
	if queue == "" {
		queue = "default"
	}

	return Config{
		AWSRegion:  region,
		BatchQueue: queue,
	}
}
