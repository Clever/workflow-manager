package executor

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSFNUpdateWorkflowStatus(t *testing.T) {
	t.Log("TODO")
}

type stateMachineNameInput struct {
	wdName    string
	wdVersion int
	namespace string
	queue     string
}

type stateMachineNameTest struct {
	input  stateMachineNameInput
	output string
}

func TestStateMachineName(t *testing.T) {
	tests := []stateMachineNameTest{
		{
			input: stateMachineNameInput{
				wdName:    "cil-reliability-dashboard:sfn",
				wdVersion: 3,
				namespace: "production",
				queue:     "default",
			},
			output: "production--cil-reliability-dashboard-sfn--3--default",
		},
	}
	for _, test := range tests {
		output := stateMachineName(
			test.input.wdName,
			test.input.wdVersion,
			test.input.namespace,
			test.input.queue,
		)
		require.Equal(t, output, test.output, "input: %#v", test.input)
	}
}
