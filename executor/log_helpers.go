package executor

import (
	counter "github.com/Clever/aws-sdk-go-counter"
	"github.com/Clever/kayvee-go/v7/logger"
)

var log = logger.New("workflow-manager")

func LogSFNCounts(sfnCounters []counter.ServiceCount) {
	for _, v := range sfnCounters {
		log.TraceD("aws-sdk-go-counter", logger.M{
			"app": "workflow-manager", "value": v.Count, "aws-operation": v.Operation, "aws-service": v.Service,
		})
	}
}
