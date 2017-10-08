package executor

import (
	"testing"

	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/stretchr/testify/assert"
	"gopkg.in/Clever/kayvee-go.v6/logger"
)

func init() {
	err := logger.SetGlobalRouting("../kvconfig.yml")
	if err != nil {
		panic(err)
	}
}

func TestDataResultsRouting(t *testing.T) {
	assert := assert.New(t)
	t.Log("setup mock logger, to capture log routing matches")
	mocklog := logger.NewMockCountLogger("workflow-manager")
	// Overrides package level logger
	log = mocklog
	counts := mocklog.RuleCounts()
	assert.Equal(0, len(counts))

	t.Log("matches 'job-status-alerts'")
	logJobStatus(&models.Job{}, &models.Workflow{WorkflowDefinition: &models.WorkflowDefinition{}})
	counts = mocklog.RuleCounts()
	assert.Equal(1, len(counts))
	assert.Contains(counts, "job-status-alerts")
	assert.Equal(counts["job-status-alerts"], 1)

	t.Log("matches 'workflow-status-change'")
	logWorkflowStatusChange(&models.Workflow{WorkflowDefinition: &models.WorkflowDefinition{}}, models.WorkflowStatusRunning)
	counts = mocklog.RuleCounts()
	assert.Equal(3, len(counts))
	assert.Contains(counts, "workflow-status-metrics")
	assert.Equal(counts["workflow-status-metrics"], 1)
	assert.Contains(counts, "workflow-status-alerts")
	assert.Equal(counts["workflow-status-alerts"], 1)
}
