package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	"github.com/Clever/workflow-manager/embedded"
	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sfn"
)

const sfnRegion = "us-east-2"

var sfnAccountID = strings.Replace(os.Getenv("AWS_ACCOUNT_NUMBER"), "-", "", -1)

type Result struct {
	Message string
}

func first(ctx context.Context) (Result, error) {
	return Result{"world"}, nil
}

func second(ctx context.Context, input Result) (Result, error) {
	return Result{"Hello, " + input.Message + "!"}, nil
}

func main() {
	ctx := context.Background()
	wfdefs, err := ioutil.ReadFile("./workflowdefinitions.yml")
	if err != nil {
		log.Fatal(err)
	}
	client, err := embedded.New(&embedded.Config{
		Environment:  "clever-dev",
		App:          "example",
		SFNAccountID: sfnAccountID,
		SFNRegion:    sfnRegion,
		SFNRoleArn:   "arn:aws:iam::589690932525:role/raf-test-step-functions",
		SFNAPI: sfn.New(session.New(&aws.Config{
			Region: aws.String(sfnRegion),
		})),
		Resources: map[string]interface{}{
			"first":  first,
			"second": second,
		},
		WorkflowDefinitions: wfdefs,
	})
	if err != nil {
		log.Fatal(err)
	}
	go client.PollForWork(ctx)
	time.Sleep(5 * time.Second)
	wf, err := client.StartWorkflow(ctx, &models.StartWorkflowRequest{
		Input:              "{}",
		Namespace:          "clever-dev",
		Queue:              "mainqueue",
		Tags:               map[string]interface{}{},
		WorkflowDefinition: &models.WorkflowDefinitionRef{Name: "hello-world"},
	})
	if err != nil {
		log.Fatalf("start workflow: %s", err.Error())
	}

	wfbs, _ := json.MarshalIndent(wf, "", "  ")
	fmt.Println("started workflow", string(wfbs))
	fmt.Println("ctrl-c to exit")
	select {}
}
