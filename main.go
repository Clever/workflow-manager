package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
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
	AWSProfile string // profile should only be set for local runs
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

	// for local runs use profile and MFA
	if c.AWSProfile != "" {
		fmt.Printf("Using profile %s\n", c.AWSProfile)
		options.AssumeRoleTokenProvider = stscreds.StdinTokenProvider
		options.Profile = c.AWSProfile
	}

	sess := session.Must(session.NewSessionWithOptions(options))

	return sess
}

func loadConfig() Config {
	region := os.Getenv("AWS_REGION")
	profile := os.Getenv("AWS_PROFILE")
	queue := os.Getenv("BATCH_QUEUE")

	if region == "" {
		region = "us-east-1"
	}
	if queue == "" {
		queue = "default"
	}

	return Config{
		AWSRegion:  region,
		AWSProfile: profile,
		BatchQueue: queue,
	}
}
