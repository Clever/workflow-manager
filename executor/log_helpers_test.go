package executor

import (
	"testing"
	"time"

	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/go-openapi/strfmt"
	"github.com/stretchr/testify/assert"
	"gopkg.in/Clever/kayvee-go.v6/logger"
)

func init() {
	err := logger.SetGlobalRouting("../kvconfig.yml")
	if err != nil {
		panic(err)
	}
}

func TestRoutingRules(t *testing.T) {
	t.Run("update-loop-lag-alert", func(t *testing.T) {
		mocklog := logger.NewMockCountLogger("workflow-manager")
		// Overrides package level logger
		log = mocklog
		logPendingWorkflowsLocked(models.Workflow{
			WorkflowSummary: models.WorkflowSummary{
				ID:                 "id",
				LastUpdated:        strfmt.DateTime(time.Now()),
				WorkflowDefinition: &models.WorkflowDefinition{},
			},
		})
		counts := mocklog.RuleCounts()
		assert.Equal(t, 1, len(counts))
		assert.Contains(t, counts, "update-loop-lag-alert")
		assert.Equal(t, counts["update-loop-lag-alert"], 1)
	})
}
