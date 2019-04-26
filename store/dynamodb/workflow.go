package dynamodb

import (
	"fmt"
	"strings"
	"time"

	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/go-openapi/strfmt"
)

// SummaryKeys are json paths to the Workflow fields we want to pull out of dynamodb
// when summaryOnly=true in WorkflowQuery
// This should be kept in sync with the WorkflowSummary model defined in swagger
var SummaryKeys = []string{
	"Workflow.createdAt",
	"Workflow.id",
	"Workflow.#I", // input
	"Workflow.lastUpdated",
	"Workflow.queue",
	"Workflow.namespace",
	"Workflow.resolvedByUser",
	"Workflow.retries",
	"Workflow.retryFor",
	"Workflow.#S", // status
	"Workflow.tags",

	"Workflow.workflowDefinition.#N",
	"Workflow.workflowDefinition.version",
}

const WorkflowTTL = 30 * 24 * time.Hour // 30 days

var summaryProjectionExpression = strings.Join(SummaryKeys, ", ")
var summaryExpressionAttributeNames = map[string]*string{
	"#S": aws.String("status"),
	"#I": aws.String("input"),
	"#N": aws.String("name"),
}

// ddbWorkflow represents the workflow as stored in dynamo.
// Use this to make PutItem queries.
type ddbWorkflow struct {
	ddbWorkflowPrimaryKey
	ddbWorkflowSecondaryKeyWorkflowDefinitionCreatedAt
	ddbWorkflowTTL
	Workflow models.Workflow
}

// EncodeWorkflow encodes a Workflow as a dynamo attribute map.
func EncodeWorkflow(workflow models.Workflow) (map[string]*dynamodb.AttributeValue, error) {
	return dynamodbattribute.MarshalMap(ddbWorkflow{
		ddbWorkflowPrimaryKey: ddbWorkflowPrimaryKey{
			ID: workflow.ID,
		},
		ddbWorkflowSecondaryKeyWorkflowDefinitionCreatedAt: ddbWorkflowSecondaryKeyWorkflowDefinitionCreatedAt{
			WorkflowDefinitionName: workflow.WorkflowDefinition.Name,
			CreatedAt:              workflow.CreatedAt,
		},
		ddbWorkflowTTL: ddbWorkflowTTL{
			TTL: strfmt.DateTime(time.Time(workflow.CreatedAt).Add(WorkflowTTL)),
		},
		Workflow: workflow,
	})
}

// DecodeWorkflow translates a workflow stored in dynamodb to a Workflow object.
func DecodeWorkflow(m map[string]*dynamodb.AttributeValue) (models.Workflow, error) {
	var dj ddbWorkflow
	if err := dynamodbattribute.UnmarshalMap(m, &dj); err != nil {
		return models.Workflow{}, err
	}
	return dj.Workflow, nil
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

// ddbWorkflowSecondaryKeyWorkflowDefinitionCreatedAt is a global secondary index that allows us to query
// for all workflows for a particular workflow, sorted by when they were created.
type ddbWorkflowSecondaryKeyWorkflowDefinitionCreatedAt struct {
	WorkflowDefinitionName string          `dynamodbav:"_gsi-wn,omitempty"`
	CreatedAt              strfmt.DateTime `dynamodbav:"_gsi-ca,omitempty"`
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

func (sk ddbWorkflowSecondaryKeyWorkflowDefinitionCreatedAt) ConstructQuery(summaryOnly bool) (*dynamodb.QueryInput, error) {
	workflowNameAV, err := dynamodbattribute.Marshal(sk.WorkflowDefinitionName)
	if err != nil {
		return nil, fmt.Errorf("could not marshal workflow definition name: %s", err)
	}

	queryInput := &dynamodb.QueryInput{
		IndexName: aws.String(sk.Name()),
		ExpressionAttributeNames: map[string]*string{
			"#W": aws.String("_gsi-wn"),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":workflowName": workflowNameAV,
		},
		KeyConditionExpression: aws.String("#W = :workflowName"),
	}

	if summaryOnly {
		onlySummaryFields(queryInput)
	}

	return queryInput, nil
}

func (sk ddbWorkflowSecondaryKeyWorkflowDefinitionCreatedAt) KeySchema() []*dynamodb.KeySchemaElement {
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

// ddbWorkflowTTL is the time at which the workflow will get TTL'd by dynamo.
type ddbWorkflowTTL struct {
	TTL strfmt.DateTime `dynamodbav:"_ttl,unixtime"` // must be unix time to work with dynamodb builtin TTL support
}

func (ttl ddbWorkflowTTL) AttributeDefinition() *dynamodb.AttributeDefinition {
	return &dynamodb.AttributeDefinition{
		AttributeName: aws.String("_ttl"),
		AttributeType: aws.String(dynamodb.ScalarAttributeTypeN),
	}
}

func onlySummaryFields(queryInput *dynamodb.QueryInput) {
	queryInput = queryInput.SetProjectionExpression(summaryProjectionExpression)
	for name, exp := range summaryExpressionAttributeNames {
		queryInput.ExpressionAttributeNames[name] = exp
	}
}
