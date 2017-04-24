package dynamodb

import (
	"time"

	"github.com/Clever/workflow-manager/resources"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

// ddbJob represents the job as stored in dynamo.
// Use this to make PutItem queries.
type ddbJob struct {
	ddbJobPrimaryKey
	ddbJobSecondaryKeyWorkflowCreatedAt
	CreatedAt   time.Time
	LastUpdated time.Time             `dynamodbav:"lastUpdated"`
	Workflow    ddbWorkflowPrimaryKey `dynamodbav:"workflow"`
	Input       []string              `dynamodbav:"input"`
	Tasks       []*resources.Task     `dynamodbav:"tasks"`
	Status      resources.JobStatus   `dynamodbav:"status"`
}

// EncodeJob encodes a Job as a dynamo attribute map.
func EncodeJob(job resources.Job) (map[string]*dynamodb.AttributeValue, error) {
	return dynamodbattribute.MarshalMap(ddbJob{
		ddbJobPrimaryKey: ddbJobPrimaryKey{
			ID: job.ID,
		},
		ddbJobSecondaryKeyWorkflowCreatedAt: ddbJobSecondaryKeyWorkflowCreatedAt{
			WorkflowName: job.Workflow.Name(),
			CreatedAt:    &job.CreatedAt,
		},
		CreatedAt:   job.CreatedAt,
		LastUpdated: job.LastUpdated,
		Workflow: ddbWorkflowPrimaryKey{
			Name:    job.Workflow.Name(),
			Version: job.Workflow.Version(),
		},
		Input:  job.Input,
		Tasks:  job.Tasks,
		Status: job.Status,
	})
}

// DecodeJob translates a job stored in dynamodb to a Job object.
func DecodeJob(m map[string]*dynamodb.AttributeValue) (resources.Job, error) {
	var dj ddbJob
	if err := dynamodbattribute.UnmarshalMap(m, &dj); err != nil {
		return resources.Job{}, err
	}

	wfpk := ddbWorkflowPrimaryKey{
		Name:    dj.Workflow.Name,
		Version: dj.Workflow.Version,
	}
	return resources.Job{
		ID:          dj.ddbJobPrimaryKey.ID,
		CreatedAt:   dj.CreatedAt,
		LastUpdated: dj.LastUpdated,
		Workflow: resources.WorkflowDefinition{
			NameStr:    wfpk.Name,
			VersionInt: wfpk.Version,
		},
		Input:  dj.Input,
		Tasks:  dj.Tasks,
		Status: dj.Status,
	}, nil
}

// ddbJobPrimaryKey represents the primary + global secondary keys of the jobs table.
// Use this to make GetItem queries.
type ddbJobPrimaryKey struct {
	// ID is the primary key
	ID string `dynamodbav:"id"`
}

func (pk ddbJobPrimaryKey) AttributeDefinitions() []*dynamodb.AttributeDefinition {
	return []*dynamodb.AttributeDefinition{
		{
			AttributeName: aws.String("id"),
			AttributeType: aws.String(dynamodb.ScalarAttributeTypeS),
		},
	}
}

func (pk ddbJobPrimaryKey) KeySchema() []*dynamodb.KeySchemaElement {
	return []*dynamodb.KeySchemaElement{
		{
			AttributeName: aws.String("id"),
			KeyType:       aws.String(dynamodb.KeyTypeHash),
		},
	}
}

// ddbJobWorkflowCreatedAtKey is a global secondary index that allows us to query
// for all jobs for a particular workflow, sorted by when they were created.
type ddbJobSecondaryKeyWorkflowCreatedAt struct {
	WorkflowName string     `dynamodbav:"_gsi-wn,omitempty"`
	CreatedAt    *time.Time `dynamodbav:"_gsi-ca,omitempty"`
}

func (sk ddbJobSecondaryKeyWorkflowCreatedAt) Name() string {
	return "workflowname-createdat"
}

func (sk ddbJobSecondaryKeyWorkflowCreatedAt) AttributeDefinitions() []*dynamodb.AttributeDefinition {
	return []*dynamodb.AttributeDefinition{
		{
			AttributeName: aws.String("_gsi-wn"),
			AttributeType: aws.String(dynamodb.ScalarAttributeTypeS),
		},
		{
			AttributeName: aws.String("_gsi-ca"),
			AttributeType: aws.String(dynamodb.ScalarAttributeTypeS),
		},
	}
}

func (sk ddbJobSecondaryKeyWorkflowCreatedAt) ConstructQuery() *dynamodb.QueryInput {
	return &dynamodb.QueryInput{
		IndexName: aws.String(sk.Name()),
		ExpressionAttributeNames: map[string]*string{
			"#W": aws.String("_gsi-wn"),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":workflowName": &dynamodb.AttributeValue{
				S: aws.String(sk.WorkflowName),
			},
		},
		KeyConditionExpression: aws.String("#W = :workflowName"),
	}
}

func (pk ddbJobSecondaryKeyWorkflowCreatedAt) KeySchema() []*dynamodb.KeySchemaElement {
	return []*dynamodb.KeySchemaElement{
		{
			AttributeName: aws.String("_gsi-wn"),
			KeyType:       aws.String(dynamodb.KeyTypeHash),
		},
		{
			AttributeName: aws.String("_gsi-ca"),
			KeyType:       aws.String(dynamodb.KeyTypeRange),
		},
	}
}
