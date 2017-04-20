package dynamodb

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"time"

	"github.com/Clever/workflow-manager/resources"
	"github.com/Clever/workflow-manager/store"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
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

// workflowsTable returns the name of the table that stores workflow definitions.
func (d DynamoDB) workflowsTable() string {
	return fmt.Sprintf("%s-workflows", d.tableNamePrefix)
}

// jobsTable returns the name of the table that stores jobs.
func (d DynamoDB) jobsTable() string {
	return fmt.Sprintf("%s-jobs", d.tableNamePrefix)
}

// InitTables creates the dynamo tables.
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
		TableName: aws.String(d.workflowsTable()),
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
		TableName: aws.String(d.jobsTable()),
	}); err != nil {
		return err
	}
	return nil
}

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

// ddbJobPrimaryKey represents the primary key of the jobs table in dynamo.
// Use this to make GetItem queries.
type ddbJobPrimaryKey struct {
	ID string `dynamodbav:"id"`
}

// ddbJob represents the job as stored in dynamo.
// Use this to make PutItem queries.
type ddbJob struct {
	ddbJobPrimaryKey
	CreatedAt   time.Time             `dynamodbav:"createdAt"`
	LastUpdated time.Time             `dynamodbav:"lastUpdated"`
	Workflow    ddbWorkflowPrimaryKey `dynamodbav:"workflow"`
	Input       []string              `dynamodbav:"input"`
	Tasks       []*resources.Task     `dynamodbav:"tasks"`
	Status      resources.JobStatus   `dynamodbav:"status"`
}

// TODO definte GSI with partition key on workflow name, sort key on createdAt time?

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

// EncodeJob encodes a Job as a dynamo attribute map.
func EncodeJob(job resources.Job) (map[string]*dynamodb.AttributeValue, error) {
	return dynamodbattribute.MarshalMap(ddbJob{
		ddbJobPrimaryKey: ddbJobPrimaryKey{
			ID: job.ID,
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

// SaveWorkflow saves a workflow definition.
// If the workflow already exists, it will return a store.ConflictError.
func (d DynamoDB) SaveWorkflow(def resources.WorkflowDefinition) error {
	def.CreatedAt = time.Now()

	data, err := EncodeWorkflow(def)
	if err != nil {
		return err
	}

	_, err = d.ddb.PutItem(&dynamodb.PutItemInput{
		TableName: aws.String(d.workflowsTable()),
		Item:      data,
		ExpressionAttributeNames: map[string]*string{
			"#N": aws.String("name"),
			"#V": aws.String("version"),
		},
		ConditionExpression: aws.String("attribute_not_exists(#N) AND attribute_not_exists(#V)"),
	})
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			if awsErr.Code() == dynamodb.ErrCodeConditionalCheckFailedException {
				return store.NewConflict(def.Name())
			}
		}
	}
	return err
}

// UpdateWorkflow updates an existing workflow definition.
// The version will be set to the version following the latest definition.
// The workflow definition returned contains this new version number.
func (d DynamoDB) UpdateWorkflow(def resources.WorkflowDefinition) (resources.WorkflowDefinition, error) {
	latest, err := d.LatestWorkflow(def.Name()) // TODO: only need version here, can optimize query
	if err != nil {
		return def, err
	}

	// TODO: this isn't thread safe...
	newVersion := resources.NewWorkflowDefinitionVersion(def, latest.Version()+1)
	if err := d.SaveWorkflow(newVersion); err != nil {
		return def, err
	}

	// need to perform a get to return any mutations that happened in Save, e.g. CreatedAt
	return d.GetWorkflow(newVersion.Name(), newVersion.Version())
}

func (d DynamoDB) GetWorkflow(name string, version int) (resources.WorkflowDefinition, error) {
	key, err := dynamodbattribute.MarshalMap(ddbWorkflowPrimaryKey{
		Name:    name,
		Version: version,
	})
	if err != nil {
		return resources.WorkflowDefinition{}, err
	}
	res, err := d.ddb.GetItem(&dynamodb.GetItemInput{
		Key:       key,
		TableName: aws.String(d.workflowsTable()),
	})
	if err != nil {
		return resources.WorkflowDefinition{}, err
	}

	if len(res.Item) == 0 {
		return resources.WorkflowDefinition{}, store.NewNotFound(name)
	}

	var wd resources.WorkflowDefinition
	if err := DecodeWorkflow(res.Item, &wd); err != nil {
		return resources.WorkflowDefinition{}, err
	}

	return wd, nil
}

// LatestWorkflow gets the latest version of a workflow definition.
func (d DynamoDB) LatestWorkflow(name string) (resources.WorkflowDefinition, error) {
	res, err := d.ddb.Query(&dynamodb.QueryInput{
		TableName: aws.String(d.workflowsTable()),
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

// SaveJob saves a job to dynamo.
func (d DynamoDB) SaveJob(job resources.Job) error {
	job.CreatedAt = time.Now()
	job.LastUpdated = job.CreatedAt

	data, err := EncodeJob(job)
	if err != nil {
		return err
	}
	_, err = d.ddb.PutItem(&dynamodb.PutItemInput{
		TableName: aws.String(d.jobsTable()),
		Item:      data,
		ExpressionAttributeNames: map[string]*string{
			"#I": aws.String("id"),
		},
		ConditionExpression: aws.String("attribute_not_exists(#I)"),
	})
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			if awsErr.Code() == dynamodb.ErrCodeConditionalCheckFailedException {
				return store.NewConflict(job.ID)
			}
		}
	}
	return err
}
func (d DynamoDB) UpdateJob(job resources.Job) error {
	panic("implement " + "UpdateJob")
	return nil
}

// populateJob fills in the workflow object contained in a job
func (d DynamoDB) populateJob(job *resources.Job) error {
	wf, err := d.GetWorkflow(job.Workflow.Name(), job.Workflow.Version())
	if err != nil {
		return err
	}
	job.Workflow = wf
	return nil
}

func (d DynamoDB) GetJob(id string) (resources.Job, error) {
	key, err := dynamodbattribute.MarshalMap(ddbJobPrimaryKey{
		ID: id,
	})
	if err != nil {
		return resources.Job{}, err
	}
	res, err := d.ddb.GetItem(&dynamodb.GetItemInput{
		Key:       key,
		TableName: aws.String(d.jobsTable()),
	})
	if err != nil {
		return resources.Job{}, err
	}

	if len(res.Item) == 0 {
		return resources.Job{}, store.NewNotFound(id)
	}

	job, err := DecodeJob(res.Item)
	if err != nil {
		return resources.Job{}, err
	}

	if err := d.populateJob(&job); err != nil {
		return resources.Job{}, err
	}

	return job, nil
}

func (d DynamoDB) GetJobsForWorkflow(workflowName string) ([]resources.Job, error) {
	panic("implement " + "GetJobsForWorkflow")
	return nil, nil
}
