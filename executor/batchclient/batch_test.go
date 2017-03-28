package batchclient

import (
	"testing"
	"time"

	"github.com/Clever/workflow-manager/resources"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/batch"
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
			MountPoints:          []*batch.MountPoint{},
			Image:                aws.String("clever/batchcli:999999"),
			ContainerInstanceArn: aws.String("arn:aws:ecs:us-east-1:589690932525:container-instance/460bbb01-93aa-4b5f-9740-0c1aed552691"),
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
		queue:  "TEMP",
		client: nil,
	}

	taskDetail, err := be.jobToTaskDetail(jobDetail)
	assert.Nil(t, err)
	assert.NotNil(t, taskDetail)

	assert.Equal(t, taskDetail.Status, resources.TaskStatusSucceeded)
	assert.Equal(t, taskDetail.CreatedAt.Unix(), *jobDetail.CreatedAt)
	assert.Equal(t, taskDetail.StartedAt.Unix(), *jobDetail.StartedAt)
	assert.Equal(t, taskDetail.StoppedAt, time.Time{})
}
