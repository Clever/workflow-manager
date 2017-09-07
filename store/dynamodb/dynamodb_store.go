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

// latestWorkflowDefinitionsTable returns the name of the table that stores the latest version of every WorkflowDefinition
func (d DynamoDB) latestWorkflowDefinitionsTable() string {
	// TODO: Actually rename table internally
	return fmt.Sprintf("%s-latest-workflows", d.tableNamePrefix)
}

// workflowDefinitionsTable returns the name of the table that stores all WorkflowDefinitions
func (d DynamoDB) workflowDefinitionsTable() string {
	// TODO: Actually rename table internally
	return fmt.Sprintf("%s-workflows", d.tableNamePrefix)
}

// jobsTable returns the name of the table that stores jobs.
func (d DynamoDB) jobsTable() string {
	return fmt.Sprintf("%s-jobs", d.tableNamePrefix)
}

// stateResourcesTable returns the name of the table that stores stateResources.
func (d DynamoDB) stateResourcesTable() string {
	return fmt.Sprintf("%s-state-resources", d.tableNamePrefix)
}

// dynamoItemsToWorkflowDefinitions takes the Items from a Query or Scan result and decodes it into an array of workflow definitions
func (d DynamoDB) dynamoItemsToWorkflowDefinitions(items []map[string]*dynamodb.AttributeValue) ([]resources.WorkflowDefinition, error) {
	workflowDefinitions := []resources.WorkflowDefinition{}

	for _, item := range items {
		var wf resources.WorkflowDefinition
		if err := DecodeWorkflowDefinition(item, &wf); err != nil {
			return []resources.WorkflowDefinition{}, err
		}
		workflowDefinitions = append(workflowDefinitions, wf)
	}

	return workflowDefinitions, nil
}

// InitTables creates the dynamo tables.
func (d DynamoDB) InitTables() error {
	// create workflowDefinitions table from name, version -> workflow object
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
		TableName: aws.String(d.workflowDefinitionsTable()),
	}); err != nil {
		return err
	}

	// create latest workflowDefinitions table from name -> workflow object
	if _, err := d.ddb.CreateTable(&dynamodb.CreateTableInput{
		AttributeDefinitions: []*dynamodb.AttributeDefinition{
			{
				AttributeName: aws.String("name"),
				AttributeType: aws.String(dynamodb.ScalarAttributeTypeS),
			},
		},
		KeySchema: []*dynamodb.KeySchemaElement{
			{
				AttributeName: aws.String("name"),
				KeyType:       aws.String(dynamodb.KeyTypeHash),
			},
		},
		ProvisionedThroughput: &dynamodb.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(1),
			WriteCapacityUnits: aws.Int64(1),
		},
		TableName: aws.String(d.latestWorkflowDefinitionsTable()),
	}); err != nil {
		return err
	}

	// create jobs table from job ID -> job object
	if _, err := d.ddb.CreateTable(&dynamodb.CreateTableInput{
		AttributeDefinitions: append(
			ddbJobPrimaryKey{}.AttributeDefinitions(),
			ddbJobSecondaryKeyWorkflowDefinitionCreatedAt{}.AttributeDefinitions()...,
		),
		KeySchema: ddbJobPrimaryKey{}.KeySchema(),
		GlobalSecondaryIndexes: []*dynamodb.GlobalSecondaryIndex{
			{
				IndexName: aws.String(ddbJobSecondaryKeyWorkflowDefinitionCreatedAt{}.Name()),
				KeySchema: ddbJobSecondaryKeyWorkflowDefinitionCreatedAt{}.KeySchema(),
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

	// create state-resources table from stateResource.{name, namespace} -> stateResource object
	if _, err := d.ddb.CreateTable(&dynamodb.CreateTableInput{
		AttributeDefinitions: ddbStateResourcePrimaryKey{}.AttributeDefinitions(),
		KeySchema:            ddbStateResourcePrimaryKey{}.KeySchema(),
		ProvisionedThroughput: &dynamodb.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(1),
			WriteCapacityUnits: aws.Int64(1),
		},
		TableName: aws.String(d.stateResourcesTable()),
	}); err != nil {
		return err
	}

	return nil
}

// SaveWorkflowDefinition saves a workflow definition.
// If the workflow already exists, it will return a store.ConflictError.
func (d DynamoDB) SaveWorkflowDefinition(def resources.WorkflowDefinition) error {
	def.CreatedAtTime = time.Now()

	data, err := EncodeWorkflowDefinition(def)
	if err != nil {
		return err
	}

	_, err = d.ddb.PutItem(&dynamodb.PutItemInput{
		TableName: aws.String(d.workflowDefinitionsTable()),
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
		return err
	}
	_, err = d.ddb.PutItem(&dynamodb.PutItemInput{
		TableName: aws.String(d.latestWorkflowDefinitionsTable()),
		Item:      data,
	})
	// TODO determine what we should do in case the 2nd write fails
	return err
}

// UpdateWorkflowDefinition updates an existing workflow definition.
// The version will be set to the version following the latest definition.
// The workflow definition returned contains this new version number.
func (d DynamoDB) UpdateWorkflowDefinition(def resources.WorkflowDefinition) (resources.WorkflowDefinition, error) {
	// TODO we should change this to use the latest-workflows table with a projection expressions
	latest, err := d.LatestWorkflowDefinition(def.Name()) // TODO: only need version here, can optimize query
	if err != nil {
		return def, err
	}

	// TODO: this isn't thread safe...
	newVersion := resources.NewWorkflowDefinitionVersion(def, latest.Version()+1)
	if err := d.SaveWorkflowDefinition(newVersion); err != nil {
		return def, err
	}

	// need to perform a get to return any mutations that happened in Save, e.g. CreatedAt
	return d.GetWorkflowDefinition(newVersion.Name(), newVersion.Version())
}

// GetWorkflowDefinitions returns the latest version of all stored workflow definitions
func (d DynamoDB) GetWorkflowDefinitions() ([]resources.WorkflowDefinition, error) {
	// Scan returns the entire table
	results, err := d.ddb.Scan(&dynamodb.ScanInput{
		ConsistentRead: aws.Bool(true),
		TableName:      aws.String(d.latestWorkflowDefinitionsTable()),
	})
	if err != nil {
		return []resources.WorkflowDefinition{}, err
	}
	return d.dynamoItemsToWorkflowDefinitions(results.Items)
}

// GetWorkflowDefinitionVersions gets all versions of a workflow definition
func (d DynamoDB) GetWorkflowDefinitionVersions(name string) ([]resources.WorkflowDefinition, error) {
	results, err := d.ddb.Query(&dynamodb.QueryInput{
		TableName: aws.String(d.workflowDefinitionsTable()),
		ExpressionAttributeNames: map[string]*string{
			"#N": aws.String("name"),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":name": &dynamodb.AttributeValue{
				S: aws.String(name),
			},
		},
		KeyConditionExpression: aws.String("#N = :name"),
		ConsistentRead:         aws.Bool(true),
	})
	if err != nil {
		return []resources.WorkflowDefinition{}, err
	}
	if len(results.Items) == 0 {
		return []resources.WorkflowDefinition{}, store.NewNotFound(name)
	}

	return d.dynamoItemsToWorkflowDefinitions(results.Items)
}

// GetWorkflowDefinition gets the specific version of a workflow definition
func (d DynamoDB) GetWorkflowDefinition(name string, version int) (resources.WorkflowDefinition, error) {
	key, err := dynamodbattribute.MarshalMap(ddbWorkflowDefinitionPrimaryKey{
		Name:    name,
		Version: version,
	})
	if err != nil {
		return resources.WorkflowDefinition{}, err
	}
	res, err := d.ddb.GetItem(&dynamodb.GetItemInput{
		Key:       key,
		TableName: aws.String(d.workflowDefinitionsTable()),
	})
	if err != nil {
		return resources.WorkflowDefinition{}, err
	}

	if len(res.Item) == 0 {
		return resources.WorkflowDefinition{}, store.NewNotFound(name)
	}

	var wd resources.WorkflowDefinition
	if err := DecodeWorkflowDefinition(res.Item, &wd); err != nil {
		return resources.WorkflowDefinition{}, err
	}

	return wd, nil
}

// LatestWorkflowDefinition gets the latest version of a workflow definition.
func (d DynamoDB) LatestWorkflowDefinition(name string) (resources.WorkflowDefinition, error) {
	res, err := d.ddb.Query(&dynamodb.QueryInput{
		TableName: aws.String(d.latestWorkflowDefinitionsTable()),
		ExpressionAttributeNames: map[string]*string{
			"#N": aws.String("name"),
		},
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":name": &dynamodb.AttributeValue{
				S: aws.String(name),
			},
		},
		KeyConditionExpression: aws.String("#N = :name"),
		ConsistentRead:         aws.Bool(true),
	})
	if err != nil {
		return resources.WorkflowDefinition{}, err
	}
	if len(res.Items) != 1 {
		return resources.WorkflowDefinition{}, store.NewNotFound(name)
	}
	var wf resources.WorkflowDefinition
	if err := DecodeWorkflowDefinition(res.Items[0], &wf); err != nil {
		return resources.WorkflowDefinition{}, err
	}
	return wf, nil
}

// SaveStateResource creates or updates a StateResource in dynamo
// always overwrite old resource in store
func (d DynamoDB) SaveStateResource(stateResource resources.StateResource) error {
	data, err := EncodeStateResource(stateResource)
	if err != nil {
		return err
	}

	_, err = d.ddb.PutItem(&dynamodb.PutItemInput{
		TableName: aws.String(d.stateResourcesTable()),
		Item:      data,
	})

	return err
}

// GetStateResource gets the StateResource matching the name and namespace.
func (d DynamoDB) GetStateResource(name, namespace string) (resources.StateResource, error) {
	key, err := dynamodbattribute.MarshalMap(ddbStateResourcePrimaryKey{
		Name:      name,
		Namespace: namespace,
	})
	if err != nil {
		return resources.StateResource{}, err
	}
	res, err := d.ddb.GetItem(&dynamodb.GetItemInput{
		Key:            key,
		TableName:      aws.String(d.stateResourcesTable()),
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return resources.StateResource{}, err
	}

	if len(res.Item) == 0 {
		return resources.StateResource{}, store.NewNotFound(fmt.Sprintf("%s--%s", namespace, name))
	}

	stateResource, err := DecodeStateResource(res.Item)
	if err != nil {
		return resources.StateResource{}, err
	}

	return stateResource, nil
}

// DeleteStateResource removes an existing StateResource matching the name and namespace
func (d DynamoDB) DeleteStateResource(name, namespace string) error {
	// TODO: maybe we want to mark for deletion instead?
	key, err := dynamodbattribute.MarshalMap(ddbStateResourcePrimaryKey{
		Name:      name,
		Namespace: namespace,
	})
	if err != nil {
		return err
	}

	_, err = d.ddb.DeleteItem(&dynamodb.DeleteItemInput{
		Key:       key,
		TableName: aws.String(d.stateResourcesTable()),
		ExpressionAttributeNames: map[string]*string{
			"#N": aws.String("name"),
			"#S": aws.String("namespace"),
		},
		ConditionExpression: aws.String("attribute_exists(#N) AND attribute_exists(#S)"),
	})
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			if awsErr.Code() == dynamodb.ErrCodeConditionalCheckFailedException {
				return store.NewNotFound(
					fmt.Sprintf("name: %s, namespace: %s", name, namespace))
			}
		}
		return err
	}

	return nil
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
	wf, err := d.GetWorkflowDefinition(job.WorkflowDefinition.Name(), job.WorkflowDefinition.Version())
	if err != nil {
		return fmt.Errorf("error getting workflow for job %s: %s", job.ID, err)
	}
	job.WorkflowDefinition = wf
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

// GetJobsForWorkflowDefinition returns the last 10 jobs for a workflow.
// It uses a global secondary index on the workflow name + created time for a job.
func (d DynamoDB) GetJobsForWorkflowDefinition(workflowName string) ([]resources.Job, error) {
	var jobs []resources.Job
	query := ddbJobSecondaryKeyWorkflowDefinitionCreatedAt{
		WorkflowDefinitionName: workflowName,
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
