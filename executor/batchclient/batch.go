package batchclient

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/Clever/workflow-manager/resources"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/batch"
	"github.com/aws/aws-sdk-go/service/batch/batchiface"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	"github.com/go-openapi/strfmt"
)

const (
	// DefaultQueue contains the swagger api's default value for queue input
	DefaultQueue = "default"

	// DependenciesEnvVarName is injected for every task
	// with a list of dependency ids
	DependenciesEnvVarName = "_BATCH_DEPENDENCIES"

	// StartingInputEnvVarName is used to pass in the input for the first task
	// in a job
	StartingInputEnvVarName = "_BATCH_START"

	productionQueueName = "workflows-production"
)

// BatchExecutor implements Executor to interact with the AWS Batch API
type BatchExecutor struct {
	client          batchiface.BatchAPI
	defaultQueue    string
	customQueues    map[string]string
	resultsDB       dynamodbiface.DynamoDBAPI
	resultsDBConfig *ResultsDBConfig
}

type ResultsDBConfig struct {
	TableNameDev  string
	TableNameProd string
}

// NewBatchExecutor creates a new BatchExecutor for interacting with an AWS Batch queue
func NewBatchExecutor(
	client batchiface.BatchAPI,
	defaultQueue string,
	customQueues map[string]string,
	resultsDB dynamodbiface.DynamoDBAPI,
	resultsDBConfig *ResultsDBConfig,
) BatchExecutor {
	return BatchExecutor{
		client:          client,
		defaultQueue:    defaultQueue,
		customQueues:    customQueues,
		resultsDB:       resultsDB,
		resultsDBConfig: resultsDBConfig,
	}
}

func (be BatchExecutor) Status(jobs []*resources.Job) []error {
	jobsByID := map[string]*resources.Job{}
	awsJobIDs := []*string{}
	jobIDs := []string{}

	for _, task := range jobs {
		awsJobIDs = append(awsJobIDs, aws.String(task.ID))
		jobIDs = append(jobIDs, task.ID)
		jobsByID[task.ID] = task
	}

	results, err := be.client.DescribeJobs(&batch.DescribeJobsInput{
		Jobs: awsJobIDs,
	})
	if err != nil {
		return []error{err}
	}
	// TODO: aws seems to silently fail on no jobs found. need to investigate
	if len(results.Jobs) == 0 {
		return []error{fmt.Errorf("No task(s) found: %v.", jobIDs)}
	}

	jobOutputsByJobID, err := be.getJobOutputs(jobs)
	if err != nil {
		return []error{err}
	}

	var jobErrors []error
	for _, awsJobDetail := range results.Jobs {
		updatedJobDetail, err := be.getJobDetailFromBatch(awsJobDetail)
		if err != nil {
			// TODO: add jobId to err for clarity
			jobErrors = append(jobErrors, err)
			continue
		}

		job := jobsByID[*awsJobDetail.JobId]

		// Set inputs based on job dependency outputs:
		// Preserve original input (really only applies to the first job in the workflow).
		updatedJobDetail.Input = job.Input
		if len(awsJobDetail.DependsOn) > 0 {
			updatedJobDetail.Input = []string{}
			for _, dependency := range awsJobDetail.DependsOn {
				dependencyOutput := jobOutputsByJobID[*dependency.JobId]
				if len(dependencyOutput) > 0 {
					updatedJobDetail.Input = append(updatedJobDetail.Input, dependencyOutput)
				}
			}
		}

		if jobOutput, ok := jobOutputsByJobID[*awsJobDetail.JobId]; ok {
			updatedJobDetail.Output = []string{jobOutput}
		}

		job.SetDetail(updatedJobDetail)
	}

	return nil
}

func (be BatchExecutor) Cancel(tasks []*resources.Job, reason string) []error {
	var taskErrors []error
	// append JobStatusUserAborted so that we can infer that failure was due to
	// user action when updating status
	userReason := fmt.Sprintf("%s: %s", resources.JobStatusUserAborted, reason)

	for _, task := range tasks {
		_, err := be.client.TerminateJob(&batch.TerminateJobInput{
			JobId:  aws.String(task.ID),
			Reason: aws.String(userReason),
		})
		if err != nil {
			taskErrors = append(taskErrors, err)
		} else {
			task.SetStatus(resources.JobStatusUserAborted)
		}
	}

	return taskErrors
}

// getJobDetailFromBatch converts batch.JobDetail to resources.JobDetail
func (be BatchExecutor) getJobDetailFromBatch(job *batch.JobDetail) (resources.JobDetail, error) {
	var statusReason, containerArn, queueName string
	var createdAt, startedAt, stoppedAt time.Time
	var inputsFromEnv []string
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
	if job.Container != nil && job.Container.TaskArn != nil {
		containerArn = *job.Container.TaskArn
	}
	if job.JobQueue != nil {
		queueName = *job.JobQueue
	}

	attempts := []*models.JobAttempt{}
	if len(job.Attempts) > 0 {
		for _, jattempt := range job.Attempts {
			attempt := &models.JobAttempt{}
			if jattempt.StartedAt != nil {
				attempt.StartedAt = strfmt.DateTime(time.Unix(0, *jattempt.StartedAt*msToNs))
			}
			if jattempt.StoppedAt != nil {
				attempt.StoppedAt = strfmt.DateTime(time.Unix(0, *jattempt.StoppedAt*msToNs))
			}
			if jattempt.Container != nil {
				if jattempt.Container.ContainerInstanceArn != nil {
					attempt.ContainerInstanceARN = *jattempt.Container.ContainerInstanceArn
				}
				if jattempt.Container.TaskArn != nil {
					attempt.TaskARN = *jattempt.Container.TaskArn
				}
				if jattempt.Container.Reason != nil {
					attempt.Reason = *jattempt.Container.Reason
				}
				if jattempt.Container.ExitCode != nil {
					attempt.ExitCode = *jattempt.Container.ExitCode
				}
			}
			attempts = append(attempts, attempt)
		}
	}

	return resources.JobDetail{
		Attempts:     attempts,
		ContainerId:  containerArn,
		CreatedAt:    createdAt,
		Input:        inputsFromEnv,
		QueueName:    queueName,
		StartedAt:    startedAt,
		Status:       be.taskStatus(job),
		StatusReason: statusReason,
		StoppedAt:    stoppedAt,
	}, nil
}

func (be BatchExecutor) getJobOutputs(jobs []*resources.Job) (map[string]string, error) {
	if len(jobs) == 0 {
		return map[string]string{}, nil
	}

	tableName := be.resultsDBConfig.TableNameDev
	if jobs[0].QueueName == productionQueueName {
		tableName = be.resultsDBConfig.TableNameProd
	}

	fetchKeys := []map[string]*dynamodb.AttributeValue{}
	for _, job := range jobs {
		fetchKeys = append(fetchKeys, map[string]*dynamodb.AttributeValue{
			"Key": {S: aws.String(job.ID)},
		})
	}

	results, err := be.resultsDB.BatchGetItem(&dynamodb.BatchGetItemInput{
		RequestItems: map[string]*dynamodb.KeysAndAttributes{
			tableName: {Keys: fetchKeys},
		},
	})
	if err != nil {
		return map[string]string{}, err
	}
	if _, ok := results.Responses[tableName]; !ok {
		return map[string]string{}, nil
	}

	outputs := make(map[string]string, len(jobs))
	for _, entry := range results.Responses[tableName] {
		// Ignore cases with empty results.
		outputAttributeValue := entry["Result"]
		if outputAttributeValue != nil {
			jobID := *entry["Key"].S
			outputs[jobID] = strings.TrimSpace(*outputAttributeValue.S)
		}
	}

	return outputs, nil
}

func (be BatchExecutor) taskStatus(job *batch.JobDetail) resources.JobStatus {
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
		return resources.JobStatusCreated
	case batch.JobStatusPending:
		// pending is waiting for dependencies
		return resources.JobStatusWaiting
	case batch.JobStatusStarting, batch.JobStatusRunnable:
		// starting is waiting for the ecs scheduler
		// runnable is queued for resources
		return resources.JobStatusQueued
	case batch.JobStatusRunning:
		return resources.JobStatusRunning
	case batch.JobStatusSucceeded:
		return resources.JobStatusSucceeded
	case batch.JobStatusFailed:
		if job.StartedAt == nil {
			if strings.HasPrefix(statusReason, string(resources.JobStatusUserAborted)) {
				return resources.JobStatusUserAborted
			} else {
				return resources.JobStatusAborted
			}
		}
		return resources.JobStatusFailed
	default:
		// TODO: actually return error here?
		return "UNKNOWN_STATE"
	}

}

// SubmitWorkflow queues a task using the AWS Batch API client and returns the taskID
func (be BatchExecutor) SubmitWorkflow(name string, definition string, dependencies []string, input []string, queue string, attempts int64) (string, error) {
	jobQueue, err := be.getJobQueue(queue)
	if err != nil {
		return "", err
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

	if attempts != 0 {
		params.RetryStrategy = &batch.RetryStrategy{
			Attempts: aws.Int64(int64(attempts)),
		}
	}

	output, err := be.client.SubmitJob(params)
	if err != nil {
		fmt.Printf("Error in batchclient.SubmitJob: %s", err)
		// TODO: type assert awserr.Error for introspection of error codes
		return "", err
	}

	return *output.JobId, nil
}

// resolves a user specified queue instead an AWS batch queue
func (be BatchExecutor) getJobQueue(queue string) (string, error) {
	var ok bool
	jobQueue := be.defaultQueue
	if queue != DefaultQueue {
		// use a custom queue
		jobQueue, ok = be.customQueues[queue]
		if !ok {
			return "", fmt.Errorf("error getting job queue: %s", queue)
		}
	}
	return jobQueue, nil
}
