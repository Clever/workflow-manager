package dynamodb

import (
	"fmt"
	"sort"
	"time"

	"github.com/Clever/workflow-manager/resources"
	"github.com/Clever/workflow-manager/store"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	"github.com/zencoder/ddbsync"
)

type DynamoDB struct {
	ddb         dynamodbiface.DynamoDBAPI
	tableConfig TableConfig
	lockDB      ddbsync.DBer
}

type TableConfig struct {
	PrefixStateResources      string
	PrefixWorkflowDefinitions string
	PrefixWorkflows           string
}

func New(ddb dynamodbiface.DynamoDBAPI, tableConfig TableConfig) DynamoDB {
	d := DynamoDB{
		ddb:         ddb,
		tableConfig: tableConfig,
	}
	d.lockDB = ddbsync.NewDatabaseFromDDBAPI(ddb, d.locksTable())
	return d
}

// locksTable returns the name of the table that stores the locks on workflows.
func (d DynamoDB) locksTable() string {
	return fmt.Sprintf("%s-locks", d.tableConfig.PrefixWorkflows)
}

// latestWorkflowDefinitionsTable returns the name of the table that stores the latest version of every WorkflowDefinition
func (d DynamoDB) latestWorkflowDefinitionsTable() string {
	return fmt.Sprintf("%s-latest-workflow-definitions", d.tableConfig.PrefixWorkflowDefinitions)
}

// workflowDefinitionsTable returns the name of the table that stores all WorkflowDefinitions
func (d DynamoDB) workflowDefinitionsTable() string {
	return fmt.Sprintf("%s-workflow-definitions", d.tableConfig.PrefixWorkflowDefinitions)
}

// workflowsTable returns the name of the table that stores workflows.
func (d DynamoDB) workflowsTable() string {
	return fmt.Sprintf("%s-workflows", d.tableConfig.PrefixWorkflows)
}

// stateResourcesTable returns the name of the table that stores stateResources.
func (d DynamoDB) stateResourcesTable() string {
	return fmt.Sprintf("%s-state-resources", d.tableConfig.PrefixStateResources)
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

	// create workflows table from workflow ID -> workflow object
	workflowAttributeDefinitions := []*dynamodb.AttributeDefinition{}
	for _, ads := range [][]*dynamodb.AttributeDefinition{
		(ddbWorkflowPrimaryKey{}.AttributeDefinitions()),
		(ddbWorkflowSecondaryKeyWorkflowDefinitionCreatedAt{}.AttributeDefinitions()),
		(ddbWorkflowSecondaryKeyStatusLastUpdated{}.AttributeDefinitions()),
	} {
		workflowAttributeDefinitions = append(workflowAttributeDefinitions, ads...)
	}
	if _, err := d.ddb.CreateTable(&dynamodb.CreateTableInput{
		AttributeDefinitions: workflowAttributeDefinitions,
		KeySchema:            ddbWorkflowPrimaryKey{}.KeySchema(),
		GlobalSecondaryIndexes: []*dynamodb.GlobalSecondaryIndex{
			{
				IndexName: aws.String(ddbWorkflowSecondaryKeyWorkflowDefinitionCreatedAt{}.Name()),
				KeySchema: ddbWorkflowSecondaryKeyWorkflowDefinitionCreatedAt{}.KeySchema(),
				Projection: &dynamodb.Projection{
					ProjectionType: aws.String(dynamodb.ProjectionTypeAll),
				},
				ProvisionedThroughput: &dynamodb.ProvisionedThroughput{
					ReadCapacityUnits:  aws.Int64(1),
					WriteCapacityUnits: aws.Int64(1),
				},
			},
			{
				IndexName: aws.String(ddbWorkflowSecondaryKeyStatusLastUpdated{}.Name()),
				KeySchema: ddbWorkflowSecondaryKeyStatusLastUpdated{}.KeySchema(),
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
		TableName: aws.String(d.workflowsTable()),
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

	// create locks table. This should probably be exposed in ddbsync
	if _, err := d.ddb.CreateTable(&dynamodb.CreateTableInput{
		AttributeDefinitions: []*dynamodb.AttributeDefinition{
			{
				AttributeName: aws.String("Name"),
				AttributeType: aws.String(dynamodb.ScalarAttributeTypeS),
			},
		},
		KeySchema: []*dynamodb.KeySchemaElement{
			{
				AttributeName: aws.String("Name"),
				KeyType:       aws.String(dynamodb.KeyTypeHash),
			},
		},
		ProvisionedThroughput: &dynamodb.ProvisionedThroughput{
			ReadCapacityUnits:  aws.Int64(1),
			WriteCapacityUnits: aws.Int64(1),
		},
		TableName: aws.String(d.locksTable()),
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

// SaveWorkflow saves a workflow to dynamo.
func (d DynamoDB) SaveWorkflow(workflow resources.Workflow) error {
	workflow.CreatedAt = time.Now()
	workflow.LastUpdated = workflow.CreatedAt

	data, err := EncodeWorkflow(workflow)
	if err != nil {
		return err
	}
	_, err = d.ddb.PutItem(&dynamodb.PutItemInput{
		TableName: aws.String(d.workflowsTable()),
		Item:      data,
		ExpressionAttributeNames: map[string]*string{
			"#I": aws.String("id"),
		},
		ConditionExpression: aws.String("attribute_not_exists(#I)"),
	})
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			if awsErr.Code() == dynamodb.ErrCodeConditionalCheckFailedException {
				return store.NewConflict(workflow.ID)
			}
		}
	}
	return err
}

func (d DynamoDB) UpdateWorkflow(workflow resources.Workflow) error {
	workflow.LastUpdated = time.Now()

	data, err := EncodeWorkflow(workflow)
	if err != nil {
		return err
	}
	_, err = d.ddb.PutItem(&dynamodb.PutItemInput{
		TableName: aws.String(d.workflowsTable()),
		Item:      data,
		ExpressionAttributeNames: map[string]*string{
			"#I": aws.String("id"),
		},
		ConditionExpression: aws.String("attribute_exists(#I)"),
	})
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			if awsErr.Code() == dynamodb.ErrCodeConditionalCheckFailedException {
				return store.NewNotFound(workflow.ID)
			}
		}
	}
	return err
}

// populateWorkflow fills in the workflow object contained in a workflow
func (d DynamoDB) populateWorkflow(workflow *resources.Workflow) error {
	wf, err := d.GetWorkflowDefinition(workflow.WorkflowDefinition.Name(), workflow.WorkflowDefinition.Version())
	if err != nil {
		return fmt.Errorf("error getting WorkflowDefinition for Workflow %s: %s", workflow.ID, err)
	}
	workflow.WorkflowDefinition = wf
	return nil
}

// GetWorkflowByID
func (d DynamoDB) GetWorkflowByID(id string) (resources.Workflow, error) {
	key, err := dynamodbattribute.MarshalMap(ddbWorkflowPrimaryKey{
		ID: id,
	})
	if err != nil {
		return resources.Workflow{}, err
	}
	res, err := d.ddb.GetItem(&dynamodb.GetItemInput{
		Key:            key,
		TableName:      aws.String(d.workflowsTable()),
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return resources.Workflow{}, err
	}

	if len(res.Item) == 0 {
		return resources.Workflow{}, store.NewNotFound(id)
	}

	workflow, err := DecodeWorkflow(res.Item)
	if err != nil {
		return resources.Workflow{}, err
	}

	if err := d.populateWorkflow(&workflow); err != nil {
		return resources.Workflow{}, err
	}

	return workflow, nil
}

// GetWorkflows returns the last 10 workflows for a workflow.
// It uses a global secondary index on the workflow name + created time for a workflow.
func (d DynamoDB) GetWorkflows(workflowName string) ([]resources.Workflow, error) {
	var workflows []resources.Workflow
	query, err := ddbWorkflowSecondaryKeyWorkflowDefinitionCreatedAt{
		WorkflowDefinitionName: workflowName,
	}.ConstructQuery()
	if err != nil {
		return workflows, err
	}

	query.TableName = aws.String(d.workflowsTable())
	query.Limit = aws.Int64(10)
	query.ScanIndexForward = aws.Bool(false) // descending order

	res, err := d.ddb.Query(query)
	if err != nil {
		return workflows, err
	}

	for _, item := range res.Items {
		workflow, err := DecodeWorkflow(item)
		if err != nil {
			return workflows, err
		}
		if err := d.populateWorkflow(&workflow); err != nil {
			return workflows, err
		}
		workflows = append(workflows, workflow)
	}
	return workflows, nil
}

type byLastUpdatedTime []resources.Workflow

func (b byLastUpdatedTime) Len() int           { return len(b) }
func (b byLastUpdatedTime) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }
func (b byLastUpdatedTime) Less(i, j int) bool { return b[i].LastUpdated.Before(b[j].LastUpdated) }

// GetPendingWorkflowIDs gets workflows that are either Queued or Running.
// It uses a global secondary index on status and last updated time in order to return
// workflows ordered by their last updated time. Workflows with the oldest last updated
// time are returned first.
func (d DynamoDB) GetPendingWorkflowIDs() ([]string, error) {
	var pendingWorkflows []resources.Workflow
	for _, statusToQuery := range []resources.WorkflowStatus{resources.Queued, resources.Running} {
		query, err := ddbWorkflowSecondaryKeyStatusLastUpdated{
			Status: statusToQuery,
		}.ConstructQuery()
		if err != nil {
			return nil, err
		}
		query.TableName = aws.String(d.workflowsTable())
		query.Limit = aws.Int64(5)
		res, err := d.ddb.Query(query)
		if err != nil {
			return nil, err
		}

		for _, item := range res.Items {
			workflow, err := DecodeWorkflow(item)
			if err != nil {
				return nil, err
			}
			pendingWorkflows = append(pendingWorkflows, workflow)
		}
	}

	sort.Sort(byLastUpdatedTime(pendingWorkflows))

	pendingWorkflowIDs := []string{}
	for _, pendingWorkflow := range pendingWorkflows {
		pendingWorkflowIDs = append(pendingWorkflowIDs, pendingWorkflow.ID)
	}
	return pendingWorkflowIDs, nil
}

// LockWorkflow acquires a lock on modifying a workflow.
func (s DynamoDB) LockWorkflow(id string) error {
	mu := ddbsync.NewMutex(id, 30 /* seconds */, s.lockDB, 0 /* no reattempts, so irrelevant */)
	if err := mu.AttemptLock(); err != nil {
		if err == ddbsync.ErrLockAlreadyHeld {
			return store.ErrWorkflowLocked
		}
		return err
	}
	return nil
}

// UnlockWorkflow releases a lock (if it exists) on modifying a workflow.
func (s DynamoDB) UnlockWorkflow(id string) error {
	mu := ddbsync.NewMutex(id, 30 /* seconds */, s.lockDB, 0 /* no reattempts, so irrelevant */)
	mu.Unlock()
	return nil
}
