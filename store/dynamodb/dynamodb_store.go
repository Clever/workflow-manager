package dynamodb

import (
	"context"
	"fmt"
	"time"

	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/Clever/workflow-manager/resources"
	"github.com/Clever/workflow-manager/store"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	"github.com/go-openapi/strfmt"
	"github.com/zencoder/ddbsync"

	"gopkg.in/Clever/kayvee-go.v6/logger"
)

// AWS ValidationException message when item is > 400KB
const errMessageItemTooLarge = "Item size has exceeded the maximum allowed size"

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

var log = logger.New("workflow-manager")

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
func (d DynamoDB) dynamoItemsToWorkflowDefinitions(items []map[string]*dynamodb.AttributeValue) ([]models.WorkflowDefinition, error) {
	workflowDefinitions := []models.WorkflowDefinition{}

	for _, item := range items {
		var wf models.WorkflowDefinition
		if err := DecodeWorkflowDefinition(item, &wf); err != nil {
			return []models.WorkflowDefinition{}, err
		}
		workflowDefinitions = append(workflowDefinitions, wf)
	}

	return workflowDefinitions, nil
}

// InitTables creates the dynamo tables.
func (d DynamoDB) InitTables(ctx context.Context, setupWorkflowsTTL bool) error {
	// create workflowDefinitions table from name, version -> workflow object
	if _, err := d.ddb.CreateTableWithContext(ctx, &dynamodb.CreateTableInput{
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
	if _, err := d.ddb.CreateTableWithContext(ctx, &dynamodb.CreateTableInput{
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
		(ddbWorkflowSecondaryKeyDefinitionResolvedByUserCreatedAt{}.AttributeDefinitions()),
		(ddbWorkflowSecondaryKeyDefinitionStatusCreatedAt{}.AttributeDefinitions()),
	} {
		workflowAttributeDefinitions = append(workflowAttributeDefinitions, ads...)
	}
	if _, err := d.ddb.CreateTableWithContext(ctx, &dynamodb.CreateTableInput{
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
				IndexName: aws.String(ddbWorkflowSecondaryKeyDefinitionResolvedByUserCreatedAt{}.Name()),
				KeySchema: ddbWorkflowSecondaryKeyDefinitionResolvedByUserCreatedAt{}.KeySchema(),
				Projection: &dynamodb.Projection{
					ProjectionType: aws.String(dynamodb.ProjectionTypeAll),
				},
				ProvisionedThroughput: &dynamodb.ProvisionedThroughput{
					ReadCapacityUnits:  aws.Int64(1),
					WriteCapacityUnits: aws.Int64(1),
				},
			},
			{
				IndexName: aws.String(ddbWorkflowSecondaryKeyDefinitionStatusCreatedAt{}.Name()),
				KeySchema: ddbWorkflowSecondaryKeyDefinitionStatusCreatedAt{}.KeySchema(),
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
	if setupWorkflowsTTL {
		if _, err := d.ddb.UpdateTimeToLiveWithContext(ctx, &dynamodb.UpdateTimeToLiveInput{
			TableName: aws.String(d.workflowsTable()),
			TimeToLiveSpecification: &dynamodb.TimeToLiveSpecification{
				AttributeName: ddbWorkflowTTL{}.AttributeDefinition().AttributeName,
				Enabled:       aws.Bool(true),
			},
		}); err != nil {
			return err
		}
	}

	// create state-resources table from stateResource.{name, namespace} -> stateResource object
	if _, err := d.ddb.CreateTableWithContext(ctx, &dynamodb.CreateTableInput{
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
	if _, err := d.ddb.CreateTableWithContext(ctx, &dynamodb.CreateTableInput{
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
func (d DynamoDB) SaveWorkflowDefinition(ctx context.Context, def models.WorkflowDefinition) error {
	def.CreatedAt = strfmt.DateTime(time.Now())

	data, err := EncodeWorkflowDefinition(def)
	if err != nil {
		return err
	}

	_, err = d.ddb.PutItemWithContext(ctx, &dynamodb.PutItemInput{
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
				return store.NewConflict(def.Name)
			}
		}
		return err
	}
	_, err = d.ddb.PutItemWithContext(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(d.latestWorkflowDefinitionsTable()),
		Item:      data,
	})
	// TODO determine what we should do in case the 2nd write fails
	return err
}

// UpdateWorkflowDefinition updates an existing workflow definition.
// The version will be set to the version following the latest definition.
// The workflow definition returned contains this new version number.
func (d DynamoDB) UpdateWorkflowDefinition(ctx context.Context, def models.WorkflowDefinition) (models.WorkflowDefinition, error) {
	// TODO we should change this to use the latest-workflows table with a projection expressions
	latest, err := d.LatestWorkflowDefinition(ctx, def.Name) // TODO: only need version here, can optimize query
	if err != nil {
		return def, err
	}

	// TODO: this isn't thread safe...
	newVersion := resources.NewWorkflowDefinitionVersion(&def, int(latest.Version+1))
	if err := d.SaveWorkflowDefinition(ctx, *newVersion); err != nil {
		return def, err
	}

	// need to perform a get to return any mutations that happened in Save, e.g. CreatedAt
	return d.GetWorkflowDefinition(ctx, newVersion.Name, int(newVersion.Version))
}

// GetWorkflowDefinitions returns the latest version of all stored workflow definitions
func (d DynamoDB) GetWorkflowDefinitions(ctx context.Context) ([]models.WorkflowDefinition, error) {
	// Scan returns the entire table
	results, err := d.ddb.ScanWithContext(ctx, &dynamodb.ScanInput{
		ConsistentRead: aws.Bool(true),
		TableName:      aws.String(d.latestWorkflowDefinitionsTable()),
	})
	if err != nil {
		return []models.WorkflowDefinition{}, err
	}
	return d.dynamoItemsToWorkflowDefinitions(results.Items)
}

// GetWorkflowDefinitionVersions gets all versions of a workflow definition
func (d DynamoDB) GetWorkflowDefinitionVersions(ctx context.Context, name string) ([]models.WorkflowDefinition, error) {
	results, err := d.ddb.QueryWithContext(ctx, &dynamodb.QueryInput{
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
		return []models.WorkflowDefinition{}, err
	}
	if len(results.Items) == 0 {
		return []models.WorkflowDefinition{}, store.NewNotFound(name)
	}

	return d.dynamoItemsToWorkflowDefinitions(results.Items)
}

// GetWorkflowDefinition gets the specific version of a workflow definition
func (d DynamoDB) GetWorkflowDefinition(ctx context.Context, name string, version int) (models.WorkflowDefinition, error) {
	key, err := dynamodbattribute.MarshalMap(ddbWorkflowDefinitionPrimaryKey{
		Name:    name,
		Version: int64(version),
	})
	if err != nil {
		return models.WorkflowDefinition{}, err
	}
	res, err := d.ddb.GetItemWithContext(ctx, &dynamodb.GetItemInput{
		Key:       key,
		TableName: aws.String(d.workflowDefinitionsTable()),
	})
	if err != nil {
		return models.WorkflowDefinition{}, err
	}

	if len(res.Item) == 0 {
		return models.WorkflowDefinition{}, store.NewNotFound(name)
	}

	var wd models.WorkflowDefinition
	if err := DecodeWorkflowDefinition(res.Item, &wd); err != nil {
		return models.WorkflowDefinition{}, err
	}

	return wd, nil
}

// LatestWorkflowDefinition gets the latest version of a workflow definition.
func (d DynamoDB) LatestWorkflowDefinition(ctx context.Context, name string) (models.WorkflowDefinition, error) {
	res, err := d.ddb.QueryWithContext(ctx, &dynamodb.QueryInput{
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
		return models.WorkflowDefinition{}, err
	}
	if len(res.Items) != 1 {
		return models.WorkflowDefinition{}, store.NewNotFound(name)
	}
	var wf models.WorkflowDefinition
	if err := DecodeWorkflowDefinition(res.Items[0], &wf); err != nil {
		return models.WorkflowDefinition{}, err
	}
	return wf, nil
}

// SaveStateResource creates or updates a StateResource in dynamo
// always overwrite old resource in store
func (d DynamoDB) SaveStateResource(ctx context.Context, stateResource models.StateResource) error {
	data, err := EncodeStateResource(stateResource)
	if err != nil {
		return err
	}

	_, err = d.ddb.PutItemWithContext(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(d.stateResourcesTable()),
		Item:      data,
	})

	return err
}

// GetStateResource gets the StateResource matching the name and namespace.
func (d DynamoDB) GetStateResource(ctx context.Context, name, namespace string) (models.StateResource, error) {
	key, err := dynamodbattribute.MarshalMap(ddbStateResourcePrimaryKey{
		Name:      name,
		Namespace: namespace,
	})
	if err != nil {
		return models.StateResource{}, err
	}
	res, err := d.ddb.GetItemWithContext(ctx, &dynamodb.GetItemInput{
		Key:            key,
		TableName:      aws.String(d.stateResourcesTable()),
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return models.StateResource{}, err
	}

	if len(res.Item) == 0 {
		return models.StateResource{}, store.NewNotFound(fmt.Sprintf("%s--%s", namespace, name))
	}

	stateResource, err := DecodeStateResource(res.Item)
	if err != nil {
		return models.StateResource{}, err
	}

	return stateResource, nil
}

// DeleteStateResource removes an existing StateResource matching the name and namespace
func (d DynamoDB) DeleteStateResource(ctx context.Context, name, namespace string) error {
	// TODO: maybe we want to mark for deletion instead?
	key, err := dynamodbattribute.MarshalMap(ddbStateResourcePrimaryKey{
		Name:      name,
		Namespace: namespace,
	})
	if err != nil {
		return err
	}

	_, err = d.ddb.DeleteItemWithContext(ctx, &dynamodb.DeleteItemInput{
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
func (d DynamoDB) SaveWorkflow(ctx context.Context, workflow models.Workflow) error {
	workflow.CreatedAt = strfmt.DateTime(time.Now())
	workflow.LastUpdated = workflow.CreatedAt

	data, err := EncodeWorkflow(workflow)
	if err != nil {
		return err
	}
	_, err = d.ddb.PutItemWithContext(ctx, &dynamodb.PutItemInput{
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

func (d DynamoDB) UpdateWorkflow(ctx context.Context, workflow models.Workflow) error {
	workflow.LastUpdated = strfmt.DateTime(time.Now())

	data, err := EncodeWorkflow(workflow)
	if err != nil {
		return err
	}
	_, err = d.ddb.PutItemWithContext(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(d.workflowsTable()),
		Item:      data,
		ExpressionAttributeNames: map[string]*string{
			"#I": aws.String("id"),
		},
		ConditionExpression: aws.String("attribute_exists(#I)"),
	})
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			switch awsErr.Code() {
			case dynamodb.ErrCodeConditionalCheckFailedException:
				return store.NewNotFound(workflow.ID)
			case "ValidationException":
				if awsErr.Message() == errMessageItemTooLarge {
					log.WarnD("workflow-too-large", logger.M{
						"error":     awsErr.Message(),
						"id":        workflow.ID,
						"name":      workflow.WorkflowDefinition.Name,
						"namespace": workflow.Namespace,
					})
					// try again without jobs
					wfCopy := resources.CopyWorkflow(workflow)
					wfCopy.Jobs = nil
					return d.UpdateWorkflow(ctx, wfCopy)
				}
			}
		}
	}
	return err
}

// DeleteWorkflow should only be used in cases where the Workflow has failed to start
// and we need to remove it for cleanup. This removes the Workflow record from DynamoDB
func (d DynamoDB) DeleteWorkflowByID(ctx context.Context, workflowID string) error {
	_, err := d.ddb.DeleteItemWithContext(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(d.workflowsTable()),
		Key: map[string]*dynamodb.AttributeValue{
			"id": &dynamodb.AttributeValue{
				S: aws.String(workflowID),
			},
		},
		ExpressionAttributeNames: map[string]*string{
			"#I": aws.String("id"),
		},
		ConditionExpression: aws.String("attribute_exists(#I)"),
	})
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			if awsErr.Code() == dynamodb.ErrCodeConditionalCheckFailedException {
				return store.NewNotFound(workflowID)
			}
		}
	}
	return err
}

// GetWorkflowByID
func (d DynamoDB) GetWorkflowByID(ctx context.Context, id string) (models.Workflow, error) {
	key, err := dynamodbattribute.MarshalMap(ddbWorkflowPrimaryKey{
		ID: id,
	})
	if err != nil {
		return models.Workflow{}, err
	}
	res, err := d.ddb.GetItemWithContext(ctx, &dynamodb.GetItemInput{
		Key:            key,
		TableName:      aws.String(d.workflowsTable()),
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return models.Workflow{}, err
	}

	if len(res.Item) == 0 {
		return models.Workflow{}, store.NewNotFound(id)
	}

	workflow, err := DecodeWorkflow(res.Item)
	if err != nil {
		return models.Workflow{}, err
	}

	return workflow, nil
}

// GetWorkflows returns all workflows matching the given query.
func (d DynamoDB) GetWorkflows(ctx context.Context, query *models.WorkflowQuery) ([]models.Workflow, string, error) {
	var workflows []models.Workflow
	nextPageToken := ""
	summaryOnly := aws.BoolValue(query.SummaryOnly)

	var dbQuery *dynamodb.QueryInput
	var err error
	statusIsSet := query.Status != ""
	resolvedByUserIsSet := query.ResolvedByUserWrapper != nil && query.ResolvedByUserWrapper.IsSet
	// status should never be nonempty when ResolvedByUser.IsSet is true, based on handler.
	if statusIsSet && resolvedByUserIsSet {
		return workflows, nextPageToken, store.NewInvalidQueryStructureError("query cannot contain Status when ResolvedByUser value is set.")
	}

	if statusIsSet {
		// if query includes status, query by status
		dbQuery, err = ddbWorkflowSecondaryKeyDefinitionStatusCreatedAt{}.ConstructQuery(query)
	} else if resolvedByUserIsSet {
		// otherwise, if query includes a ResolvedByUser value that is set, query with the ResolvedByUser value
		dbQuery, err = ddbWorkflowSecondaryKeyDefinitionResolvedByUserCreatedAt{}.ConstructQuery(query)
	} else {
		// otherwise, query on just the workflow definition name
		dbQuery, err = ddbWorkflowSecondaryKeyWorkflowDefinitionCreatedAt{
			WorkflowDefinitionName: aws.StringValue(query.WorkflowDefinitionName),
		}.ConstructQuery(summaryOnly)
	}
	if err != nil {
		return workflows, nextPageToken, err
	}

	dbQuery.TableName = aws.String(d.workflowsTable())
	dbQuery.Limit = aws.Int64(query.Limit)
	dbQuery.ScanIndexForward = aws.Bool(query.OldestFirst)

	pageKey, err := ParsePageKey(query.PageToken)
	if err != nil {
		return workflows, nextPageToken, store.NewInvalidPageTokenError(err)
	}
	if pageKey != nil {
		dbQuery.SetExclusiveStartKey(map[string]*dynamodb.AttributeValue(*pageKey))
	}

	res, err := d.ddb.QueryWithContext(ctx, dbQuery)
	if err != nil {
		return workflows, nextPageToken, err
	}

	for _, item := range res.Items {
		workflow, err := DecodeWorkflow(item)
		if err != nil {
			return workflows, nextPageToken, err
		}

		workflows = append(workflows, workflow)
	}

	nextPageKey := NewPageKey(res.LastEvaluatedKey)
	if nextPageKey != nil {
		nextPageToken, err = nextPageKey.ToJSON()
		if err != nil {
			return workflows, nextPageToken, err
		}
	}

	return workflows, nextPageToken, nil
}

type byLastUpdatedTime []models.Workflow

func (b byLastUpdatedTime) Len() int      { return len(b) }
func (b byLastUpdatedTime) Swap(i, j int) { b[i], b[j] = b[j], b[i] }
func (b byLastUpdatedTime) Less(i, j int) bool {
	return time.Time(b[i].LastUpdated).Before(time.Time(b[j].LastUpdated))
}
