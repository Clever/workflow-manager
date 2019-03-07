package sfnfunction

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sfn"
	"github.com/stretchr/testify/assert"
)

type newTest struct {
	fn       interface{}
	expected error
}

func (nt newTest) Run(t *testing.T) {
	_, got := New(nt.fn)
	assert.Equal(t, nt.expected, got)
}

var newTests = []newTest{
	{
		fn:       func() {},
		expected: nil,
	},
	{
		fn:       struct{}{},
		expected: ErrBadFunctionSignature{"handler kind struct is not func"},
	},
	{
		fn:       func(a1, a2, a3 string) {},
		expected: ErrBadFunctionSignature{"function may not take more than two arguments, but function takes 3"},
	},
	{
		fn:       func(a1, a2 string) {},
		expected: ErrBadFunctionSignature{"function takes two arguments, but the first is not Context. got string"},
	},
	{
		fn:       func(s string, s2 string) {},
		expected: ErrBadFunctionSignature{"function takes two arguments, but the first is not Context. got string"},
	},
	{
		fn:       func() string { return "" },
		expected: ErrBadFunctionSignature{"function returns a single value, but it does not implement error"},
	},
	{
		fn:       func() (string, string) { return "", "" },
		expected: ErrBadFunctionSignature{"function returns two values, but the second does not implement error"},
	},
}

func TestNew(t *testing.T) {
	for i, nt := range newTests {
		t.Run(fmt.Sprintf("%d", i), nt.Run)
	}
}

func mustNew(fn interface{}) *Resource {
	r, err := New(fn)
	if err != nil {
		panic(err)
	}
	return r
}

type callTest struct {
	r        *Resource
	input    string
	expected Result
}

func (ct callTest) Run(t *testing.T) {
	got := ct.r.Call(context.Background(), ct.input)
	assert.Equal(t, ct.expected, got)
}

var callTests = []callTest{
	{
		mustNew(func(echo string) (string, error) {
			return echo, nil
		}),
		// inputs and outputs are all json unmarshal/marshal-able
		`"happy path"`,
		Result{Success: &sfn.SendTaskSuccessInput{
			Output: aws.String(`"happy path"`),
		}},
	},
	{
		mustNew(func(echo string) (string, error) {
			return "", errors.New(echo)
		}),
		`"error path"`,
		Result{Failure: &sfn.SendTaskFailureInput{
			Error: aws.String("errorString"),
			Cause: aws.String("error path"),
		}},
	},
}

func TestCall(t *testing.T) {
	for i, ct := range callTests {
		t.Run(fmt.Sprintf("%d", i), ct.Run)
	}
}
