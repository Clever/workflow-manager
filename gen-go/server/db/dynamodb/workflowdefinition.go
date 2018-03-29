package dynamodb

import (
	"context"
	"fmt"

	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/Clever/workflow-manager/gen-go/server/db"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
)

// WorkflowDefinitionTable represents the user-configurable properties of the WorkflowDefinition table.
type WorkflowDefinitionTable struct {
	DynamoDBAPI        dynamodbiface.DynamoDBAPI
	Prefix             string
	ReadCapacityUnits  int64
	WriteCapacityUnits int64
}

// ddbWorkflowDefinitionPrimaryKey represents the primary key of a WorkflowDefinition in DynamoDB.
type ddbWorkflowDefinitionPrimaryKey struct {
	Name    string `dynamodbav:"name"`
	Version int64  `dynamodbav:"version"`
}

// ddbWorkflowDefinition represents a WorkflowDefinition as stored in DynamoDB.
type ddbWorkflowDefinition struct {
	ddbWorkflowDefinitionPrimaryKey
	WorkflowDefinition models.WorkflowDefinition `dynamodbav:"workflow-definition"`
}

func (t WorkflowDefinitionTable) name() string {
	return fmt.Sprintf("%s-workflow-definitions", t.Prefix)
}

func (t WorkflowDefinitionTable) create(ctx context.Context) error {
	if _, err := t.DynamoDBAPI.CreateTableWithContext(ctx, &dynamodb.CreateTableInput{
		AttributeDefinitions: []*dynamodb.AttributeDefinition{
			{
				AttributeName: aws.String("name"),
				AttributeType: aws.String(dynamodb.ScalarAttributeTypeS),
			},
			{
				AttributeName: aws.String("version"),
				AttributeType: aws.String(dynamodb.ScalarAttributeTypeN),
			},
		},
		KeySchema: []*dynamodb.KeySchemaElement{
			{
				AttributeName: aws.String("name"),
				KeyType:       aws.String(dynamodb.KeyTypeHash),
			},
			{
				AttributeName: aws.String("version"),
				KeyType:       aws.String(dynamodb.KeyTypeRange),
			},
		},
		ProvisionedThroughput: &dynamodb.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(t.ReadCapacityUnits),
			WriteCapacityUnits: aws.Int64(t.WriteCapacityUnits),
		},
		TableName: aws.String(t.name()),
	}); err != nil {
		return err
	}
	return nil
}

func (t WorkflowDefinitionTable) saveWorkflowDefinition(ctx context.Context, m models.WorkflowDefinition) error {
	data, err := encodeWorkflowDefinition(m)
	if err != nil {
		return err
	}
	_, err = t.DynamoDBAPI.PutItemWithContext(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(t.name()),
		Item:      data,
		ExpressionAttributeNames: map[string]*string{
			"#NAME":    aws.String("name"),
			"#VERSION": aws.String("version"),
		},
		ConditionExpression: aws.String("attribute_not_exists(#NAME) AND attribute_not_exists(#VERSION)"),
	})
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			if awsErr.Code() == dynamodb.ErrCodeConditionalCheckFailedException {
				return db.ErrWorkflowDefinitionAlreadyExists{
					Name:    m.Name,
					Version: m.Version,
				}
			}
		}
		return err
	}
	return nil
}

func (t WorkflowDefinitionTable) getWorkflowDefinition(ctx context.Context, name string, version int64) (*models.WorkflowDefinition, error) {
	key, err := dynamodbattribute.MarshalMap(ddbWorkflowDefinitionPrimaryKey{
		Name:    name,
		Version: version,
	})
	if err != nil {
		return nil, err
	}
	res, err := t.DynamoDBAPI.GetItemWithContext(ctx, &dynamodb.GetItemInput{
		Key:       key,
		TableName: aws.String(t.name()),
	})
	if err != nil {
		return nil, err
	}

	if len(res.Item) == 0 {
		return nil, db.ErrWorkflowDefinitionNotFound{
			Name:    name,
			Version: version,
		}
	}

	var m models.WorkflowDefinition
	if err := decodeWorkflowDefinition(res.Item, &m); err != nil {
		return nil, err
	}

	return &m, nil
}

func (t WorkflowDefinitionTable) deleteWorkflowDefinition(ctx context.Context, name string, version int64) error {
	key, err := dynamodbattribute.MarshalMap(ddbWorkflowDefinitionPrimaryKey{
		Name:    name,
		Version: version,
	})
	if err != nil {
		return err
	}
	_, err = t.DynamoDBAPI.DeleteItemWithContext(ctx, &dynamodb.DeleteItemInput{
		Key:       key,
		TableName: aws.String(t.name()),
	})
	if err != nil {
		return err
	}
	return nil
}

// encodeWorkflowDefinition encodes a WorkflowDefinition as a DynamoDB map of attribute values.
func encodeWorkflowDefinition(m models.WorkflowDefinition) (map[string]*dynamodb.AttributeValue, error) {
	return dynamodbattribute.MarshalMap(ddbWorkflowDefinition{
		ddbWorkflowDefinitionPrimaryKey: ddbWorkflowDefinitionPrimaryKey{
			Name:    m.Name,
			Version: m.Version,
		},
		WorkflowDefinition: m,
	})
}

// decodeWorkflowDefinition translates a WorkflowDefinition stored in DynamoDB to a WorkflowDefinition struct.
func decodeWorkflowDefinition(m map[string]*dynamodb.AttributeValue, out *models.WorkflowDefinition) error {
	var ddbWorkflowDefinition ddbWorkflowDefinition
	if err := dynamodbattribute.UnmarshalMap(m, &ddbWorkflowDefinition); err != nil {
		return err
	}
	*out = ddbWorkflowDefinition.WorkflowDefinition
	return nil
}
