package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sfn"

	"github.com/Clever/workflow-manager/embedded"
	"github.com/Clever/workflow-manager/gen-go/models"
)

const sfnRegion = "us-east-2"

var sfnAccountID = strings.ReplaceAll(os.Getenv("AWS_ACCOUNT_NUMBER"), "-", "")

func first(ctx context.Context, input string) (string, error) {
	log.Printf("in first before sleep for %s", input)
	time.Sleep(20 * time.Second)
	log.Printf("in first after sleep for %s", input)
	return input, nil
}

func second(input string) (string, error) {
	log.Printf("in second before sleep for %s", input)
	time.Sleep(20 * time.Second)
	log.Printf("in second after sleep for %s", input)
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
	time.Sleep(2 * time.Second)
	wf1, err := client.StartWorkflow(ctx, &models.StartWorkflowRequest{
		Input: `"first wf"`,
		WorkflowDefinition: &models.WorkflowDefinitionRef{
			Name: "hello-world-v4",
		},
	})
	if err != nil {
		log.Fatalf("start workflow: %s", err.Error())
	}

	wf2, err := client.StartWorkflow(ctx, &models.StartWorkflowRequest{
		Input: `"second wf"`,

		WorkflowDefinition: &models.WorkflowDefinitionRef{
			Name: "hello-world-v4",
		},
	})
	if err != nil {
		log.Fatalf("start workflow 2: %s", err.Error())
	}

	// wfbs, _ := json.MarshalIndent(wf, "", "  ")
	// fmt.Println("started workflow", string(wfbs))

	for {
		time.Sleep(2 * time.Second)
		wf, err := client.GetWorkflowByID(ctx, &models.GetWorkflowByIDInput{WorkflowID: wf1.ID})
		if err != nil {
			fmt.Println("Oops, err", err)
			break
		}
		// bs, _ := json.MarshalIndent(wf, "", "  ")
		// fmt.Println("got workflow", string(bs))
		if wf.Status == models.WorkflowStatusFailed ||
			wf.Status == models.WorkflowStatusSucceeded ||
			wf.Status == models.WorkflowStatusCancelled {
			break
		}
	}
	for {
		time.Sleep(2 * time.Second)
		wf, err := client.GetWorkflowByID(ctx, &models.GetWorkflowByIDInput{WorkflowID: wf2.ID})
		if err != nil {
			fmt.Println("Oops, err", err)
			break
		}
		// bs, _ := json.MarshalIndent(wf, "", "  ")
		// fmt.Println("got workflow", string(bs))
		if wf.Status == models.WorkflowStatusFailed ||
			wf.Status == models.WorkflowStatusSucceeded ||
			wf.Status == models.WorkflowStatusCancelled {
			break
		}
	}
	time.Sleep(1 * time.Second)
}
