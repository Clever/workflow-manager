package dynamodb

import (
	"bytes"
	"encoding/gob"
	"fmt"

	"github.com/Clever/workflow-manager/resources"
	"github.com/Clever/workflow-manager/store"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
)

type DynamoDB struct {
	ddb             dynamodbiface.DynamoDBAPI
	tableNamePrefix string
}

func New(ddb dynamodbiface.DynamoDBAPI, tableNamePrefix string) DynamoDB {
	return DynamoDB{
		ddb:             ddb,
		tableNamePrefix: tableNamePrefix,
	}
}

func (d DynamoDB) WorkflowsTable() string {
	return fmt.Sprintf("%s-workflows", d.tableNamePrefix)
}

func (d DynamoDB) JobsTable() string {
	return fmt.Sprintf("%s-jobs", d.tableNamePrefix)
}

func (d DynamoDB) InitTables() error {
	// create workflows table from name, version -> workflow object
	if _, err := d.ddb.CreateTable(&dynamodb.CreateTableInput{
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
			ReadCapacityUnits:  aws.Int64(1),
			WriteCapacityUnits: aws.Int64(1),
		},
		TableName: aws.String(d.WorkflowsTable()),
	}); err != nil {
		return err
	}

	// create jobs table from job ID to to job object
	if _, err := d.ddb.CreateTable(&dynamodb.CreateTableInput{
		AttributeDefinitions: []*dynamodb.AttributeDefinition{
			{
				AttributeName: aws.String("id"),
				AttributeType: aws.String("S"),
			},
		},
		KeySchema: []*dynamodb.KeySchemaElement{
			{
				AttributeName: aws.String("id"),
				KeyType:       aws.String(dynamodb.KeyTypeHash),
			},
		},
		ProvisionedThroughput: &dynamodb.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(1),
			WriteCapacityUnits: aws.Int64(1),
		},
		TableName: aws.String(d.JobsTable()),
	}); err != nil {
		return err
	}
	return nil
}

type dynamodbWorkflow struct {
	Name     string `dynamodbav:"name"`
	Version  int    `dynamodbav:"version"`
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
	return dynamodbattribute.MarshalMap(dynamodbWorkflow{
		Name:     def.Name(),
		Version:  def.Version(),
		Workflow: defGOB.Bytes(),
	})
}

// DecodeWorkflow translates the WorkflowDefinition stored in dynamodb to a WorkflowDefinition object.
func DecodeWorkflow(m map[string]*dynamodb.AttributeValue, out *resources.WorkflowDefinition) error {
	var ddbWorkflow dynamodbWorkflow
	if err := dynamodbattribute.UnmarshalMap(m, &ddbWorkflow); err != nil {
		return err
	}
	wfBuf := bytes.NewBuffer(ddbWorkflow.Workflow)
	return gob.NewDecoder(wfBuf).Decode(out)
}

func (d DynamoDB) SaveWorkflow(def resources.WorkflowDefinition) error {
	data, err := EncodeWorkflow(def)
	if err != nil {
		return err
	}

	_, err = d.ddb.PutItem(&dynamodb.PutItemInput{
		TableName: aws.String(d.WorkflowsTable()),
		Item:      data,
	})

	return nil
}
func (d DynamoDB) UpdateWorkflow(def resources.WorkflowDefinition) (resources.WorkflowDefinition, error) {
	panic("implement " + "UpdateWorkflow")
	return resources.WorkflowDefinition{}, nil
}
func (d DynamoDB) GetWorkflow(name string, version int) (resources.WorkflowDefinition, error) {
	panic("implement " + "GetWorkflow")
	return resources.WorkflowDefinition{}, nil
}

func (d DynamoDB) LatestWorkflow(name string) (resources.WorkflowDefinition, error) {
	res, err := d.ddb.Query(&dynamodb.QueryInput{
		TableName: aws.String(d.WorkflowsTable()),
		ExpressionAttributeNames: map[string]*string{
			"#N": aws.String("name"),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":name": &dynamodb.AttributeValue{
				S: aws.String(name),
			},
		},
		KeyConditionExpression: aws.String("#N = :name"),
		Limit:            aws.Int64(1),
		ConsistentRead:   aws.Bool(true),
		ScanIndexForward: aws.Bool(false), // descending order
	})
	if err != nil {
		return resources.WorkflowDefinition{}, err
	}
	if len(res.Items) != 1 {
		return resources.WorkflowDefinition{}, store.NewNotFound(name)
	}
	var wf resources.WorkflowDefinition
	if err := DecodeWorkflow(res.Items[0], &wf); err != nil {
		return resources.WorkflowDefinition{}, err
	}
	return wf, nil
}

func (d DynamoDB) SaveJob(job resources.Job) error {
	panic("implement " + "SaveJob")
	return nil
}
func (d DynamoDB) UpdateJob(job resources.Job) error {
	panic("implement " + "UpdateJob")
	return nil
}
func (d DynamoDB) GetJob(id string) (resources.Job, error) {
	panic("implement " + "GetJob")
	return resources.Job{}, nil

}
func (d DynamoDB) GetJobsForWorkflow(workflowName string) ([]resources.Job, error) {
	panic("implement " + "GetJobsForWorkflow")
	return nil, nil
}
