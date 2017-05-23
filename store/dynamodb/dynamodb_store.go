package dynamodb

import (
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
		AttributeDefinitions: append(
			ddbJobPrimaryKey{}.AttributeDefinitions(),
			ddbJobSecondaryKeyWorkflowCreatedAt{}.AttributeDefinitions()...,
		),
		KeySchema: ddbJobPrimaryKey{}.KeySchema(),
		GlobalSecondaryIndexes: []*dynamodb.GlobalSecondaryIndex{
			{
				IndexName: aws.String(ddbJobSecondaryKeyWorkflowCreatedAt{}.Name()),
				KeySchema: ddbJobSecondaryKeyWorkflowCreatedAt{}.KeySchema(),
				Projection: &dynamodb.Projection{
					ProjectionType: aws.String(dynamodb.ProjectionTypeAll),
				},
				ProvisionedThroughput: &dynamodb.ProvisionedThroughput{
					ReadCapacityUnits:  aws.Int64(1),
					WriteCapacityUnits: aws.Int64(1),
				},
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

// TODO
func (d DynamoDB) GetWorkflows() ([]resources.WorkflowDefinition, error) {
	return nil, nil
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
	job.LastUpdated = time.Now()

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
		ConditionExpression: aws.String("attribute_exists(#I)"),
	})
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			if awsErr.Code() == dynamodb.ErrCodeConditionalCheckFailedException {
				return store.NewNotFound(job.ID)
			}
		}
	}
	return err
}

// populateJob fills in the workflow object contained in a job
func (d DynamoDB) populateJob(job *resources.Job) error {
	wf, err := d.GetWorkflow(job.Workflow.Name(), job.Workflow.Version())
	if err != nil {
		return fmt.Errorf("error getting workflow for job %s: %s", job.ID, err)
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
		Key:            key,
		TableName:      aws.String(d.jobsTable()),
		ConsistentRead: aws.Bool(true),
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

// GetJobsForWorkflow returns the last 10 jobs for a workflow.
// It uses a global secondary index on the workflow name + created time for a job.
func (d DynamoDB) GetJobsForWorkflow(workflowName string) ([]resources.Job, error) {
	var jobs []resources.Job
	query := ddbJobSecondaryKeyWorkflowCreatedAt{
		WorkflowName: workflowName,
	}.ConstructQuery()

	query.TableName = aws.String(d.jobsTable())
	query.Limit = aws.Int64(10)
	query.ScanIndexForward = aws.Bool(false) // descending order

	res, err := d.ddb.Query(query)
	if err != nil {
		return jobs, err
	}

	for _, item := range res.Items {
		job, err := DecodeJob(item)
		if err != nil {
			return jobs, err
		}
		if err := d.populateJob(&job); err != nil {
			return jobs, err
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}
