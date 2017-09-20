package batchclient

import (
	"testing"
	"time"

	"github.com/Clever/workflow-manager/mocks/mock_batchiface"
	"github.com/Clever/workflow-manager/mocks/mock_dynamodbiface"
	"github.com/Clever/workflow-manager/resources"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/batch"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

type testConfig struct {
	mockController      *gomock.Controller
	mockBatchClient     *mock_batchiface.MockBatchAPI
	mockResultsDBClient *mock_dynamodbiface.MockDynamoDBAPI
	executor            *BatchExecutor
}

func TestStatus(t *testing.T) {
	c := newTestConfig(t)
	defer c.mockController.Finish()

	t.Log("Converts AWS Batch JobDetail to resources.JobDetail")
	jobID1 := aws.String("5a4a8864-2b4e-4c7f-84c1-e3aae28b4ebd")
	batchJobDetail1 := batch.JobDetail{
		Status:        aws.String("SUCCEEDED"),
		StatusReason:  aws.String("Essential container in task exited"),
		JobDefinition: aws.String("arn:aws:batch:us-east-1:58111111125:job-definition/batchcli:1"),
		JobId:         jobID1,
		Container: &batch.ContainerDetail{
			MountPoints: []*batch.MountPoint{},
			Image:       aws.String("clever/batchcli:999999"),
			TaskArn:     aws.String("arn:aws:ecs:us-east-1:589690932525:task/97be0e7f-38f6-4675-9bac-82d36a3170d1"),
			Environment: []*batch.KeyValuePair{
				&batch.KeyValuePair{
					Name:  aws.String("_BATCH_DEPENDENCIES"),
					Value: aws.String("ae862a59-b068-4f20-ab46-e3e8e593aaf2"),
				},
			},
			Vcpus:      aws.Int64(1),
			JobRoleArn: aws.String("arn:aws:iam::589111111111:role/workflow-batch-cli-dev"),
		},
		JobName:   aws.String("935f4920-8955-4229-9425-f60e53b41ba8--echo"),
		StartedAt: aws.Int64(1490662008178),
		CreatedAt: aws.Int64(1490661946376),
		//StoppedAt: aws.Int64(1490662008500),  // Works with incomplete information
	}

	jobID2 := aws.String("6a4a8864-2b4e-4c7f-84c1-e3aae28b4ebd")
	batchJobDetail2 := batchJobDetail1
	batchJobDetail2.JobId = jobID2
	batchJobDetail2.DependsOn = []*batch.JobDependency{{JobId: jobID1}}

	c.mockBatchClient.EXPECT().
		DescribeJobs(&batch.DescribeJobsInput{Jobs: []*string{jobID1, jobID2}}).
		Return(&batch.DescribeJobsOutput{
			Jobs: []*batch.JobDetail{&batchJobDetail1, &batchJobDetail2},
		}, nil)

	c.mockResultsDBClient.EXPECT().
		BatchGetItem(&dynamodb.BatchGetItemInput{
			RequestItems: map[string]*dynamodb.KeysAndAttributes{
				c.executor.resultsDBConfig.TableNameDev: {
					Keys: []map[string]*dynamodb.AttributeValue{
						{"Key": {S: jobID1}},
						{"Key": {S: jobID2}},
					},
				},
			},
		}).
		Return(&dynamodb.BatchGetItemOutput{
			Responses: map[string][]map[string]*dynamodb.AttributeValue{
				c.executor.resultsDBConfig.TableNameDev: {
					{
						"Key":    {S: jobID1},
						"Result": {S: aws.String("job 1 output")},
					},
					{
						"Key":    {S: jobID2},
						"Result": {S: aws.String("job 2 output")},
					},
				},
			},
		}, nil)

	job1 := &resources.Job{
		ID:   *jobID1,
		Name: "should not be overwritten",
		JobDetail: resources.JobDetail{
			Input: []string{"initial input - should not be overwritten"},
		},
	}
	job2 := &resources.Job{
		ID:   *jobID2,
		Name: "should not be overwritten",
	}

	errs := c.executor.Status([]*resources.Job{job1, job2})
	for _, err := range errs {
		assert.NoError(t, err)
	}

	assert.Equal(t, *jobID1, job1.ID)
	assert.Equal(t, resources.JobStatusSucceeded, job1.Status)
	assert.Equal(t, "2017-03-28T00:45:46.376Z", job1.CreatedAt.UTC().Format(time.RFC3339Nano))
	assert.Equal(t, "2017-03-28T00:46:48.178Z", job1.StartedAt.UTC().Format(time.RFC3339Nano))
	assert.Equal(t, time.Time{}, job1.StoppedAt)
	assert.Equal(t, "should not be overwritten", job1.Name)
	assert.Equal(t, []string{"initial input - should not be overwritten"}, job1.Input)
	assert.Equal(t, []string{"job 1 output"}, job1.Output)

	assert.Equal(t, *jobID2, job2.ID)
	assert.Equal(t, "should not be overwritten", job2.Name)
	assert.Equal(t, job1.Output, job2.Input)
	assert.Equal(t, []string{"job 2 output"}, job2.Output)

	t.Log("Sets task status correctly when user has canceled it")
	cancelledBatchJobDetails := batchJobDetail1
	cancelledBatchJobDetails.Status = aws.String(batch.JobStatusFailed)
	cancelledBatchJobDetails.Status = aws.String(batch.JobStatusFailed)
	cancelledBatchJobDetails.StatusReason = aws.String("ABORTED_BY_USER: your message here")
	cancelledBatchJobDetails.StartedAt = nil

	c.mockBatchClient.EXPECT().
		DescribeJobs(&batch.DescribeJobsInput{Jobs: []*string{jobID1}}).
		Return(&batch.DescribeJobsOutput{
			Jobs: []*batch.JobDetail{&cancelledBatchJobDetails},
		}, nil)

	c.mockResultsDBClient.EXPECT().
		BatchGetItem(&dynamodb.BatchGetItemInput{
			RequestItems: map[string]*dynamodb.KeysAndAttributes{
				c.executor.resultsDBConfig.TableNameDev: {
					Keys: []map[string]*dynamodb.AttributeValue{
						{"Key": {S: jobID1}},
					},
				},
			},
		}).
		Return(&dynamodb.BatchGetItemOutput{
			Responses: map[string][]map[string]*dynamodb.AttributeValue{},
		}, nil)

	errs = c.executor.Status([]*resources.Job{job1})
	for _, err := range errs {
		assert.NoError(t, err)
	}
	assert.Equal(t, job1.Status, resources.JobStatusUserAborted)
}

func TestSubmitWorkflowToCustomQueue(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	mockClient := mock_batchiface.NewMockBatchAPI(mockCtrl)

	be := BatchExecutor{
		defaultQueue: "TEMP",
		client:       mockClient,
		customQueues: map[string]string{
			"custom": "some-queue",
		},
	}

	// args
	name := "name"
	definition := "definition"
	dependencies := []string{}
	input := []string{}

	t.Log("submits successfully to default queue")
	mockClient.EXPECT().SubmitJob(gomock.Any()).Return(&batch.SubmitJobOutput{
		JobId: aws.String("job-id"),
	}, nil)
	out, err := be.SubmitWorkflow(name, definition, dependencies, input, DefaultQueue, 0)
	assert.NoError(t, err)
	assert.Equal(t, "job-id", out)

	t.Log("submits successfully to custom queue that exists")
	mockClient.EXPECT().SubmitJob(gomock.Any()).Return(&batch.SubmitJobOutput{
		JobId: aws.String("job-id"),
	}, nil)
	out, err = be.SubmitWorkflow(name, definition, dependencies, input, "custom", 0)
	assert.NoError(t, err)
	assert.Equal(t, "job-id", out)

	t.Log("errors if you submit job to custom queue that does not exist")
	_, err = be.SubmitWorkflow(name, definition, dependencies, input, "invalid-queue", 0)
	assert.Error(t, err)
}

func newTestConfig(t *testing.T) *testConfig {
	mockController := gomock.NewController(t)
	mockBatchClient := mock_batchiface.NewMockBatchAPI(mockController)
	mockResultsDBClient := mock_dynamodbiface.NewMockDynamoDBAPI(mockController)

	return &testConfig{
		mockController:      mockController,
		mockBatchClient:     mockBatchClient,
		mockResultsDBClient: mockResultsDBClient,
		executor: &BatchExecutor{
			defaultQueue: "TEMP",
			client:       mockBatchClient,
			resultsDB:    mockResultsDBClient,
			resultsDBConfig: &ResultsDBConfig{
				TableNameDev: "results-dev",
			},
		},
	}
}
