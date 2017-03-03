package batchclient

import (
	"github.com/aws/aws-sdk-go/service/batch"
)

type BatchExecutor struct {
	client batch.batchiface
	queue  string
}

func NewBatchExecutor(client batch.batchiface, queue string) {
	return BatchExecutor{
		client,
		queue,
	}
}

func (be BatchExecutor) SubmitJob(name string, definition string, dependencies []string) {

}
