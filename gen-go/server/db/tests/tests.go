package tests

import (
	"context"
	"testing"

	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/Clever/workflow-manager/gen-go/server/db"
	"github.com/stretchr/testify/require"
)

func RunDBTests(t *testing.T, dbFactory func() db.Interface) {
	t.Run("GetWorkflowDefinition", GetWorkflowDefinition(dbFactory(), t))
	t.Run("GetWorkflowDefinitionsByNameAndVersion", GetWorkflowDefinitionsByNameAndVersion(dbFactory(), t))
	t.Run("SaveWorkflowDefinition", SaveWorkflowDefinition(dbFactory(), t))
	t.Run("DeleteWorkflowDefinition", DeleteWorkflowDefinition(dbFactory(), t))
}

func GetWorkflowDefinition(s db.Interface, t *testing.T) func(t *testing.T) {
	return func(t *testing.T) {
		ctx := context.Background()
		m := models.WorkflowDefinition{
			Name:    "string1",
			Version: 1,
		}
		require.Nil(t, s.SaveWorkflowDefinition(ctx, m))
		m2, err := s.GetWorkflowDefinition(ctx, m.Name, m.Version)
		require.Nil(t, err)
		require.Equal(t, m.Name, m2.Name)
		require.Equal(t, m.Version, m2.Version)

		_, err = s.GetWorkflowDefinition(ctx, "string2", 2)
		require.NotNil(t, err)
		require.IsType(t, err, db.ErrWorkflowDefinitionNotFound{})
	}
}

type getWorkflowDefinitionsByNameAndVersionInput struct {
	ctx   context.Context
	input db.GetWorkflowDefinitionsByNameAndVersionInput
}
type getWorkflowDefinitionsByNameAndVersionOutput struct {
	workflowDefinitions []models.WorkflowDefinition
	err                 error
}
type getWorkflowDefinitionsByNameAndVersionTest struct {
	testName string
	d        db.Interface
	input    getWorkflowDefinitionsByNameAndVersionInput
	output   getWorkflowDefinitionsByNameAndVersionOutput
}

func (g getWorkflowDefinitionsByNameAndVersionTest) run(t *testing.T) {
	workflowDefinitions, err := g.d.GetWorkflowDefinitionsByNameAndVersion(g.input.ctx, g.input.input)
	require.Equal(t, g.output.err, err)
	require.Equal(t, g.output.workflowDefinitions, workflowDefinitions)
}

func GetWorkflowDefinitionsByNameAndVersion(d db.Interface, t *testing.T) func(t *testing.T) {
	return func(t *testing.T) {
		ctx := context.Background()
		require.Nil(t, d.SaveWorkflowDefinition(ctx, models.WorkflowDefinition{
			Name:    "string1",
			Version: 1,
		}))
		require.Nil(t, d.SaveWorkflowDefinition(ctx, models.WorkflowDefinition{
			Name:    "string1",
			Version: 2,
		}))
		require.Nil(t, d.SaveWorkflowDefinition(ctx, models.WorkflowDefinition{
			Name:    "string1",
			Version: 3,
		}))
		tests := []getWorkflowDefinitionsByNameAndVersionTest{
			{
				testName: "basic",
				d:        d,
				input: getWorkflowDefinitionsByNameAndVersionInput{
					ctx: context.Background(),
					input: db.GetWorkflowDefinitionsByNameAndVersionInput{
						Name: "string1",
					},
				},
				output: getWorkflowDefinitionsByNameAndVersionOutput{
					workflowDefinitions: []models.WorkflowDefinition{
						models.WorkflowDefinition{
							Name:    "string1",
							Version: 1,
						},
						models.WorkflowDefinition{
							Name:    "string1",
							Version: 2,
						},
						models.WorkflowDefinition{
							Name:    "string1",
							Version: 3,
						},
					},
					err: nil,
				},
			},
			{
				testName: "descending",
				d:        d,
				input: getWorkflowDefinitionsByNameAndVersionInput{
					ctx: context.Background(),
					input: db.GetWorkflowDefinitionsByNameAndVersionInput{
						Name:       "string1",
						Descending: true,
					},
				},
				output: getWorkflowDefinitionsByNameAndVersionOutput{
					workflowDefinitions: []models.WorkflowDefinition{
						models.WorkflowDefinition{
							Name:    "string1",
							Version: 3,
						},
						models.WorkflowDefinition{
							Name:    "string1",
							Version: 2,
						},
						models.WorkflowDefinition{
							Name:    "string1",
							Version: 1,
						},
					},
					err: nil,
				},
			},
			{
				testName: "starting after",
				d:        d,
				input: getWorkflowDefinitionsByNameAndVersionInput{
					ctx: context.Background(),
					input: db.GetWorkflowDefinitionsByNameAndVersionInput{
						Name:              "string1",
						VersionStartingAt: db.Int64(2),
					},
				},
				output: getWorkflowDefinitionsByNameAndVersionOutput{
					workflowDefinitions: []models.WorkflowDefinition{
						models.WorkflowDefinition{
							Name:    "string1",
							Version: 2,
						},
						models.WorkflowDefinition{
							Name:    "string1",
							Version: 3,
						},
					},
					err: nil,
				},
			},
		}
		for _, test := range tests {
			t.Run(test.testName, test.run)
		}
	}
}

func SaveWorkflowDefinition(s db.Interface, t *testing.T) func(t *testing.T) {
	return func(t *testing.T) {
		ctx := context.Background()
		m := models.WorkflowDefinition{
			Name:    "string1",
			Version: 1,
		}
		require.Nil(t, s.SaveWorkflowDefinition(ctx, m))
		require.Equal(t, db.ErrWorkflowDefinitionAlreadyExists{
			Name:    "string1",
			Version: 1,
		}, s.SaveWorkflowDefinition(ctx, m))
	}
}

func DeleteWorkflowDefinition(s db.Interface, t *testing.T) func(t *testing.T) {
	return func(t *testing.T) {
		ctx := context.Background()
		m := models.WorkflowDefinition{
			Name:    "string1",
			Version: 1,
		}
		require.Nil(t, s.SaveWorkflowDefinition(ctx, m))
		require.Nil(t, s.DeleteWorkflowDefinition(ctx, m.Name, m.Version))
	}
}
