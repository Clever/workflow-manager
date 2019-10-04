package sfncache

import (
	"context"

	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/sfn"
	"github.com/aws/aws-sdk-go/service/sfn/sfniface"
	lru "github.com/hashicorp/golang-lru"
)

// SFNCache caches SFN API calls.
type SFNCache struct {
	sfniface.SFNAPI
	describeStateMachineWithContextCache *lru.Cache
}

// New creates a new cached version of SFNAPI.
func New(sfnapi sfniface.SFNAPI) (*SFNCache, error) {
	describeStateMachineCacheWithContext, err := lru.New(1000)
	if err != nil {
		return nil, err
	}
	return &SFNCache{
		SFNAPI:                               sfnapi,
		describeStateMachineWithContextCache: describeStateMachineCacheWithContext,
	}, nil
}

// DescribeStateMachineWithContext is cached aggressively since most state machines are immutable
func (s *SFNCache) DescribeStateMachineWithContext(ctx context.Context, i *sfn.DescribeStateMachineInput, options ...request.Option) (*sfn.DescribeStateMachineOutput, error) {
	cacheKey := i.String()
	cacheVal, ok := s.describeStateMachineWithContextCache.Get(cacheKey)
	if ok {
		return cacheVal.(*sfn.DescribeStateMachineOutput), nil
	}
	out, err := s.SFNAPI.DescribeStateMachineWithContext(ctx, i)
	if err != nil {
		return out, err
	}
	s.describeStateMachineWithContextCache.Add(cacheKey, out)
	return out, nil
}
