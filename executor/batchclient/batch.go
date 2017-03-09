package batchclient

import (
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/batch"
	"github.com/aws/aws-sdk-go/service/batch/batchiface"
)

// DependenciesEnvVarName is injected for every task
const DependenciesEnvVarName = "_DEPENDENCIES"

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

// SubmitJob queues a task using the AWS Batch API client and returns the taskID
func (be BatchExecutor) SubmitJob(name string, definition string, dependencies []string, input string) (string, error) {
	jobDeps := []*batch.JobDependency{}

	for _, d := range dependencies {
		jobDeps = append(jobDeps, &batch.JobDependency{
			JobId: aws.String(d),
		})
	}

	params := &batch.SubmitJobInput{
		JobName:       aws.String(name),
		JobDefinition: aws.String(definition),
		JobQueue:      aws.String(be.queue),
		ContainerOverrides: &batch.ContainerOverrides{
			Environment: []*batch.KeyValuePair{
				&batch.KeyValuePair{
					Name:  aws.String(DependenciesEnvVarName),
					Value: aws.String(strings.Join(dependencies, ",")),
				},
			},
		},
		DependsOn: jobDeps,
		// this parameter is used to add a default CMD argument to
		// the worker container.
		Parameters: map[string]*string{
			"_WF_START": aws.String(input),
		},
	}

	output, err := be.client.SubmitJob(params)
	if err != nil {
		// TODO: type assert awserr.Error for introspection of error codes
		return "", err
	}

	return *output.JobId, nil
}
