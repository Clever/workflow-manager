package executor

import (
	"testing"

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

}
