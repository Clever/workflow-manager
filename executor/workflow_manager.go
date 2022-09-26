package executor

import (
	"context"

	"github.com/Clever/workflow-manager/gen-go/models"
)

// WorkflowManager is the interface for creating, stopping and checking status for Workflows
type WorkflowManager interface {
	CreateWorkflow(ctx context.Context, def models.WorkflowDefinition, input string, namespace string, queue string, tags map[string]interface{}) (*models.Workflow, error)
	RetryWorkflow(ctx context.Context, workflow models.Workflow, startAt, input string) (*models.Workflow, error)
	CancelWorkflow(ctx context.Context, workflow *models.Workflow, reason string) error
	UpdateWorkflowSummary(ctx context.Context, workflow *models.Workflow) error
	UpdateWorkflowHistory(ctx context.Context, workflow *models.Workflow) error
}
