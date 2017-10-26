package sfncounter

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/sfn"
	"github.com/aws/aws-sdk-go/service/sfn/sfniface"
	"github.com/signalfx/golib/datapoint"
)

// SFNCounter periodically logs a cumulative counter for API calls made to SFN
type SFNCounter struct {
	sfniface.SFNAPI
	getExecutionHistory *CumulativeBucket
}

// New creates a new SFNCounter
func New(sfnapi sfniface.SFNAPI) *SFNCounter {
	return &SFNCounter{
		SFNAPI: sfnapi,
		getExecutionHistory: &CumulativeBucket{
			MetricName: "get-execution-history",
			Dimensions: map[string]string{},
		},
	}
}

// countGetExecutionHistory counts GetExectuionHistory requests.
// These requests are limited via a token bucket of size 250 with refill rate of 5/sec.
// See http://docs.aws.amazon.com/step-functions/latest/dg/limits.html.
func (s *SFNCounter) countGetExecutionHistoryRequest(*request.Request) {
	s.getExecutionHistory.Add(1)
}

// GetExecutionHistoryPagesWithContext is counted by injecting a request option that increments the count.
// This captures all underlying GetExectuionHistory HTTP requests.
func (s *SFNCounter) GetExecutionHistoryPagesWithContext(ctx aws.Context, i *sfn.GetExecutionHistoryInput, fn func(*sfn.GetExecutionHistoryOutput, bool) bool, opts ...request.Option) error {
	opts = append(opts, s.countGetExecutionHistoryRequest)
	return s.SFNAPI.GetExecutionHistoryPagesWithContext(ctx, i, fn, opts...)
}

// GetExecutionHistoryCount returns the cumulative count of requests to GetExecutionHistory.
func (s *SFNCounter) GetExecutionHistoryCount() datapoint.IntValue {
	// cumulative bucket tracks count, sum, and sum squares.
	// these are int, int, and float values, respectively
	// since we're always adding 1 (`Add(1)` above) these datapoints all be the same.
	// so just pick the first int we see
	dps := s.getExecutionHistory.Datapoints()
	for _, dp := range dps {
		if iv, ok := dp.Value.(datapoint.IntValue); ok {
			return iv
		}
	}
	return nil
}
