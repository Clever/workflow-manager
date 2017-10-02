package resources

import (
	"fmt"
	"testing"
	"time"

	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/go-openapi/swag"
	"github.com/stretchr/testify/assert"
)

// KitchenSinkWorkflowDefinition returns a WorkflowDefinition resource to use for tests
func KitchenSinkWorkflowDefinition(t *testing.T) *models.WorkflowDefinition {

	wfd, err := NewWorkflowDefinition(fmt.Sprintf("kitchensink-%s", time.Now().Format(time.RFC3339Nano)),
		&models.SLStateMachine{
			Comment: "description",
			StartAt: "start-state",
			States: map[string]models.SLState{
				"start-state": models.SLState{
					Next:     "second-state",
					Resource: "fake-resource-1",
					End:      false,
					Retry: []*models.SLRetrier{{
						ErrorEquals: []models.SLErrorEquals{"States.ALL"},
						MaxAttempts: swag.Int64(2),
					}},
				},
				"second-state": models.SLState{
					Next:     "end-state",
					Resource: "fake-resource-2",
					End:      false,
					Retry:    []*models.SLRetrier{},
				},
				"end-state": models.SLState{
					Resource: "fake-resource-3",
					End:      true,
					Retry:    []*models.SLRetrier{},
				},
			},
		},
	)
	assert.Nil(t, err)

	return wfd
}
