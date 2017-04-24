package dynamodb

import (
	"bytes"
	"encoding/gob"

	"github.com/Clever/workflow-manager/resources"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

// ddbWorkflowPrimaryKey represents the primary key of the workflow defintiions table in dynamo.
// Use this to make GetItem queries.
type ddbWorkflowPrimaryKey struct {
	Name    string `dynamodbav:"name"`
	Version int    `dynamodbav:"version"`
}

// ddbWorkflow represents the workflow definition as stored in dynamo.
// Use this to make PutItem queries.
type ddbWorkflow struct {
	ddbWorkflowPrimaryKey
	Workflow []byte `dynamodbav:"workflow"`
}

// EncodeWorkflow encodes a WorkflowDefinition as a dynamo attribute map.
// Since WorkflowDefinitions contain interface types, the main piece of
// the encoding is a full gob-encoding of the WorkflowDefinition.
func EncodeWorkflow(def resources.WorkflowDefinition) (map[string]*dynamodb.AttributeValue, error) {
	var defGOB bytes.Buffer
	if err := gob.NewEncoder(&defGOB).Encode(def); err != nil {
		return nil, err
	}
	return dynamodbattribute.MarshalMap(ddbWorkflow{
		ddbWorkflowPrimaryKey: ddbWorkflowPrimaryKey{
			Name:    def.Name(),
			Version: def.Version(),
		},
		Workflow: defGOB.Bytes(),
	})
}

// DecodeWorkflow translates the WorkflowDefinition stored in dynamodb to a WorkflowDefinition object.
func DecodeWorkflow(m map[string]*dynamodb.AttributeValue, out *resources.WorkflowDefinition) error {
	var ddbWorkflow ddbWorkflow
	if err := dynamodbattribute.UnmarshalMap(m, &ddbWorkflow); err != nil {
		return err
	}
	wfBuf := bytes.NewBuffer(ddbWorkflow.Workflow)
	return gob.NewDecoder(wfBuf).Decode(out)
}
