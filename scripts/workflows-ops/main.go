package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Clever/wag/clientconfig/v9"
	"github.com/davecgh/go-spew/spew"

	"github.com/Clever/workflow-manager/gen-go/client"
	"github.com/Clever/workflow-manager/gen-go/models"
)

// quotas defined https://docs.aws.amazon.com/step-functions/latest/dg/limits-overview.html#service-limits-api-action-throttling-general
const (
	// DescribeExecution limit is 15/s per account per region. So we need to share it with regular workflow-manager operations. Which is why we opt to use 1/3 of that i.e 5/s.
	rateLimitDescribe = 5

	// StopExecution limit is 200/s per account per region. Generally there are not many calls to stop execution as part of normal operations so we use 180/s to be safe.
	rateLimitStop = 180

	// This is the number of workers processing the queue
	// The queue is populated according to the rate limit
	concurrency = 50
)

// API params
var (
	limit          int64 = 50
	oldestFirst          = true
	resolvedByUser       = false
)

// Progress metrics
var (
	reportSuccessRate       = 5 * time.Second
	successCount      int64 = 0
)

type cmdFlags struct {
	workflow     string
	cmd          string
	status       string
	cancelReason string
}

var (
	workflow     = flag.String("workflow", "", "workflow name, for example multiverse:master")
	cmd          = flag.String("cmd", "", "cmd. Can be set to \"cancel\" or \"refresh\"")
	cancelReason = flag.String("cancel-reason", "", "reason for canceling the workflow")
	status       = flag.String("status", "", "status of the workflow to filter by, for example queued or running")
)

func main() {
	flag.Parse()
	flags := cmdFlags{
		workflow:     *workflow,
		cmd:          *cmd,
		status:       *status,
		cancelReason: *cancelReason,
	}

	if flags.workflow == "" || flags.cmd == "" || flags.status == "" {
		log.Fatal("please set a workflow name, cmd, and status")
	}

	if flags.cmd != "cancel" && flags.cmd != "refresh" {
		log.Fatal("cmd must be set to \"cancel\" or \"refresh\"")
	}

	if flags.cmd == "cancel" && flags.cancelReason == "" {
		log.Fatal("cancel-reason must be set when cmd is set to \"cancel\"")
	}

	if flags.status != "queued" && flags.status != "running" {
		log.Fatal("status must be set to \"queued\" or \"running\"")
	}

	spew.Dump(flags)

	CheckForContinue()

	os.Setenv("SERVICE_WORKFLOW_MANAGER_HTTP_HOST", "production--workflow-manager--8e75288e.int.clever.com")
	os.Setenv("SERVICE_WORKFLOW_MANAGER_HTTP_PORT", "443")
	os.Setenv("SERVICE_WORKFLOW_MANAGER_HTTP_PROTO", "https")
	cl, err := client.NewFromDiscovery(clientconfig.WithoutTracing("workflow-manager"))
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	itr, err := cl.NewGetWorkflowsIter(ctx, &models.GetWorkflowsInput{
		Limit:                  &limit,
		OldestFirst:            &oldestFirst,
		Status:                 &flags.status,
		ResolvedByUser:         &resolvedByUser,
		WorkflowDefinitionName: flags.workflow,
	})
	if err != nil {
		log.Fatalf("failed to init iterator: %v", err)
	}

	workc := make(chan string, 50)
	for i := 0; i < concurrency; i++ {
		go worker(ctx, cl, flags, workc)
		log.Printf("%d/%d workers started", i+1, concurrency)
	}

	go func() {
		for range time.Tick(reportSuccessRate) {
			log.Printf("success count: %d", atomic.LoadInt64(&successCount))
			log.Printf("work queue size: %d", len(workc))
		}
	}()

	rateLimit := time.Second / rateLimitDescribe
	if flags.cmd == "cancel" {
		rateLimit = time.Second / rateLimitStop
	}

	limiter := time.Tick(rateLimit)
	wf := models.Workflow{}
	log.Println("queuing work...")
	for itr.Next(&wf) {
		// block on the rate limiter
		<-limiter
		workc <- wf.ID
	}

	if err := itr.Err(); err != nil {
		log.Fatalf("iterator error: %v", err)
	} else {
		log.Println("successfully processed all records")
	}
}

func worker(ctx context.Context, cl *client.WagClient, flags cmdFlags, work <-chan string) {
	for id := range work {
		var err error
		var r *models.Workflow
		if flags.cmd == "cancel" {
			err = cl.CancelWorkflow(ctx, &models.CancelWorkflowInput{
				WorkflowID: id,
				Reason:     &models.CancelReason{Reason: flags.cancelReason},
			})
		} else if flags.cmd == "refresh" {
			r, err = cl.GetWorkflowByID(ctx, &models.GetWorkflowByIDInput{WorkflowID: id})
		}

		if err != nil {
			log.Printf("%s(%s): %v", flags.cmd, id, err)
		} else if flags.cmd == "refresh" && r.Status == models.WorkflowStatus(flags.status) {
			log.Printf("status unchanged ID = %s: %s", r.ID, r.Status)
		} else {
			atomic.AddInt64(&successCount, 1)
		}
	}
}

func CheckForContinue() {
	fmt.Println("Continue? (y/n)")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	text := strings.TrimSpace(scanner.Text())
	if err := scanner.Err(); err != nil {
		log.Fatalf("scanner error: %s", err)
	}
	if strings.ToLower(text) != "y" {
		log.Fatalf("Exiting...")
	}
}
