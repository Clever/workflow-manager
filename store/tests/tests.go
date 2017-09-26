package tests

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/Clever/workflow-manager/resources"
	"github.com/Clever/workflow-manager/store"
	"github.com/stretchr/testify/require"
)

func RunStoreTests(t *testing.T, storeFactory func() store.Store) {
	t.Run("GetWorkflowDefinitions", GetWorkflowDefinitions(storeFactory(), t))
	t.Run("UpdateWorkflowDefinition", UpdateWorkflowDefinition(storeFactory(), t))
	t.Run("GetWorkflowDefinition", GetWorkflowDefinition(storeFactory(), t))
	t.Run("SaveWorkflowDefinition", SaveWorkflowDefinition(storeFactory(), t))
	t.Run("SaveStateResource", SaveStateResource(storeFactory(), t))
	t.Run("GetStateResource", GetStateResource(storeFactory(), t))
	t.Run("DeleteStateResource", DeleteStateResource(storeFactory(), t))
	t.Run("SaveWorkflow", SaveWorkflow(storeFactory(), t))
	t.Run("UpdateWorkflow", UpdateWorkflow(storeFactory(), t))
	t.Run("GetWorkflowByID", GetWorkflowByID(storeFactory(), t))
	t.Run("GetWorkflows", GetWorkflows(storeFactory(), t))
	t.Run("GetWorkflowsPagination", GetWorkflowsPagination(storeFactory(), t))
	t.Run("GetPendingWorkflowIDs", GetPendingWorkflowIDs(storeFactory(), t))
	t.Run("LockWorkflow/UnlockWorkflow", LockUnlockWorkflow(storeFactory(), t))
}

func UpdateWorkflowDefinition(s store.Store, t *testing.T) func(t *testing.T) {
	return func(t *testing.T) {
		// create kitchensink workflow
		wf := resources.KitchenSinkWorkflowDefinition(t)
		require.Nil(t, s.SaveWorkflowDefinition(wf))

		// get kitchensink workflow
		wflatest, err := s.LatestWorkflowDefinition(wf.Name())
		require.Nil(t, err)
		require.Equal(t, wflatest.Version(), 0)
		require.NotNil(t, wflatest.StartAt())
		require.Equal(t, wflatest.StartAt().Name(), "start-state")
		require.WithinDuration(t, wflatest.CreatedAt(), time.Now(), 1*time.Second)

		// update kitchensink workflow
		wflatest.Description = "update the description"
		wfupdated, err := s.UpdateWorkflowDefinition(wflatest)
		require.Nil(t, err)
		require.Equal(t, wfupdated.Description, "update the description")
		require.Equal(t, wfupdated.Version(), wflatest.Version()+1)
		require.WithinDuration(t, wfupdated.CreatedAt(), time.Now(), 1*time.Second)
		require.True(t, wfupdated.CreatedAt().After(wflatest.CreatedAt()))

		// get kitchensink workflow
		wflatest2, err := s.LatestWorkflowDefinition(wf.Name())
		require.Nil(t, err)
		require.Equal(t, wflatest2.Version(), wfupdated.Version())
		require.WithinDuration(t, wflatest2.CreatedAt(), time.Now(), 1*time.Second)
		require.Equal(t, wflatest2.CreatedAt(), wfupdated.CreatedAt())
	}
}

func GetWorkflowDefinitions(s store.Store, t *testing.T) func(t *testing.T) {
	return func(t *testing.T) {
		numWfs := 2
		for wfNum := 0; wfNum < numWfs; wfNum++ {
			wf := resources.KitchenSinkWorkflowDefinition(t)
			require.Nil(t, s.SaveWorkflowDefinition(wf))
		}
		wfs, err := s.GetWorkflowDefinitions()
		require.Nil(t, err)
		require.Equal(t, numWfs, len(wfs))
		// TODO more sophisticated test against versions, etc
	}
}

func GetWorkflowDefinition(s store.Store, t *testing.T) func(t *testing.T) {
	return func(t *testing.T) {
		wf := resources.KitchenSinkWorkflowDefinition(t)
		require.Nil(t, s.SaveWorkflowDefinition(wf))
		gwf, err := s.GetWorkflowDefinition(wf.Name(), wf.Version())
		require.Nil(t, err)
		require.Equal(t, wf.Name(), gwf.Name())
		require.Equal(t, wf.Version(), gwf.Version())
		require.WithinDuration(t, gwf.CreatedAt(), time.Now(), 1*time.Second)
		// TODO: deeper test of equality

		_, err = s.GetWorkflowDefinition("doesntexist", 1)
		require.NotNil(t, err)
		require.IsType(t, err, models.NotFound{})
	}
}

func SaveWorkflowDefinition(s store.Store, t *testing.T) func(t *testing.T) {
	return func(t *testing.T) {
		wf := resources.KitchenSinkWorkflowDefinition(t)
		require.Nil(t, s.SaveWorkflowDefinition(wf))

		err := s.SaveWorkflowDefinition(wf)
		require.NotNil(t, err)
		require.IsType(t, err, store.ConflictError{})

		// TODO: test behavior when workflow is invalid, e.g. breaks a length limit on a field / array
	}
}

func SaveStateResource(s store.Store, t *testing.T) func(t *testing.T) {
	return func(t *testing.T) {
		require.Nil(t, s.SaveStateResource(resources.NewBatchResource(
			"name",
			"namespace",
			"aws:batch:arn")))
		stateResource, err := s.GetStateResource("name", "namespace")
		require.Nil(t, err)
		require.Equal(t, "name", stateResource.Name)
		require.Equal(t, "namespace", stateResource.Namespace)
		require.Equal(t, "aws:batch:arn", stateResource.URI)
	}
}

func GetStateResource(s store.Store, t *testing.T) func(t *testing.T) {
	return func(t *testing.T) {
		require.Nil(t, s.SaveStateResource(resources.NewBatchResource(
			"name",
			"namespace",
			"aws:batch:arn")))
		stateResource, err := s.GetStateResource("name", "namespace")
		require.Nil(t, err)
		require.Equal(t, "name", stateResource.Name)
		require.Equal(t, "namespace", stateResource.Namespace)
		require.Equal(t, "aws:batch:arn", stateResource.URI)

		_, err = s.GetStateResource("doesntexist", "nope")
		require.NotNil(t, err)
		require.IsType(t, err, models.NotFound{})
	}
}

func DeleteStateResource(s store.Store, t *testing.T) func(t *testing.T) {
	return func(t *testing.T) {
		require.Nil(t, s.SaveStateResource(resources.NewBatchResource(
			"name",
			"namespace",
			"aws:batch:arn")))
		stateResource, err := s.GetStateResource("name", "namespace")
		require.Nil(t, err)
		require.Equal(t, "name", stateResource.Name)

		require.Nil(t, s.DeleteStateResource("name", "namespace"))
		_, err = s.GetStateResource("name", "namespace")
		require.Error(t, err)
		require.IsType(t, err, models.NotFound{})
	}
}

func SaveWorkflow(s store.Store, t *testing.T) func(t *testing.T) {
	return func(t *testing.T) {
		wf := resources.KitchenSinkWorkflowDefinition(t)
		require.Nil(t, s.SaveWorkflowDefinition(wf))
		tags := map[string]string{"team": "infra", "tag2": "value2"}
		workflow := resources.NewWorkflow(wf, []string{"input"}, "namespace", "queue", tags)
		require.Nil(t, s.SaveWorkflow(*workflow))
		// TODO: test behavior when workflow is invalid, e.g. breaks a length limit on a field / array
	}
}

func UpdateWorkflow(s store.Store, t *testing.T) func(t *testing.T) {
	return func(t *testing.T) {
		wf := resources.KitchenSinkWorkflowDefinition(t)
		require.Nil(t, s.SaveWorkflowDefinition(wf))
		input := []string{"input"}
		tags := map[string]string{"team": "infra", "tag2": "value2"}
		workflow := resources.NewWorkflow(wf, input, "namespace", "queue", tags)
		require.Nil(t, s.SaveWorkflow(*workflow))

		updatedWorkflow, err := s.GetWorkflowByID(workflow.ID)
		require.Nil(t, err)
		updatedWorkflow.Status = resources.Succeeded
		require.Nil(t, s.UpdateWorkflow(updatedWorkflow))

		savedWorkflow, err := s.GetWorkflowByID(workflow.ID)
		require.Nil(t, err)
		require.Equal(t, savedWorkflow.Status, resources.Succeeded)
		require.WithinDuration(t, savedWorkflow.CreatedAt, time.Now(), 1*time.Second)
		require.WithinDuration(t, savedWorkflow.LastUpdated, time.Now(), 1*time.Second)
		require.True(t, savedWorkflow.LastUpdated.After(savedWorkflow.CreatedAt))
		require.NotEqual(t, savedWorkflow.LastUpdated, savedWorkflow.CreatedAt)
	}
}

func GetWorkflowByID(s store.Store, t *testing.T) func(t *testing.T) {
	return func(t *testing.T) {
		wf := resources.KitchenSinkWorkflowDefinition(t)
		require.Nil(t, s.SaveWorkflowDefinition(wf))
		input := []string{"input"}
		tags := map[string]string{"team": "infra", "tag2": "value2"}
		workflow := resources.NewWorkflow(wf, input, "namespace", "queue", tags)
		require.Nil(t, s.SaveWorkflow(*workflow))

		savedWorkflow, err := s.GetWorkflowByID(workflow.ID)
		require.Nil(t, err)

		expected := resources.Workflow{
			ID:                 workflow.ID,
			WorkflowDefinition: wf,
			Input:              input,
			Status:             resources.Queued,
			Namespace:          "namespace",
			Queue:              "queue",
			Tags:               tags,
		}
		require.Equal(t, savedWorkflow.Input, expected.Input)
		require.Equal(t, savedWorkflow.Status, expected.Status)
		require.Equal(t, savedWorkflow.Namespace, expected.Namespace)
		require.Equal(t, savedWorkflow.Queue, expected.Queue)
		require.Equal(t, savedWorkflow.Tags, expected.Tags)
		require.WithinDuration(t, savedWorkflow.CreatedAt, time.Now(), 1*time.Second)
		require.Equal(t, savedWorkflow.CreatedAt, savedWorkflow.LastUpdated)
	}
}

func GetWorkflows(s store.Store, t *testing.T) func(t *testing.T) {
	return func(t *testing.T) {
		// Set up workflows:
		definition := resources.KitchenSinkWorkflowDefinition(t)
		require.NoError(t, s.SaveWorkflowDefinition(definition))

		tags := map[string]string{"team": "infra", "tag2": "value2"}

		runningWorkflow := resources.NewWorkflow(definition, []string{"input"}, "namespace", "queue", tags)
		runningWorkflow.Status = resources.Running
		require.NoError(t, s.SaveWorkflow(*runningWorkflow))

		failedWorkflow := resources.NewWorkflow(definition, []string{"input"}, "namespace", "queue", tags)
		failedWorkflow.Status = resources.Failed
		require.NoError(t, s.SaveWorkflow(*failedWorkflow))

		// Set up workflows for a separate definition that will be ignored by the query:
		otherWorkflowDefinition := resources.KitchenSinkWorkflowDefinition(t)
		require.NoError(t, s.SaveWorkflowDefinition(otherWorkflowDefinition))

		otherDefinitionWorkflow := resources.NewWorkflow(
			otherWorkflowDefinition, []string{"input"}, "namespace", "queue", tags,
		)
		otherDefinitionWorkflow.Status = resources.Running
		require.NoError(t, s.SaveWorkflow(*otherDefinitionWorkflow))

		// Verify results for query with no status filtering:
		workflows, _, err := s.GetWorkflows(&store.WorkflowQuery{
			DefinitionName: definition.Name(),
			Limit:          10,
		})
		require.NoError(t, err)
		require.Len(t, workflows, 2)
		require.Equal(t, failedWorkflow.ID, workflows[0].ID)
		require.Equal(t, runningWorkflow.ID, workflows[1].ID)

		// Verify results for query with status filtering:
		workflows, _, err = s.GetWorkflows(&store.WorkflowQuery{
			DefinitionName: definition.Name(),
			Status:         string(resources.Running),
			Limit:          10,
		})
		require.NoError(t, err)
		require.Len(t, workflows, 1)
		require.Equal(t, runningWorkflow.ID, workflows[0].ID)
	}
}

func GetWorkflowsPagination(s store.Store, t *testing.T) func(t *testing.T) {
	return func(t *testing.T) {
		definition := resources.KitchenSinkWorkflowDefinition(t)
		require.NoError(t, s.SaveWorkflowDefinition(definition))

		workflow1 := resources.NewWorkflow(definition, []string{"input"}, "namespace", "queue", map[string]string{})
		workflow1.Status = resources.Running
		require.NoError(t, s.SaveWorkflow(*workflow1))

		workflow2 := resources.NewWorkflow(definition, []string{"input"}, "namespace", "queue", map[string]string{})
		workflow2.Status = resources.Succeeded
		require.NoError(t, s.SaveWorkflow(*workflow2))

		workflow3 := resources.NewWorkflow(definition, []string{"input"}, "namespace", "queue", map[string]string{})
		workflow3.Status = resources.Running
		require.NoError(t, s.SaveWorkflow(*workflow3))

		limit := 1
		getAllPages := func(query store.WorkflowQuery) []resources.Workflow {
			nextPageToken := ""
			nextQuery := query
			workflows := []resources.Workflow{}
			workflowsPage := []resources.Workflow{}
			var err error
			for {
				workflowsPage, nextPageToken, err = s.GetWorkflows(&nextQuery)
				require.NoError(t, err)

				nextQuery.PageToken = nextPageToken

				// Make sure we always have exactly <limit> items if there's a next page token returned.
				if nextPageToken != "" {
					require.Len(t, workflowsPage, limit)
				} else {
					require.True(t, len(workflowsPage) <= limit)
				}

				workflows = append(workflows, workflowsPage...)

				if nextPageToken == "" {
					break
				}
			}

			return workflows
		}
		query := store.WorkflowQuery{
			DefinitionName: definition.Name(),
			Limit:          limit,
			Status:         string(resources.Running),
		}

		workflows := getAllPages(query)
		require.Len(t, workflows, 2)
		require.Equal(t, workflow3.ID, workflows[0].ID)
		require.Equal(t, workflow1.ID, workflows[1].ID)

		// Make sure paging works in both sort directions.
		query.OldestFirst = true
		workflows = getAllPages(query)
		require.Len(t, workflows, 2)
		require.Equal(t, workflow1.ID, workflows[0].ID)
		require.Equal(t, workflow3.ID, workflows[1].ID)

		// Verify handling for invalid page tokens.
		query.PageToken = "invalid token"
		workflows, nextPageToken, err := s.GetWorkflows(&query)
		assert.Error(t, err)
		assert.IsType(t, store.InvalidPageTokenError{}, err)
		assert.Equal(t, "", nextPageToken)
		assert.Len(t, workflows, 0)
	}
}

func GetPendingWorkflowIDs(s store.Store, t *testing.T) func(t *testing.T) {
	return func(t *testing.T) {
		wf1 := resources.KitchenSinkWorkflowDefinition(t)
		require.Nil(t, s.SaveWorkflowDefinition(wf1))
		wf2 := resources.KitchenSinkWorkflowDefinition(t)
		require.Nil(t, s.SaveWorkflowDefinition(wf2))
		input := []string{"input"}
		tags := map[string]string{"team": "infra", "tag2": "value2"}
		workflow1 := resources.NewWorkflow(wf1, input, "namespace", "queue", tags)
		require.Nil(t, s.SaveWorkflow(*workflow1))
		workflow2 := resources.NewWorkflow(wf2, input, "namespace", "queue", tags)
		require.Nil(t, s.SaveWorkflow(*workflow2))

		pendingWorkflowIDs, err := s.GetPendingWorkflowIDs()
		require.Nil(t, err)
		require.Equal(t, pendingWorkflowIDs, []string{workflow1.ID, workflow2.ID})

		workflow1.Status = resources.Running
		require.Nil(t, s.UpdateWorkflow(*workflow1))

		pendingWorkflowIDs, err = s.GetPendingWorkflowIDs()
		require.Nil(t, err)
		require.Equal(t, pendingWorkflowIDs, []string{workflow2.ID, workflow1.ID})

		workflow2.Status = resources.Succeeded
		require.Nil(t, s.UpdateWorkflow(*workflow2))

		pendingWorkflowIDs, err = s.GetPendingWorkflowIDs()
		require.Nil(t, err)
		require.Equal(t, pendingWorkflowIDs, []string{workflow1.ID})

	}
}

func LockUnlockWorkflow(s store.Store, t *testing.T) func(t *testing.T) {
	return func(t *testing.T) {
		wfd1 := resources.KitchenSinkWorkflowDefinition(t)
		require.Nil(t, s.SaveWorkflowDefinition(wfd1))
		input := []string{"input"}
		tags := map[string]string{"team": "infra", "tag2": "value2"}
		wf1 := resources.NewWorkflow(wfd1, input, "namespace", "queue", tags)
		require.Nil(t, s.SaveWorkflow(*wf1))

		require.Nil(t, s.LockWorkflow(wf1.ID))
		require.Equal(t, s.LockWorkflow(wf1.ID), store.ErrWorkflowLocked)
		require.Nil(t, s.UnlockWorkflow(wf1.ID))
		require.Nil(t, s.UnlockWorkflow(wf1.ID)) // unlocking an unlocked workflow is ok
		require.Nil(t, s.LockWorkflow(wf1.ID))   // can reacquire the lock
	}
}
