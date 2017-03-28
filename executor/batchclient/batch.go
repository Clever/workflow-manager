package batchclient

import (
	"fmt"
	"strings"
	"time"

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

func (be BatchExecutor) Status(tasks []*resources.Task) []error {
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
		return []error{err}
	}

	var taskErrors []error
	for _, jobDetail := range results.Jobs {
		taskDetail, err := be.jobToTaskDetail(jobDetail)
		if err != nil {
			// TODO: add jobId to err for clarity
			taskErrors = append(taskErrors, err)
		}
		status[*jobDetail.JobId].SetDetail(taskDetail)
	}

	return nil
}

func (be BatchExecutor) jobToTaskDetail(job *batch.JobDetail) (resources.TaskDetail, error) {
	var statusReason, containerArn string
	var createdAt, startedAt, stoppedAt time.Time
	if job.StatusReason != nil {
		statusReason = *job.StatusReason
	}
	if job.CreatedAt != nil {
		createdAt = time.Unix(*job.CreatedAt, 0)
	}
	if job.StartedAt != nil {
		startedAt = time.Unix(*job.StartedAt, 0)
	}
	if job.StoppedAt != nil {
		stoppedAt = time.Unix(*job.StoppedAt, 0)
	}
	if job.Container != nil && job.Container.ContainerInstanceArn != nil {
		containerArn = *job.Container.ContainerInstanceArn
	}

	return resources.TaskDetail{
		StatusReason: statusReason,
		Status:       be.taskStatus(job),
		CreatedAt:    createdAt,
		StartedAt:    startedAt,
		StoppedAt:    stoppedAt,
		Container:    containerArn,
	}, nil
}

func (be BatchExecutor) taskStatus(job *batch.JobDetail) resources.TaskStatus {
	if job == nil || job.Status == nil {
		return "UNKNOWN_STATE"
	}

	switch *job.Status {
	case batch.JobStatusSubmitted:
		// submitted is queued for batch scheduler
		return resources.TaskStatusCreated
	case batch.JobStatusPending:
		// pending is waiting for dependencies
		return resources.TaskStatusWaiting
	case batch.JobStatusStarting, batch.JobStatusRunnable:
		// starting is waiting for the ecs scheduler
		// runnable is queued for resources
		return resources.TaskStatusQueued
	case batch.JobStatusRunning:
		return resources.TaskStatusRunning
	case batch.JobStatusSucceeded:
		return resources.TaskStatusSucceeded
	case batch.JobStatusFailed:
		if job.StartedAt == nil {
			return resources.TaskStatusAborted
		}
		return resources.TaskStatusFailed
	default:
		// TODO: actually return error here?
		return "UNKNOWN_STATE"
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
