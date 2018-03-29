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
