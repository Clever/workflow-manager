package dynamodb

import (
	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

type ddbStateResourcePrimaryKey struct {
	Name      string `dynamodbav:"name"`
	Namespace string `dynamodbav:"namespace"`
}

func (pk ddbStateResourcePrimaryKey) AttributeDefinitions() []*dynamodb.AttributeDefinition {
	return []*dynamodb.AttributeDefinition{
		{
			AttributeName: aws.String("name"),
			AttributeType: aws.String(dynamodb.ScalarAttributeTypeS),
		},
		{
			AttributeName: aws.String("namespace"),
			AttributeType: aws.String(dynamodb.ScalarAttributeTypeS),
		},
	}
}

func (pk ddbStateResourcePrimaryKey) KeySchema() []*dynamodb.KeySchemaElement {
	return []*dynamodb.KeySchemaElement{
		{
			AttributeName: aws.String("name"),
			KeyType:       aws.String(dynamodb.KeyTypeHash),
		},
		{
			AttributeName: aws.String("namespace"),
			KeyType:       aws.String(dynamodb.KeyTypeRange),
		},
	}
}

type ddbStateResource struct {
	ddbStateResourcePrimaryKey
	StateResource models.StateResource
}

// EncodeStateResource encodes a StateResource into a dyanmo attribute map
func EncodeStateResource(resource models.StateResource) (map[string]*dynamodb.AttributeValue, error) {
	return dynamodbattribute.MarshalMap(ddbStateResource{
		ddbStateResourcePrimaryKey: ddbStateResourcePrimaryKey{
			Name:      resource.Name,
			Namespace: resource.Namespace,
		},
		StateResource: resource,
	})
}

// DecodeStateResource translates a StateResource stored in dynamdb to a StateResource object
func DecodeStateResource(m map[string]*dynamodb.AttributeValue) (models.StateResource, error) {
	var res ddbStateResource
	if err := dynamodbattribute.UnmarshalMap(m, &res); err != nil {
		return models.StateResource{}, err
	}
	return res.StateResource, nil
}
