package sfncache

import (
	"github.com/aws/aws-sdk-go/service/sfn"
	"github.com/aws/aws-sdk-go/service/sfn/sfniface"
	lru "github.com/hashicorp/golang-lru"
)

type SFNCache struct {
	sfniface.SFNAPI
	describeStateMachineCache *lru.Cache
}

// New creates a new cached version of SFNAPI.
func New(sfnapi sfniface.SFNAPI) (sfniface.SFNAPI, error) {
	describeStateMachineCache, err := lru.New(1000)
	if err != nil {
		return nil, err
	}
	return &SFNCache{
		SFNAPI: sfnapi,
		describeStateMachineCache: describeStateMachineCache,
	}, nil
}

// DescribeStateMachine is cached aggressively since state machines are immutable.
func (s *SFNCache) DescribeStateMachine(i *sfn.DescribeStateMachineInput) (*sfn.DescribeStateMachineOutput, error) {
	cacheKey := i.String()
	cacheVal, ok := s.describeStateMachineCache.Get(cacheKey)
	if ok {
		return cacheVal.(*sfn.DescribeStateMachineOutput), nil
	}
	out, err := s.SFNAPI.DescribeStateMachine(i)
	if err != nil {
		return out, err
	}
	s.describeStateMachineCache.Add(cacheKey, out)
	return out, nil
}
