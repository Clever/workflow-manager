package main

import (
	"context"
	"log"
	"os"
	"sync/atomic"
	"time"

	"github.com/Clever/wag/clientconfig/v9"

	"github.com/Clever/workflow-manager/gen-go/client"
	"github.com/Clever/workflow-manager/gen-go/models"
)

// This script cancels all work from a specific workflow definition.
// It works by fetching all workflows in a given state (usually running or queued),
// then looping through and canceling each individual workflow.

// Knobs to turn
const (
	// Note that we are limited by the AWS SFN API Quotas. The stop
	// execution limit is 200/s per account per region. We need to share
	// with regular application functionality.
	// A CancelWorkflow api call will be queued on this interval.
	rateLimit = 1 * time.Millisecond
	// This is the number of workers processing the queue
	// The queue is populated according to the rate limit
	concurrency = 50
)

// API params
var (
	limit          int64 = 50
	oldestFirst          = false
	status               = string(models.JobStatusRunning)
	resolvedByUser       = false
	wfName               = "multiverse:master"
)

// Progress metrics
var (
	reportSuccessRate       = 5 * time.Second
	successCount      int64 = 0
)

func main() {
	os.Setenv("SERVICE_WORKFLOW_MANAGER_HTTP_HOST", "production--workflow-manager--a6127c9c.int.clever.com")
	os.Setenv("SERVICE_WORKFLOW_MANAGER_HTTP_PORT", "443")
	os.Setenv("SERVICE_WORKFLOW_MANAGER_HTTP_PROTO", "https")
	cl, err := client.NewFromDiscovery(clientconfig.WithoutTracing("workflow-manager"))
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	if wfName == "" {
		log.Fatal("please set a workflow name")
	}
	itr, err := cl.NewGetWorkflowsIter(ctx, &models.GetWorkflowsInput{
		Limit:                  &limit,
		OldestFirst:            &oldestFirst,
		Status:                 &status,
		ResolvedByUser:         &resolvedByUser,
		WorkflowDefinitionName: wfName,
	})
	if err != nil {
		log.Fatalf("failed to init iterator: %v", err)
	}

	workc := make(chan string, 200)
	for i := 0; i < concurrency; i++ {
		go worker(ctx, cl, workc)
		log.Printf("%d/%d workers started", i+1, concurrency)
	}

	go func() {
		for range time.Tick(reportSuccessRate) {
			log.Printf("success count: %d", atomic.LoadInt64(&successCount))
			log.Printf("work queue size: %d", len(workc))
		}
	}()

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

func worker(ctx context.Context, cl *client.WagClient, work <-chan string) {
	for id := range work {
		err := cl.CancelWorkflow(ctx, &models.CancelWorkflowInput{WorkflowID: id, Reason: &models.CancelReason{Reason: "canceled by script for FLARE-1599"}})
		log.Print("canceling workflow ", id)
		if err != nil {
			log.Printf("error canceling workflow %s: %v", id, err)
			continue
		}
		atomic.AddInt64(&successCount, 1)
	}
}
