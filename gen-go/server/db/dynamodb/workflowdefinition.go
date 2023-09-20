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
	"github.com/go-openapi/strfmt"
)

var _ = strfmt.DateTime{}

// WorkflowDefinitionTable represents the user-configurable properties of the WorkflowDefinition table.
type WorkflowDefinitionTable struct {
	DynamoDBAPI        dynamodbiface.DynamoDBAPI
	Prefix             string
	TableName          string
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
	models.WorkflowDefinition
}

func (t WorkflowDefinitionTable) name() string {
	if t.TableName != "" {
		return t.TableName
	}
	return fmt.Sprintf("%s-workflow-definitions", t.Prefix)
}

func (t WorkflowDefinitionTable) create(ctx context.Context) error {
	if _, err := t.DynamoDBAPI.CreateTableWithContext(ctx, &dynamodb.CreateTableInput{
		AttributeDefinitions: []*dynamodb.AttributeDefinition{
			{
				AttributeName: aws.String("name"),
				AttributeType: aws.String("S"),
			},
			{
				AttributeName: aws.String("version"),
				AttributeType: aws.String("N"),
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
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case dynamodb.ErrCodeConditionalCheckFailedException:
				return db.ErrWorkflowDefinitionAlreadyExists{
					Name:    m.Name,
					Version: m.Version,
				}
			case dynamodb.ErrCodeResourceNotFoundException:
				return fmt.Errorf("table or index not found: %s", t.name())
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
		Key:            key,
		TableName:      aws.String(t.name()),
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case dynamodb.ErrCodeResourceNotFoundException:
				return nil, fmt.Errorf("table or index not found: %s", t.name())
			}
		}
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

func (t WorkflowDefinitionTable) getWorkflowDefinitionsByNameAndVersion(ctx context.Context, input db.GetWorkflowDefinitionsByNameAndVersionInput, fn func(m *models.WorkflowDefinition, lastWorkflowDefinition bool) bool) error {
	if input.VersionStartingAt != nil && input.StartingAfter != nil {
		return fmt.Errorf("Can specify only one of input.VersionStartingAt or input.StartingAfter")
	}
	if input.Name == "" {
		return fmt.Errorf("Hash key input.Name cannot be empty")
	}
	queryInput := &dynamodb.QueryInput{
		TableName: aws.String(t.name()),
		ExpressionAttributeNames: map[string]*string{
			"#NAME": aws.String("name"),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":name": {
				S: aws.String(input.Name),
			},
		},
		ScanIndexForward: aws.Bool(!input.Descending),
		ConsistentRead:   aws.Bool(!input.DisableConsistentRead),
	}
	if input.Limit != nil {
		queryInput.Limit = input.Limit
	}
	if input.VersionStartingAt == nil {
		queryInput.KeyConditionExpression = aws.String("#NAME = :name")
	} else {
		queryInput.ExpressionAttributeNames["#VERSION"] = aws.String("version")
		queryInput.ExpressionAttributeValues[":version"] = &dynamodb.AttributeValue{
			N: aws.String(fmt.Sprintf("%d", *input.VersionStartingAt)),
		}
		if input.Descending {
			queryInput.KeyConditionExpression = aws.String("#NAME = :name AND #VERSION <= :version")
		} else {
			queryInput.KeyConditionExpression = aws.String("#NAME = :name AND #VERSION >= :version")
		}
	}
	if input.StartingAfter != nil {
		queryInput.ExclusiveStartKey = map[string]*dynamodb.AttributeValue{
			"version": {
				N: aws.String(fmt.Sprintf("%d", input.StartingAfter.Version)),
			},
			"name": {
				S: aws.String(input.StartingAfter.Name),
			},
		}
	}

	totalRecordsProcessed := int64(0)
	var pageFnErr error
	pageFn := func(queryOutput *dynamodb.QueryOutput, lastPage bool) bool {
		if len(queryOutput.Items) == 0 {
			return false
		}
		items, err := decodeWorkflowDefinitions(queryOutput.Items)
		if err != nil {
			pageFnErr = err
			return false
		}
		hasMore := true
		for i := range items {
			if lastPage == true {
				hasMore = i < len(items)-1
			}
			if !fn(&items[i], !hasMore) {
				return false
			}
			totalRecordsProcessed++
			// if the Limit of records have been passed to fn, don't pass anymore records.
			if input.Limit != nil && totalRecordsProcessed == *input.Limit {
				return false
			}
		}
		return true
	}

	err := t.DynamoDBAPI.QueryPagesWithContext(ctx, queryInput, pageFn)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case dynamodb.ErrCodeResourceNotFoundException:
				return fmt.Errorf("table or index not found: %s", t.name())
			}
		}
		return err
	}
	if pageFnErr != nil {
		return pageFnErr
	}

	return nil
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
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case dynamodb.ErrCodeResourceNotFoundException:
				return fmt.Errorf("table or index not found: %s", t.name())
			}
		}
		return err
	}

	return nil
}

// encodeWorkflowDefinition encodes a WorkflowDefinition as a DynamoDB map of attribute values.
func encodeWorkflowDefinition(m models.WorkflowDefinition) (map[string]*dynamodb.AttributeValue, error) {
	return dynamodbattribute.MarshalMap(ddbWorkflowDefinition{
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

// decodeWorkflowDefinitions translates a list of WorkflowDefinitions stored in DynamoDB to a slice of WorkflowDefinition structs.
func decodeWorkflowDefinitions(ms []map[string]*dynamodb.AttributeValue) ([]models.WorkflowDefinition, error) {
	workflowDefinitions := make([]models.WorkflowDefinition, len(ms))
	for i, m := range ms {
		var workflowDefinition models.WorkflowDefinition
		if err := decodeWorkflowDefinition(m, &workflowDefinition); err != nil {
			return nil, err
		}
		workflowDefinitions[i] = workflowDefinition
	}
	return workflowDefinitions, nil
}
