package embedded

import (
	"context"
	"fmt"
	"strings"
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
	{
		description: "error - workflow already exists",
		wfm: &Embedded{
			workflowDefinitions: []models.WorkflowDefinition{
				models.WorkflowDefinition{
					Name: "test-wfd",
				},
			},
		},
		input: &models.NewWorkflowDefinitionRequest{
			Name: "test-wfd",
			StateMachine: &models.SLStateMachine{
				Comment: "this is a test",
			},
		},
		expected: fmt.Errorf("test-wfd workflow definition already exists"),
	},
}

func TestNewWorkflowDefinition(t *testing.T) {
	for _, ntest := range newWorkflowDefinitionTests {
		t.Run(ntest.description, ntest.Run)
	}
}

type parseWorkflowDefinitionTest struct {
	description string
	input       string
	assertions  func(*testing.T, models.WorkflowDefinition, error)
}

func (n parseWorkflowDefinitionTest) Run(t *testing.T) {
	out, err := ParseWorkflowDefinition([]byte(n.input))
	if n.assertions != nil {
		n.assertions(t, out, err)
	}
}

var testHelloWorldWorkflowDefintionYAML = `
manager: step-functions
name: hello-world
stateMachine:
  Version: '1.0'
  StartAt: start
  States:
    start:
      Type: Task
      Resource: printer
      HeartbeatSeconds: 30
      End: true
`

var testMalformedWorkflowDefintionYAML = `
manager: step-functions
name: hello-world
stateMachine:
  Version: '1.0'
  StartAt: start
  States:
    start:
      Type: Task
	Resource: printer
      HeartbeatSeconds: 30
      End: true
`

var parseWorkflowDefinitionTests = []parseWorkflowDefinitionTest{
	{
		description: "happy path",
		input:       testHelloWorldWorkflowDefintionYAML,
		assertions: func(t *testing.T, wfd models.WorkflowDefinition, err error) {
			require.Nil(t, err)
			require.Equal(t, "hello-world", wfd.Name)
		},
	},
	{
		description: "err - malformed yaml",
		input:       testMalformedWorkflowDefintionYAML,
		assertions: func(t *testing.T, wfd models.WorkflowDefinition, err error) {
			require.Error(t, err)
			// verify we're getting an error from the YAML lib
			require.Contains(t, strings.ToLower(err.Error()), "yaml")
		},
	},
}

func TestParseWorkflowDefintion(t *testing.T) {
	for _, ntest := range parseWorkflowDefinitionTests {
		t.Run(ntest.description, ntest.Run)
	}
}
