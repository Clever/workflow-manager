package batchclient

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Clever/workflow-manager/gen-go/models"
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

	// TODO: Temporary hack to allow us to determine at request-time which worker results table to
	// fetch outputs from.
	// Needs cleanup.
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
	// TODO: Temporary hack to allow us to determine at request-time which worker results table to
	// fetch outputs from.
	// Needs cleanup.
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

func (be BatchExecutor) Status(jobs []*models.Job) []error {
	jobsByID := map[string]*models.Job{}
	awsJobIDs := []*string{}
	jobIDs := []string{}

	for _, job := range jobs {
		awsJobIDs = append(awsJobIDs, aws.String(job.ID))
		jobIDs = append(jobIDs, job.ID)
		jobsByID[job.ID] = job
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
		job := jobsByID[*awsJobDetail.JobId]
		if err := be.updateJobWithDetailsFromBatch(job, awsJobDetail); err != nil {
			// TODO: add jobId to err for clarity
			jobErrors = append(jobErrors, err)
			continue
		}

		// Set inputs based on successful job dependency outputs:
		if len(awsJobDetail.DependsOn) > 0 {
			input := []string{}
			for _, dependency := range awsJobDetail.DependsOn {
				jobDependency := jobsByID[*dependency.JobId]
				if jobDependency.Status != models.JobStatusSucceeded {
					continue
				}

				dependencyOutput := jobOutputsByJobID[*dependency.JobId]
				if len(dependencyOutput) > 0 {
					var output []string
					json.Unmarshal([]byte(dependencyOutput), &output)
					input = append(input, output...)
				}
			}
			inputBytes, _ := json.Marshal(input)
			job.Input = string(inputBytes)
		}

		if jobOutput, ok := jobOutputsByJobID[*awsJobDetail.JobId]; ok {
			//outputBytes, _ := json.Marshal([]string{jobOutput})
			job.Output = jobOutput
		}
	}

	return nil
}

func (be BatchExecutor) Cancel(tasks []*models.Job, reason string) []error {
	var taskErrors []error
	// append JobStatusUserAborted so that we can infer that failure was due to
	// user action when updating status
	userReason := fmt.Sprintf("%s: %s", models.JobStatusAbortedByUser, reason)

	for _, task := range tasks {
		_, err := be.client.TerminateJob(&batch.TerminateJobInput{
			JobId:  aws.String(task.ID),
			Reason: aws.String(userReason),
		})
		if err != nil {
			taskErrors = append(taskErrors, err)
		} else {
			task.Status = models.JobStatusAbortedByUser
		}
	}

	return taskErrors
}

// updateJobWithDetailsFromBatch updates a job with data from batch.JobDetail.
func (be BatchExecutor) updateJobWithDetailsFromBatch(ourJob *models.Job, batchJob *batch.JobDetail) error {
	var statusReason, containerArn, queueName string
	var createdAt, startedAt, stoppedAt time.Time
	msToNs := int64(time.Millisecond)
	if batchJob.StatusReason != nil {
		statusReason = *batchJob.StatusReason
	}
	if batchJob.CreatedAt != nil {
		createdAt = time.Unix(0, *batchJob.CreatedAt*msToNs)
	}
	if batchJob.StartedAt != nil {
		startedAt = time.Unix(0, *batchJob.StartedAt*msToNs)
	}
	if batchJob.StoppedAt != nil {
		stoppedAt = time.Unix(0, *batchJob.StoppedAt*msToNs)
	}
	if batchJob.Container != nil && batchJob.Container.TaskArn != nil {
		containerArn = *batchJob.Container.TaskArn
	}
	if batchJob.JobQueue != nil {
		queueName = *batchJob.JobQueue
	}

	attempts := []*models.JobAttempt{}
	if len(batchJob.Attempts) > 0 {
		for _, jattempt := range batchJob.Attempts {
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

	// modify the job with what we gathered from batch job details
	ourJob.Attempts = attempts
	ourJob.Container = containerArn
	ourJob.CreatedAt = strfmt.DateTime(createdAt)
	ourJob.Queue = queueName
	ourJob.StartedAt = strfmt.DateTime(startedAt)
	ourJob.Status = be.taskStatus(batchJob)
	ourJob.StatusReason = statusReason
	ourJob.StoppedAt = strfmt.DateTime(stoppedAt)
	return nil
}

func (be BatchExecutor) getJobOutputs(jobs []*models.Job) (map[string]string, error) {
	if len(jobs) == 0 {
		return map[string]string{}, nil
	}

	tableName := be.resultsDBConfig.TableNameDev
	if jobs[0].Queue == productionQueueName {
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

func (be BatchExecutor) taskStatus(job *batch.JobDetail) models.JobStatus {
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
		return models.JobStatusCreated
	case batch.JobStatusPending:
		// pending is waiting for dependencies
		return models.JobStatusWaitingForDeps
	case batch.JobStatusStarting, batch.JobStatusRunnable:
		// starting is waiting for the ecs scheduler
		// runnable is queued for resources
		return models.JobStatusQueued
	case batch.JobStatusRunning:
		return models.JobStatusRunning
	case batch.JobStatusSucceeded:
		return models.JobStatusSucceeded
	case batch.JobStatusFailed:
		if job.StartedAt == nil {
			if strings.HasPrefix(statusReason, string("ABORTED_BY_USER")) {
				return models.JobStatusAbortedByUser
			} else {
				return models.JobStatusAbortedDepsFailed
			}
		}
		return models.JobStatusFailed
	default:
		// TODO: actually return error here?
		return "UNKNOWN_STATE"
	}

}

// SubmitWorkflow queues a task using the AWS Batch API client and returns the taskID
func (be BatchExecutor) SubmitWorkflow(name string, definition string, dependencies []string, input string, queue string, attempts int64) (string, error) {
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

	if len(input) > 0 {
		environment = append(environment, &batch.KeyValuePair{
			Name:  aws.String(StartingInputEnvVarName),
			Value: aws.String(input),
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
