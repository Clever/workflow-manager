package batchclient

import (
	"testing"
	"time"

	"github.com/Clever/workflow-manager/mocks/mock_batchiface"
	"github.com/Clever/workflow-manager/resources"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/batch"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestJobToTaskDetail(t *testing.T) {
	t.Log("Converts AWS Batch JobDetail to resources.TaskDetail")
	jobDetail := &batch.JobDetail{
		Status:        aws.String("SUCCEEDED"),
		StatusReason:  aws.String("Essential container in task exited"),
		JobDefinition: aws.String("arn:aws:batch:us-east-1:58111111125:job-definition/batchcli:1"),
		JobId:         aws.String("5a4a8864-2b4e-4c7f-84c1-e3aae28b4ebd"),
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

	be := BatchExecutor{
		defaultQueue: "TEMP",
		client:       nil,
	}

	taskDetail, err := be.jobToTaskDetail(jobDetail)
	assert.Nil(t, err)
	assert.NotNil(t, taskDetail)

	assert.Equal(t, taskDetail.Status, resources.TaskStatusSucceeded)
	assert.Equal(t, taskDetail.CreatedAt.UTC().Format(time.RFC3339Nano), "2017-03-28T00:45:46.376Z")
	assert.Equal(t, taskDetail.StartedAt.UTC().Format(time.RFC3339Nano), "2017-03-28T00:46:48.178Z")
	assert.Equal(t, taskDetail.StoppedAt, time.Time{})

	t.Log("Sets task status correctly when user has canceled it")
	jobDetail.Status = aws.String(batch.JobStatusFailed)
	jobDetail.StatusReason = aws.String("ABORTED_BY_USER: your message here")
	jobDetail.StartedAt = nil

	taskDetail, err = be.jobToTaskDetail(jobDetail)
	assert.NoError(t, err)
	assert.NotNil(t, taskDetail)
	assert.Equal(t, taskDetail.Status, resources.TaskStatusUserAborted)
}

func TestSubmitJobToCustomQueue(t *testing.T) {
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
	out, err := be.SubmitJob(name, definition, dependencies, input, "")
	assert.NoError(t, err)
	assert.Equal(t, "job-id", out)

	t.Log("submits successfully to custom queue that exists")
	mockClient.EXPECT().SubmitJob(gomock.Any()).Return(&batch.SubmitJobOutput{
		JobId: aws.String("job-id"),
	}, nil)
	out, err = be.SubmitJob(name, definition, dependencies, input, "custom")
	assert.NoError(t, err)
	assert.Equal(t, "job-id", out)

	t.Log("errors if you submit job to custom queue that does not exist")
	_, err = be.SubmitJob(name, definition, dependencies, input, "invalid-queue")
	assert.Error(t, err)
}
