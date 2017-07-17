package batchclient

import (
	"encoding/json"
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
	client       batchiface.BatchAPI
	defaultQueue string
	customQueues map[string]string
}

// NewBatchExecutor creates a new BatchExecutor for interacting with an AWS Batch queue
func NewBatchExecutor(client batchiface.BatchAPI, defaultQueue string, customQueues map[string]string) BatchExecutor {
	return BatchExecutor{
		client,
		defaultQueue,
		customQueues,
	}
}

func (be BatchExecutor) Status(tasks []*resources.Task) []error {
	status := map[string]*resources.Task{}
	jobs := []*string{}
	taskIds := []string{}

	for _, task := range tasks {
		jobs = append(jobs, aws.String(task.ID))
		taskIds = append(taskIds, task.ID)
		status[task.ID] = task
	}

	results, err := be.client.DescribeJobs(&batch.DescribeJobsInput{
		Jobs: jobs,
	})
	if err != nil {
		return []error{err}
	}
	// TODO: aws seems to silently fail on no jobs found. need to investigate
	if len(results.Jobs) == 0 {
		return []error{fmt.Errorf("No task(s) found: %v.", taskIds)}
	}

	var taskErrors []error
	for _, jobDetail := range results.Jobs {
		taskDetail, err := be.jobToTaskDetail(jobDetail)
		if err != nil {
			// TODO: add jobId to err for clarity
			taskErrors = append(taskErrors, err)
			continue
		}
		status[*jobDetail.JobId].SetDetail(taskDetail)
	}

	return nil
}

func (be BatchExecutor) Cancel(tasks []*resources.Task, reason string) []error {
	var taskErrors []error
	// append TaskStatusUserAborted so that we can infer that failure was due to
	// user action when updating status
	userReason := fmt.Sprintf("%s: %s", resources.TaskStatusUserAborted, reason)

	for _, task := range tasks {
		_, err := be.client.TerminateJob(&batch.TerminateJobInput{
			JobId:  aws.String(task.ID),
			Reason: aws.String(userReason),
		})
		if err != nil {
			taskErrors = append(taskErrors, err)
		} else {
			task.SetStatus(resources.TaskStatusUserAborted)
		}
	}

	return taskErrors
}

func (be BatchExecutor) jobToTaskDetail(job *batch.JobDetail) (resources.TaskDetail, error) {
	var statusReason, containerArn string
	var createdAt, startedAt, stoppedAt time.Time
	msToNs := int64(time.Millisecond)
	if job.StatusReason != nil {
		statusReason = *job.StatusReason
	}
	if job.CreatedAt != nil {
		createdAt = time.Unix(0, *job.CreatedAt*msToNs)
	}
	if job.StartedAt != nil {
		startedAt = time.Unix(0, *job.StartedAt*msToNs)
	}
	if job.StoppedAt != nil {
		stoppedAt = time.Unix(0, *job.StoppedAt*msToNs)
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
		ContainerId:  containerArn,
	}, nil
}

func (be BatchExecutor) taskStatus(job *batch.JobDetail) resources.TaskStatus {
	if job == nil || job.Status == nil {
		return "UNKNOWN_STATE"
	}
	statusReason := ""
	if job.StatusReason != nil {
		statusReason = *job.StatusReason
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
			if strings.HasPrefix(string(resources.TaskStatusUserAborted), statusReason) {
				return resources.TaskStatusUserAborted
			} else {
				return resources.TaskStatusAborted
			}
		}
		return resources.TaskStatusFailed
	default:
		// TODO: actually return error here?
		return "UNKNOWN_STATE"
	}

}

// SubmitJob queues a task using the AWS Batch API client and returns the taskID
func (be BatchExecutor) SubmitJob(name string, definition string, dependencies []string, input []string, queue string) (string, error) {
	var ok bool
	jobQueue := be.defaultQueue
	if queue != "" {
		// use a custom queue
		jobQueue, ok = be.customQueues[queue]
		if !ok {
			return "", fmt.Errorf("attempted to submit job to invalid queue: %s", queue)
		}

	}
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

	if input != nil && len(input) > 0 {
		inputStr, err := json.Marshal(input)
		if err != nil {
			return "", fmt.Errorf("Failed to marshall task %s input: %s", name, err)
		}
		environment = append(environment, &batch.KeyValuePair{
			Name:  aws.String(StartingInputEnvVarName),
			Value: aws.String(string(inputStr)),
		})
	}

	params := &batch.SubmitJobInput{
		JobName:       aws.String(name),
		JobDefinition: aws.String(definition),
		JobQueue:      aws.String(jobQueue),
		ContainerOverrides: &batch.ContainerOverrides{
			Environment: environment,
		},
		DependsOn: jobDeps,
	}

	output, err := be.client.SubmitJob(params)
	if err != nil {
		fmt.Printf("Error in batchclient.SubmitJob: %s", err)
		// TODO: type assert awserr.Error for introspection of error codes
		return "", err
	}

	return *output.JobId, nil
}
