package embedded

import (
	"context"
	"testing"

	"github.com/Clever/workflow-manager/gen-go/client"
	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/stretchr/testify/require"
)

type newWorkflowDefinitionTest struct {
	description string
	wfm         client.Client
	input       *models.NewWorkflowDefinitionRequest
	expected    error
	assertions  func(context.Context, *testing.T, client.Client)
}

func (n newWorkflowDefinitionTest) Run(t *testing.T) {
	ctx := context.Background()
	_, err := n.wfm.NewWorkflowDefinition(ctx, n.input)
	require.Equal(t, n.expected, err)
	if n.assertions != nil {
		n.assertions(ctx, t, n.wfm)
	}
}

var newWorkflowDefinitionTests = []newWorkflowDefinitionTest{
	{
		description: "happy path",
		wfm:         &Embedded{},
		input: &models.NewWorkflowDefinitionRequest{
			Name: "test-wfd",
			StateMachine: &models.SLStateMachine{
				Comment: "this is a test",
			},
		},
		expected: nil,
		assertions: func(ctx context.Context, t *testing.T, wfm client.Client) {
			wfd, err := wfm.GetWorkflowDefinitionByNameAndVersion(ctx, &models.GetWorkflowDefinitionByNameAndVersionInput{Name: "test-wfd"})
			require.Nil(t, err)
			require.Equal(t, "this is a test", wfd.StateMachine.Comment)
		},
	},
}

func TestNewWorkflowDefinition(t *testing.T) {
	for _, ntest := range newWorkflowDefinitionTests {
		t.Run(ntest.description, ntest.Run)
	}
}
