package dynamodb

import (
	"time"

	"github.com/Clever/workflow-manager/resources"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

// ddbWorkflow represents the workflow as stored in dynamo.
// Use this to make PutItem queries.
type ddbWorkflow struct {
	ddbWorkflowPrimaryKey
	ddbWorkflowSecondaryKeyWorkflowDefinitionCreatedAt
	CreatedAt          time.Time
	LastUpdated        time.Time                       `dynamodbav:"lastUpdated"`
	WorkflowDefinition ddbWorkflowDefinitionPrimaryKey `dynamodbav:"workflow"`
	Input              []string                        `dynamodbav:"input"`
	Tasks              []*resources.Task               `dynamodbav:"tasks"`
	Status             resources.WorkflowStatus        `dynamodbav:"status"`
}

// EncodeWorkflow encodes a Workflow as a dynamo attribute map.
func EncodeWorkflow(workflow resources.Workflow) (map[string]*dynamodb.AttributeValue, error) {
	return dynamodbattribute.MarshalMap(ddbWorkflow{
		ddbWorkflowPrimaryKey: ddbWorkflowPrimaryKey{
			ID: workflow.ID,
		},
		ddbWorkflowSecondaryKeyWorkflowDefinitionCreatedAt: ddbWorkflowSecondaryKeyWorkflowDefinitionCreatedAt{
			WorkflowDefinitionName: workflow.WorkflowDefinition.Name(),
			CreatedAt:              &workflow.CreatedAt,
		},
		CreatedAt:   workflow.CreatedAt,
		LastUpdated: workflow.LastUpdated,
		WorkflowDefinition: ddbWorkflowDefinitionPrimaryKey{
			Name:    workflow.WorkflowDefinition.Name(),
			Version: workflow.WorkflowDefinition.Version(),
		},
		Input:  workflow.Input,
		Tasks:  workflow.Tasks,
		Status: workflow.Status,
	})
}

// DecodeWorkflow translates a workflow stored in dynamodb to a Workflow object.
func DecodeWorkflow(m map[string]*dynamodb.AttributeValue) (resources.Workflow, error) {
	var dj ddbWorkflow
	if err := dynamodbattribute.UnmarshalMap(m, &dj); err != nil {
		return resources.Workflow{}, err
	}

	wfpk := ddbWorkflowDefinitionPrimaryKey{
		Name:    dj.WorkflowDefinition.Name,
		Version: dj.WorkflowDefinition.Version,
	}
	return resources.Workflow{
		ID:          dj.ddbWorkflowPrimaryKey.ID,
		CreatedAt:   dj.CreatedAt,
		LastUpdated: dj.LastUpdated,
		WorkflowDefinition: resources.WorkflowDefinition{
			NameStr:    wfpk.Name,
			VersionInt: wfpk.Version,
		},
		Input:  dj.Input,
		Tasks:  dj.Tasks,
		Status: dj.Status,
	}, nil
}

// ddbWorkflowPrimaryKey represents the primary + global secondary keys of the workflows table.
// Use this to make GetItem queries.
type ddbWorkflowPrimaryKey struct {
	// ID is the primary key
	ID string `dynamodbav:"id"`
}

func (pk ddbWorkflowPrimaryKey) AttributeDefinitions() []*dynamodb.AttributeDefinition {
	return []*dynamodb.AttributeDefinition{
		{
			AttributeName: aws.String("id"),
			AttributeType: aws.String(dynamodb.ScalarAttributeTypeS),
		},
	}
}

func (pk ddbWorkflowPrimaryKey) KeySchema() []*dynamodb.KeySchemaElement {
	return []*dynamodb.KeySchemaElement{
		{
			AttributeName: aws.String("id"),
			KeyType:       aws.String(dynamodb.KeyTypeHash),
		},
	}
}

// ddbWorkflowWorkflowDefinitionCreatedAtKey is a global secondary index that allows us to query
// for all workflows for a particular workflow, sorted by when they were created.
type ddbWorkflowSecondaryKeyWorkflowDefinitionCreatedAt struct {
	WorkflowDefinitionName string     `dynamodbav:"_gsi-wn,omitempty"`
	CreatedAt              *time.Time `dynamodbav:"_gsi-ca,omitempty"`
}

func (sk ddbWorkflowSecondaryKeyWorkflowDefinitionCreatedAt) Name() string {
	return "workflowname-createdat"
}

func (sk ddbWorkflowSecondaryKeyWorkflowDefinitionCreatedAt) AttributeDefinitions() []*dynamodb.AttributeDefinition {
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

func (sk ddbWorkflowSecondaryKeyWorkflowDefinitionCreatedAt) ConstructQuery() *dynamodb.QueryInput {
	return &dynamodb.QueryInput{
		IndexName: aws.String(sk.Name()),
		ExpressionAttributeNames: map[string]*string{
			"#W": aws.String("_gsi-wn"),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":workflowName": &dynamodb.AttributeValue{
				S: aws.String(sk.WorkflowDefinitionName),
			},
		},
		KeyConditionExpression: aws.String("#W = :workflowName"),
	}
}

func (pk ddbWorkflowSecondaryKeyWorkflowDefinitionCreatedAt) KeySchema() []*dynamodb.KeySchemaElement {
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
