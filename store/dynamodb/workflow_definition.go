package dynamodb

import (
	"bytes"
	"encoding/gob"

	"github.com/Clever/workflow-manager/resources"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

// ddbWorkflowDefinitionPrimaryKey represents the primary key of the workflow defintiions table in dynamo.
// Use this to make GetItem queries.
type ddbWorkflowDefinitionPrimaryKey struct {
	Name    string `dynamodbav:"name"`
	Version int    `dynamodbav:"version"`
}

// ddbWorkflowDefinition represents the workflow definition as stored in dynamo.
// Use this to make PutItem queries.
type ddbWorkflowDefinition struct {
	ddbWorkflowDefinitionPrimaryKey
	WorkflowDefinition []byte `dynamodbav:"workflow"`
}

// EncodeWorkflowDefinition encodes a WorkflowDefinition as a dynamo attribute map.
// Since WorkflowDefinitions contain interface types, the main piece of
// the encoding is a full gob-encoding of the WorkflowDefinition.
func EncodeWorkflowDefinition(def resources.WorkflowDefinition) (map[string]*dynamodb.AttributeValue, error) {
	var defGOB bytes.Buffer
	if err := gob.NewEncoder(&defGOB).Encode(def); err != nil {
		return nil, err
	}
	return dynamodbattribute.MarshalMap(ddbWorkflowDefinition{
		ddbWorkflowDefinitionPrimaryKey: ddbWorkflowDefinitionPrimaryKey{
			Name:    def.Name(),
			Version: def.Version(),
		},
		WorkflowDefinition: defGOB.Bytes(),
	})
}

// DecodeWorkflowDefinition translates the WorkflowDefinition stored in dynamodb to a WorkflowDefinition object.
func DecodeWorkflowDefinition(m map[string]*dynamodb.AttributeValue, out *resources.WorkflowDefinition) error {
	var ddbWorkflowDefinition ddbWorkflowDefinition
	if err := dynamodbattribute.UnmarshalMap(m, &ddbWorkflowDefinition); err != nil {
		return err
	}
	wfBuf := bytes.NewBuffer(ddbWorkflowDefinition.WorkflowDefinition)
	return gob.NewDecoder(wfBuf).Decode(out)
}
