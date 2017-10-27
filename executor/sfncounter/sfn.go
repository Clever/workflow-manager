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
	describeExecution   *CumulativeBucket
}

// New creates a new SFNCounter
func New(sfnapi sfniface.SFNAPI) *SFNCounter {
	return &SFNCounter{
		SFNAPI: sfnapi,
		getExecutionHistory: &CumulativeBucket{
			MetricName: "get-execution-history",
			Dimensions: map[string]string{},
		},
		describeExecution: &CumulativeBucket{
			MetricName: "describe-execution",
			Dimensions: map[string]string{},
		},
	}
}

// countGetExecutionHistoryRequest counts GetExecutionHistory requests.
// These requests are limited via a token bucket of size 250 with refill rate of 5/sec.
// See http://docs.aws.amazon.com/step-functions/latest/dg/limits.html.
func (s *SFNCounter) countGetExecutionHistoryRequest(*request.Request) {
	s.getExecutionHistory.Add(1)
}

// GetExecutionHistoryPagesWithContext counts GetExecutionHistory requests by injecting a request option that increments the counter.
// This captures all underlying GetExectuionHistory HTTP requests.
func (s *SFNCounter) GetExecutionHistoryPagesWithContext(ctx aws.Context, i *sfn.GetExecutionHistoryInput, fn func(*sfn.GetExecutionHistoryOutput, bool) bool, opts ...request.Option) error {
	opts = append(opts, s.countGetExecutionHistoryRequest)
	return s.SFNAPI.GetExecutionHistoryPagesWithContext(ctx, i, fn, opts...)
}

// getExecutionHistoryCounter returns the cumulative counter of requests to GetExecutionHistory.
func (s *SFNCounter) getExecutionHistoryCounter() datapoint.IntValue {
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

// countDescribeExecutionRequest counts DescribeExecution requests.
// These requests are limited via a token bucket of size 200 with refill rate of 2/sec.
// See http://docs.aws.amazon.com/step-functions/latest/dg/limits.html.
func (s *SFNCounter) countDescribeExecutionRequest(*request.Request) {
	s.describeExecution.Add(1)
}

// DescribeExecutionWithContext counts DescribeExecution requests by injecting a request option that increments the counter.
func (s *SFNCounter) DescribeExecutionWithContext(ctx aws.Context, i *sfn.DescribeExecutionInput, opts ...request.Option) (*sfn.DescribeExecutionOutput, error) {
	opts = append(opts, s.countDescribeExecutionRequest)
	return s.SFNAPI.DescribeExecutionWithContext(ctx, i, opts...)
}

// describeExecutionCounter returns the cumulative counter of requests to describeExecution.
func (s *SFNCounter) describeExecutionCounter() datapoint.IntValue {
	dps := s.describeExecution.Datapoints()
	for _, dp := range dps {
		if iv, ok := dp.Value.(datapoint.IntValue); ok {
			return iv
		}
	}
	return nil
}

type Counters struct {
	GetExecutionHistory int64
	DescribeExecution   int64
}

// Counters returns all counter values
func (s *SFNCounter) Counters() Counters {
	var counters Counters
	if val := s.getExecutionHistoryCounter(); val != nil {
		counters.GetExecutionHistory = val.Int()
	}
	if val := s.describeExecutionCounter(); val != nil {
		counters.DescribeExecution = val.Int()
	}
	return counters
}
