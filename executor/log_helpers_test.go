package executor

import (
	"testing"

	counter "github.com/Clever/aws-sdk-go-counter"
	"github.com/Clever/kayvee-go/v7/logger"
	"github.com/stretchr/testify/assert"
)

func init() {
	err := logger.SetGlobalRouting("../kvconfig.yml")
	if err != nil {
		panic(err)
	}
}

func TestRoutingRules(t *testing.T) {
	t.Run("aws-sdk-go-counter", func(t *testing.T) {
		mocklog := logger.NewMockCountLogger("workflow-manager")
		log = mocklog
		LogSFNCounts([]counter.ServiceCount{
			{
				Service:   "sfn",
				Operation: "GetExecutionHistory",
				Count:     100,
			},
			{
				Service:   "sfn",
				Operation: "DescribeExecution",
				Count:     200,
			},
		})
		counts := mocklog.RuleCounts()
		assert.Equal(t, 1, len(counts))
		assert.Contains(t, counts, "aws-sdk-go-counter")
		assert.Equal(t, 2, counts["aws-sdk-go-counter"])
	})
}
