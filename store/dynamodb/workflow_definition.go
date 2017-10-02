package dynamodb

import (
	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

// ddbWorkflowDefinitionPrimaryKey represents the primary key of the workflow defintiions table in dynamo.
// Use this to make GetItem queries.
type ddbWorkflowDefinitionPrimaryKey struct {
	Name    string `dynamodbav:"name"`
	Version int64  `dynamodbav:"version"`
}

// ddbWorkflowDefinition represents the workflow definition as stored in dynamo.
// Use this to make PutItem queries.
type ddbWorkflowDefinition struct {
	ddbWorkflowDefinitionPrimaryKey
	WorkflowDefinition models.WorkflowDefinition `dynamodbav:"workflow-definition"`
}

// EncodeWorkflowDefinition encodes a WorkflowDefinition as a dynamo attribute map.
func EncodeWorkflowDefinition(def models.WorkflowDefinition) (map[string]*dynamodb.AttributeValue, error) {
	return dynamodbattribute.MarshalMap(ddbWorkflowDefinition{
		ddbWorkflowDefinitionPrimaryKey: ddbWorkflowDefinitionPrimaryKey{
			Name:    def.Name,
			Version: def.Version,
		},
		WorkflowDefinition: def,
	})
}

// DecodeWorkflowDefinition translates the WorkflowDefinition stored in dynamodb to a WorkflowDefinition object.
func DecodeWorkflowDefinition(m map[string]*dynamodb.AttributeValue, out *models.WorkflowDefinition) error {
	var ddbWorkflowDefinition ddbWorkflowDefinition
	if err := dynamodbattribute.UnmarshalMap(m, &ddbWorkflowDefinition); err != nil {
		return err
	}
	*out = ddbWorkflowDefinition.WorkflowDefinition
	return nil
}
