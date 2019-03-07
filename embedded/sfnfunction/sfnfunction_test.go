package sfnfunction

import (
	"fmt"
	"testing"

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
