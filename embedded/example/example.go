package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Clever/workflow-manager/embedded"
	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sfn"
)

const sfnRegion = "us-east-2"

var sfnAccountID = strings.Replace(os.Getenv("AWS_ACCOUNT_NUMBER"), "-", "", -1)

func first(ctx context.Context) (string, error) {
	return "world", nil
}

func second(input string) (string, error) {
	return "Hello, " + input + "!", nil
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Signal(syscall.SIGTERM))
	go func() {
		for range c {
			// sig is a ^C, handle it
			cancel()
		}
	}()

	wfdefs, err := ioutil.ReadFile("./workflowdefinitions.yml")
	if err != nil {
		log.Fatal(err)
	}
	client, err := embedded.New(&embedded.Config{
		Environment:  "clever-dev",
		App:          "example",
		SFNAccountID: sfnAccountID,
		SFNRegion:    sfnRegion,
		SFNRoleArn:   "arn:aws:iam::589690932525:role/raf-test-step-fjunctions",
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
	time.Sleep(2 * time.Second)
	wf, err := client.StartWorkflow(ctx, &models.StartWorkflowRequest{
		Input: "{}",
		WorkflowDefinition: &models.WorkflowDefinitionRef{
			Name: "hello-world-v2",
		},
	})
	if err != nil {
		log.Fatalf("start workflow: %s", err.Error())
	}

	wfbs, _ := json.MarshalIndent(wf, "", "  ")
	fmt.Println("started workflow", string(wfbs))

	for {
		time.Sleep(2 * time.Second)
		wf, err := client.GetWorkflowByID(ctx, wf.ID)
		if err != nil {
			fmt.Println("Oops, err", err)
			break
		}
		bs, _ := json.MarshalIndent(wf, "", "  ")
		fmt.Println("got workflow", string(bs))
		if wf.Status == models.WorkflowStatusFailed ||
			wf.Status == models.WorkflowStatusSucceeded ||
			wf.Status == models.WorkflowStatusCancelled {
			break
		}
	}
	cancel()
	time.Sleep(1 * time.Second)
}
