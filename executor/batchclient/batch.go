package batchclient

import (
	"fmt"
	"strings"

	"github.com/Clever/workflow-manager/resources"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/batch"
	"github.com/aws/aws-sdk-go/service/batch/batchiface"
)

// DependenciesEnvVarName is injected for every task
// with a list of dependency ids
const DependenciesEnvVarName = "_BATCH_DEPENDENCIES"

// StartingInputEnvVarName is used to pass in the input for the first task
// in a job
const StartingInputEnvVarName = "_BATCH_START"

// BatchExecutor implements Executor to interact with the AWS Batch API
type BatchExecutor struct {
	client batchiface.BatchAPI
	queue  string
}

// NewBatchExecutor creates a new BatchExecutor for interacting with an AWS Batch queue
func NewBatchExecutor(client batchiface.BatchAPI, queue string) BatchExecutor {
	return BatchExecutor{
		client,
		queue,
	}
}

func (be BatchExecutor) Status(tasks []*resources.Task) error {
	status := map[string]*resources.Task{}
	jobs := []*string{}

	for _, task := range tasks {
		jobs = append(jobs, aws.String(task.ID))
		status[task.ID] = task
	}

	results, err := be.client.DescribeJobs(&batch.DescribeJobsInput{
		Jobs: jobs,
	})
	if err != nil {
		return err
	}

	for _, jobDetail := range results.Jobs {
		status[*jobDetail.JobId].SetStatus(be.convertStatus(jobDetail.Status))
	}

	return nil
}

func (be BatchExecutor) convertStatus(jobStatus *string) resources.TaskStatus {
	switch *jobStatus {
	case batch.JobStatusSubmitted, batch.JobStatusRunnable:
		// submitted is queued for scheduler
		// runnable is queued for resources
		return resources.TaskStatusQueued
	case batch.JobStatusPending:
		// pending is waiting for dependencies
		return resources.TaskStatusWaiting
	case batch.JobStatusRunning:
		return resources.TaskStatusRunning
	case batch.JobStatusSucceeded:
		return resources.TaskStatusSucceeded
	case batch.JobStatusFailed:
		return resources.TaskStatusFailed
	default:
		// TODO: actually return error here?
		return "INVALID_STATE_ERROR"
	}
}

// SubmitJob queues a task using the AWS Batch API client and returns the taskID
func (be BatchExecutor) SubmitJob(name string, definition string, dependencies []string, input string) (string, error) {
	jobDeps := []*batch.JobDependency{}

	for _, d := range dependencies {
		jobDeps = append(jobDeps, &batch.JobDependency{
			JobId: aws.String(d),
		})
	}

	environment := []*batch.KeyValuePair{
		&batch.KeyValuePair{
			Name:  aws.String(DependenciesEnvVarName),
			Value: aws.String(strings.Join(dependencies, ",")),
		},
	}

	// this parameter can be optionally
	// used to add a default CMD argument to
	// the worker container.
	jobParams := map[string]*string{}
	if input != "" {
		jobParams[StartingInputEnvVarName] = aws.String(input)
		environment = append(environment, &batch.KeyValuePair{
			Name:  aws.String(StartingInputEnvVarName),
			Value: aws.String(input),
		})
	} else {
		// TODO: fix: AWS does not like empty string here
		jobParams[StartingInputEnvVarName] = aws.String(" ")
	}

	params := &batch.SubmitJobInput{
		JobName:       aws.String(name),
		JobDefinition: aws.String(definition),
		JobQueue:      aws.String(be.queue),
		ContainerOverrides: &batch.ContainerOverrides{
			Environment: environment,
		},
		DependsOn:  jobDeps,
		Parameters: jobParams,
	}

	output, err := be.client.SubmitJob(params)
	if err != nil {
		fmt.Printf("Error in batchclient.SubmitJob: %s", err)
		// TODO: type assert awserr.Error for introspection of error codes
		return "", err
	}

	return *output.JobId, nil
}
