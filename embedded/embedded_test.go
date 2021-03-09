package embedded

import (
	"context"
	"testing"

	"github.com/Clever/workflow-manager/embedded/sfnfunction"
	"github.com/Clever/workflow-manager/gen-go/client"
	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/stretchr/testify/require"
	errors "golang.org/x/xerrors"
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
	if n.expected == nil {
		require.Equal(t, n.expected, err)
	} else {
		require.EqualError(t, err, n.expected.Error())
	}
	if n.assertions != nil {
		n.assertions(ctx, t, n.wfm)
	}
}

var testWorkflowDefinition = models.WorkflowDefinition{
	DefaultTags: map[string]interface{}{
		"hashtag": "yoloswag",
	},
	Manager: models.Manager("gcp"),
	Name:    "test-wfd",
	StateMachine: &models.SLStateMachine{
		Comment: "this is a test",
		States: map[string]models.SLState{
			"test-state": models.SLState{
				Type: models.SLStateTypeSucceed,
				End:  true,
			},
		},
	},
	Version: 5,
}

var newWorkflowDefinitionTests = []newWorkflowDefinitionTest{
	{
		description: "happy path",
		wfm:         &Embedded{},
		input: &models.NewWorkflowDefinitionRequest{
			DefaultTags:  testWorkflowDefinition.DefaultTags,
			Manager:      testWorkflowDefinition.Manager,
			Name:         testWorkflowDefinition.Name,
			StateMachine: testWorkflowDefinition.StateMachine,
			Version:      testWorkflowDefinition.Version,
		},
		expected: nil,
		assertions: func(ctx context.Context, t *testing.T, wfm client.Client) {
			wfd, err := wfm.GetWorkflowDefinitionByNameAndVersion(ctx, &models.GetWorkflowDefinitionByNameAndVersionInput{Name: testWorkflowDefinition.Name})
			require.Nil(t, err)
			require.Equal(t, testWorkflowDefinition.DefaultTags, wfd.DefaultTags)
			require.Equal(t, testWorkflowDefinition.Manager, wfd.Manager)
			require.Equal(t, testWorkflowDefinition.Name, wfd.Name)
			require.Equal(t, testWorkflowDefinition.StateMachine, wfd.StateMachine)
			require.Equal(t, testWorkflowDefinition.Version, wfd.Version)
		},
	},
	{
		description: "happy path - replace Version -1 workflow definitions",
		wfm: &Embedded{
			workflowDefinitions: []models.WorkflowDefinition{
				models.WorkflowDefinition{
					Name:         "test-wfd",
					StateMachine: testWorkflowDefinition.StateMachine,
					Version:      -1,
				},
			},
		},
		input: &models.NewWorkflowDefinitionRequest{
			Name: testWorkflowDefinition.Name,
			StateMachine: &models.SLStateMachine{
				Comment: "this is the new world",
				States: map[string]models.SLState{
					"new-state": models.SLState{
						Type: models.SLStateTypeSucceed,
						End:  true,
					},
				},
			},
		},
		assertions: func(ctx context.Context, t *testing.T, wfm client.Client) {
			wfd, err := wfm.GetWorkflowDefinitionByNameAndVersion(ctx, &models.GetWorkflowDefinitionByNameAndVersionInput{Name: testWorkflowDefinition.Name})
			require.Nil(t, err)
			require.Equal(t, testWorkflowDefinition.Name, wfd.Name)
			require.Equal(t, &models.SLStateMachine{
				Comment: "this is the new world",
				States: map[string]models.SLState{
					"new-state": models.SLState{
						Type: models.SLStateTypeSucceed,
						End:  true,
					},
				},
			}, wfd.StateMachine)
		},
	},
	{
		description: "error - workflow already exists",
		wfm: &Embedded{
			workflowDefinitions: []models.WorkflowDefinition{
				testWorkflowDefinition,
			},
		},
		input: &models.NewWorkflowDefinitionRequest{
			Name: testWorkflowDefinition.Name,
			StateMachine: &models.SLStateMachine{
				Comment: "this won't replace",
			},
		},
		expected: errors.Errorf("test-wfd workflow definition already exists"),
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
version: -1.0
stateMachine:
  Version: '-1.0'
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
			require.Equal(t, int64(-1), wfd.Version)
		},
	},
	// {
	// 	description: "err - malformed yaml",
	// 	input:       testMalformedWorkflowDefintionYAML,
	// 	assertions: func(t *testing.T, wfd models.WorkflowDefinition, err error) {
	// 		require.Error(t, err)
	// 		// verify we're getting an error from the YAML lib
	// 		require.Contains(t, strings.ToLower(err.Error()), "yaml")
	// 	},
	// },
}

func TestParseWorkflowDefintion(t *testing.T) {
	for _, ntest := range parseWorkflowDefinitionTests {
		t.Run(ntest.description, ntest.Run)
	}
}

type validateWorkflowDefinitionStatesTest struct {
	description string
	input       models.WorkflowDefinition
	assertions  func(*testing.T, error)
}

func (n validateWorkflowDefinitionStatesTest) Run(t *testing.T) {
	resources := map[string]*sfnfunction.Resource{}
	t.Log(n.description)
	err := validateWorkflowDefinitionStates(n.input, resources)
	if n.assertions != nil {
		n.assertions(t, err)
	}
}

var validateWorkflowDefinitionStatesTests = []validateWorkflowDefinitionStatesTest{
	{
		description: "validate task state - resource does not exist",
		input: models.WorkflowDefinition{
			StateMachine: &models.SLStateMachine{
				States: map[string]models.SLState{
					"task": models.SLState{
						Type:     models.SLStateTypeTask,
						Resource: "dne",
						End:      true,
					},
				},
			},
		},
		assertions: func(t *testing.T, err error) {
			require.Error(t, err)
			require.Contains(t, err.Error(), "unknown resource")
		},
	},
	{
		description: "validate task state - resource empty",
		input: models.WorkflowDefinition{
			StateMachine: &models.SLStateMachine{
				States: map[string]models.SLState{
					"task": models.SLState{
						Type: models.SLStateTypeTask,
						End:  true,
					},
				},
			},
		},
		assertions: func(t *testing.T, err error) {
			require.Error(t, err)
			require.Contains(t, err.Error(), "specify resource")
		},
	},
	{
		description: "validate pass state - empty results",
		input: models.WorkflowDefinition{
			StateMachine: &models.SLStateMachine{
				States: map[string]models.SLState{
					"pass": models.SLState{
						Type: models.SLStateTypePass,
						End:  true,
					},
				},
			},
		},
		assertions: func(t *testing.T, err error) {
			require.Error(t, err)
			require.Contains(t, err.Error(), "specify results")
		},
	},
	{
		description: "validate choice state - no choices",
		input: models.WorkflowDefinition{
			StateMachine: &models.SLStateMachine{
				States: map[string]models.SLState{
					"choice": models.SLState{
						Type: models.SLStateTypeChoice,
						End:  true,
					},
				},
			},
		},
		assertions: func(t *testing.T, err error) {
			require.Error(t, err)
			require.Contains(t, err.Error(), "choice")
		},
	},
	{
		description: "validate wait state - invalid wait",
		input: models.WorkflowDefinition{
			StateMachine: &models.SLStateMachine{
				States: map[string]models.SLState{
					"wait": models.SLState{
						Type: models.SLStateTypeWait,
						End:  true,
					},
				},
			},
		},
		assertions: func(t *testing.T, err error) {
			require.Error(t, err)
			require.Contains(t, err.Error(), "seconds parameter")
		},
	},
	{
		description: "validate succeed state",
		input: models.WorkflowDefinition{
			StateMachine: &models.SLStateMachine{
				States: map[string]models.SLState{
					"succeed": models.SLState{
						Type: models.SLStateTypeSucceed,
						End:  true,
					},
				},
			},
		},
		assertions: func(t *testing.T, err error) {
			require.NoError(t, err)
		},
	},
	{
		description: "validate fail state",
		input: models.WorkflowDefinition{
			StateMachine: &models.SLStateMachine{
				States: map[string]models.SLState{
					"fail": models.SLState{
						Type: models.SLStateTypeFail,
						End:  true,
					},
				},
			},
		},
		assertions: func(t *testing.T, err error) {
			require.NoError(t, err)
		},
	},
	{
		description: "validate parallel state",
		input: models.WorkflowDefinition{
			StateMachine: &models.SLStateMachine{
				States: map[string]models.SLState{
					"parallel": models.SLState{
						Type: models.SLStateTypeParallel,
						End:  true,
					},
				},
			},
		},
		assertions: func(t *testing.T, err error) {
			require.NoError(t, err)
		},
	},
	{
		description: "validate map state (invalid inner)",
		input: models.WorkflowDefinition{
			StateMachine: &models.SLStateMachine{
				States: map[string]models.SLState{
					"map": models.SLState{
						Type:     models.SLStateTypeMap,
						Iterator: &models.SLStateMachine{},
						End:      true,
					},
				},
			},
		},
		assertions: func(t *testing.T, err error) {
			require.Error(t, err)
		},
	},
	{
		description: "validate map state (valid inner)",
		input: models.WorkflowDefinition{
			StateMachine: &models.SLStateMachine{
				States: map[string]models.SLState{
					"map": models.SLState{
						Type: models.SLStateTypeMap,
						Iterator: &models.SLStateMachine{
							States: map[string]models.SLState{
								"pass": models.SLState{
									End:    true,
									Type:   models.SLStateTypePass,
									Result: "result",
								},
							},
						},
						End: true,
					},
				},
			},
		},
		assertions: func(t *testing.T, err error) {
			require.NoError(t, err)
		},
	},
	{
		description: "invalid state type",
		input: models.WorkflowDefinition{
			StateMachine: &models.SLStateMachine{
				States: map[string]models.SLState{
					"state": models.SLState{
						Type: models.SLStateType("whodis"),
						End:  true,
					},
				},
			},
		},
		assertions: func(t *testing.T, err error) {
			require.Error(t, err)
			require.Contains(t, err.Error(), "invalid state type")
		},
	},
	{
		description: "validate next state - no state",
		input: models.WorkflowDefinition{
			StateMachine: &models.SLStateMachine{
				States: map[string]models.SLState{
					"pass": models.SLState{
						Type:   models.SLStateTypePass,
						Result: "passing",
					},
				},
			},
		},
		assertions: func(t *testing.T, err error) {
			require.Error(t, err)
			require.Contains(t, err.Error(), "must specify next state")
		},
	},
	{
		description: "validate end state exists",
		input: models.WorkflowDefinition{
			StateMachine: &models.SLStateMachine{
				States: map[string]models.SLState{
					"no-end-state": models.SLState{
						Type: models.SLStateTypeSucceed,
					},
				},
			},
		},
		assertions: func(t *testing.T, err error) {
			require.Error(t, err)
			require.Contains(t, err.Error(), "end state")
		},
	},
}

func TestValidateWorkflowDefintionStates(t *testing.T) {
	for _, ntest := range validateWorkflowDefinitionStatesTests {
		t.Run(ntest.description, ntest.Run)
	}
}
