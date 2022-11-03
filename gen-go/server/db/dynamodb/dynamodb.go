package dynamodb

import (
	"context"
	"errors"
	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/Clever/workflow-manager/gen-go/server/db"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	"github.com/go-openapi/strfmt"
	"time"
)

// Config is used to create a new DB struct.
type Config struct {
	// DynamoDBAPI is used to communicate with DynamoDB. It is required.
	// It can be overriden on a table-by-table basis.
	DynamoDBAPI dynamodbiface.DynamoDBAPI

	// DefaultPrefix configures a prefix on all table names. It is required.
	// It can be overriden on a table-by-table basis.
	DefaultPrefix string

	// DefaultWriteCapacityUnits configures a default write capacity when creating tables. It defaults to 1.
	// It can be overriden on a table-by-table basis.
	DefaultWriteCapacityUnits int64

	// DefaultReadCapacityUnits configures a default read capacity when creating tables. It defaults to 1.
	// It can be overriden on a table-by-table basis.
	DefaultReadCapacityUnits int64
	// WorkflowDefinitionTable configuration.
	WorkflowDefinitionTable WorkflowDefinitionTable
}

// maxDynamoDBBatchItems is the AWS-defined maximum number of items that can be written at once
// https://docs.aws.amazon.com/amazondynamodb/latest/APIReference/API_BatchWriteItem.html
const maxDynamoDBBatchItems = 25

// New creates a new DB object.
func New(config Config) (*DB, error) {
	if config.DynamoDBAPI == nil {
		return nil, errors.New("must specify DynamoDBAPI")
	}
	if config.DefaultPrefix == "" {
		return nil, errors.New("must specify DefaultPrefix")
	}

	if config.DefaultWriteCapacityUnits == 0 {
		config.DefaultWriteCapacityUnits = 1
	}
	if config.DefaultReadCapacityUnits == 0 {
		config.DefaultReadCapacityUnits = 1
	}
	// configure WorkflowDefinition table
	workflowDefinitionTable := config.WorkflowDefinitionTable
	if workflowDefinitionTable.DynamoDBAPI == nil {
		workflowDefinitionTable.DynamoDBAPI = config.DynamoDBAPI
	}
	if workflowDefinitionTable.Prefix == "" {
		workflowDefinitionTable.Prefix = config.DefaultPrefix
	}
	if workflowDefinitionTable.ReadCapacityUnits == 0 {
		workflowDefinitionTable.ReadCapacityUnits = config.DefaultReadCapacityUnits
	}
	if workflowDefinitionTable.WriteCapacityUnits == 0 {
		workflowDefinitionTable.WriteCapacityUnits = config.DefaultWriteCapacityUnits
	}

	return &DB{
		workflowDefinitionTable: workflowDefinitionTable,
	}, nil
}

// DB implements the database interface using DynamoDB to store data.
type DB struct {
	workflowDefinitionTable WorkflowDefinitionTable
}

var _ db.Interface = DB{}

// CreateTables creates all tables.
func (d DB) CreateTables(ctx context.Context) error {
	if err := d.workflowDefinitionTable.create(ctx); err != nil {
		return err
	}
	return nil
}

// SaveWorkflowDefinition saves a WorkflowDefinition to the database.
func (d DB) SaveWorkflowDefinition(ctx context.Context, m models.WorkflowDefinition) error {
	return d.workflowDefinitionTable.saveWorkflowDefinition(ctx, m)
}

// GetWorkflowDefinition retrieves a WorkflowDefinition from the database.
func (d DB) GetWorkflowDefinition(ctx context.Context, name string, version int64) (*models.WorkflowDefinition, error) {
	return d.workflowDefinitionTable.getWorkflowDefinition(ctx, name, version)
}

// GetWorkflowDefinitionsByNameAndVersion retrieves a page of WorkflowDefinitions from the database.
func (d DB) GetWorkflowDefinitionsByNameAndVersion(ctx context.Context, input db.GetWorkflowDefinitionsByNameAndVersionInput, fn func(m *models.WorkflowDefinition, lastWorkflowDefinition bool) bool) error {
	return d.workflowDefinitionTable.getWorkflowDefinitionsByNameAndVersion(ctx, input, fn)
}

// DeleteWorkflowDefinition deletes a WorkflowDefinition from the database.
func (d DB) DeleteWorkflowDefinition(ctx context.Context, name string, version int64) error {
	return d.workflowDefinitionTable.deleteWorkflowDefinition(ctx, name, version)
}

func toDynamoTimeString(d strfmt.DateTime) string {
	return time.Time(d).Format(time.RFC3339) // dynamodb attributevalue only supports RFC3339 resolution
}

func toDynamoTimeStringPtr(d *strfmt.DateTime) string {
	return time.Time(*d).Format(time.RFC3339) // dynamodb attributevalue only supports RFC3339 resolution
}
