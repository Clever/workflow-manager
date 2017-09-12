package executor

import (
	"testing"

	"github.com/Clever/kayvee-go/logger"
	"github.com/Clever/workflow-manager/resources"
	"github.com/stretchr/testify/assert"
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
	logJobStatus(&resources.Job{}, &resources.Workflow{})
	counts = mocklog.RuleCounts()
	assert.Equal(1, len(counts))
	assert.Contains(counts, "job-status-alerts")
	assert.Equal(counts["job-status-alerts"], 1)

	t.Log("matches 'workflow-status-change'")
	logWorkflowStatusChange(&resources.Workflow{}, resources.Running)
	counts = mocklog.RuleCounts()
	assert.Equal(3, len(counts))
	assert.Contains(counts, "workflow-status-metrics")
	assert.Equal(counts["workflow-status-metrics"], 1)
	assert.Contains(counts, "workflow-status-alerts")
	assert.Equal(counts["workflow-status-alerts"], 1)
}
