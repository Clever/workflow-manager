package sfncache

import (
	"context"
	"regexp"

	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/sfn"
	"github.com/aws/aws-sdk-go/service/sfn/sfniface"
	lru "github.com/hashicorp/golang-lru"
)

type SFNCache struct {
	sfniface.SFNAPI
	describeStateMachineWithContextCache *lru.Cache
}

// New creates a new cached version of SFNAPI.
func New(sfnapi sfniface.SFNAPI) (sfniface.SFNAPI, error) {
	describeStateMachineCacheWithContext, err := lru.New(1000)
	if err != nil {
		return nil, err
	}
	return &SFNCache{
		SFNAPI:                               sfnapi,
		describeStateMachineWithContextCache: describeStateMachineCacheWithContext,
	}, nil
}

// example: arn:aws:states:us-west-1:123:stateMachine:env--org--appdeploy---1--Start
var arnVersionRegex = regexp.MustCompile(`.*:stateMachine:.+--.+---1--.+`)

// DescribeStateMachineWithContext is cached aggressively since state machines are immutable.
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

	if !arnVersionRegex.MatchString(*out.StateMachineArn) {
		s.describeStateMachineWithContextCache.Add(cacheKey, out)
	}

	return out, nil
}
