package tests

import (
	"context"
	"fmt"
	"testing"
	"time"

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
	t.Run("UpdateLargeWorkflow", UpdateLargeWorkflow(storeFactory(), t))
	t.Run("DeleteWorkflow", DeleteWorkflow(storeFactory(), t))
	t.Run("GetWorkflowByID", GetWorkflowByID(storeFactory(), t))
}

func UpdateWorkflowDefinition(s store.Store, t *testing.T) func(t *testing.T) {
	return func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		// create kitchensink workflow
		wf := resources.KitchenSinkWorkflowDefinition(t)
		require.Nil(t, s.SaveWorkflowDefinition(ctx, *wf))

		// get kitchensink workflow
		wflatest, err := s.LatestWorkflowDefinition(ctx, wf.Name)
		require.Nil(t, err)
		require.Equal(t, wflatest.Version, int64(0))
		require.Equal(t, wflatest.StateMachine.StartAt, "start-state")
		require.WithinDuration(t, time.Time(wflatest.CreatedAt), time.Now(), 1*time.Second)

		// update kitchensink workflow
		wflatest.StateMachine.Comment = "update the description"
		wfupdated, err := s.UpdateWorkflowDefinition(ctx, wflatest)
		require.NoError(t, err)
		require.Equal(t, wfupdated.StateMachine.Comment, "update the description")
		require.Equal(t, wfupdated.Version, wflatest.Version+1)
		require.WithinDuration(t, time.Time(wfupdated.CreatedAt), time.Now(), 1*time.Second)
		require.True(t, time.Time(wfupdated.CreatedAt).After(time.Time(wflatest.CreatedAt)))

		// get kitchensink workflow
		wflatest2, err := s.LatestWorkflowDefinition(ctx, wf.Name)
		require.Nil(t, err)
		require.Equal(t, wflatest2.Version, wfupdated.Version)
		require.WithinDuration(t, time.Time(wflatest2.CreatedAt), time.Now(), 1*time.Second)
		require.Equal(t, time.Time(wflatest2.CreatedAt), time.Time(wfupdated.CreatedAt))
	}
}

func GetWorkflowDefinitions(s store.Store, t *testing.T) func(t *testing.T) {
	return func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		numWfs := 2
		for wfNum := 0; wfNum < numWfs; wfNum++ {
			wf := resources.KitchenSinkWorkflowDefinition(t)
			require.Nil(t, s.SaveWorkflowDefinition(ctx, *wf))
		}
		wfs, err := s.GetWorkflowDefinitions(ctx)
		require.Nil(t, err)
		require.Equal(t, numWfs, len(wfs))
		// TODO more sophisticated test against versions, etc
	}
}

func GetWorkflowDefinition(s store.Store, t *testing.T) func(t *testing.T) {
	return func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		wf := resources.KitchenSinkWorkflowDefinition(t)
		require.Nil(t, s.SaveWorkflowDefinition(ctx, *wf))
		gwf, err := s.GetWorkflowDefinition(ctx, wf.Name, int(wf.Version))
		require.Nil(t, err)
		require.Equal(t, wf.Name, gwf.Name)
		require.Equal(t, wf.Version, gwf.Version)
		require.WithinDuration(t, time.Time(gwf.CreatedAt), time.Now(), 1*time.Second)
		// TODO: deeper test of equality

		_, err = s.GetWorkflowDefinition(ctx, "doesntexist", 1)
		require.NotNil(t, err)
		require.IsType(t, err, models.NotFound{})
	}
}

func SaveWorkflowDefinition(s store.Store, t *testing.T) func(t *testing.T) {
	return func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		wf := resources.KitchenSinkWorkflowDefinition(t)
		require.Nil(t, s.SaveWorkflowDefinition(ctx, *wf))

		err := s.SaveWorkflowDefinition(ctx, *wf)
		require.NotNil(t, err)
		require.IsType(t, err, store.ConflictError{})

		// TODO: test behavior when workflow is invalid, e.g. breaks a length limit on a field / array
	}
}

func SaveStateResource(s store.Store, t *testing.T) func(t *testing.T) {
	return func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		sr := &models.StateResource{
			Name:      "name",
			Namespace: "namespace",
			Type:      models.StateResourceTypeActivityARN,
			URI:       "arn:activity",
		}
		require.Nil(t, s.SaveStateResource(ctx, *sr))
		stateResource, err := s.GetStateResource(ctx, sr.Name, sr.Namespace)
		require.Nil(t, err)
		require.Equal(t, sr.Name, stateResource.Name)
		require.Equal(t, sr.Namespace, stateResource.Namespace)
		require.Equal(t, sr.URI, stateResource.URI)
	}
}

func GetStateResource(s store.Store, t *testing.T) func(t *testing.T) {
	return func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		sr := &models.StateResource{
			Name:      "name",
			Namespace: "namespace",
			Type:      models.StateResourceTypeActivityARN,
			URI:       "arn:activity",
		}
		require.Nil(t, s.SaveStateResource(ctx, *sr))
		stateResource, err := s.GetStateResource(ctx, sr.Name, sr.Namespace)
		require.Nil(t, err)
		require.Equal(t, sr.Name, stateResource.Name)
		require.Equal(t, sr.Namespace, stateResource.Namespace)
		require.Equal(t, sr.URI, stateResource.URI)

		_, err = s.GetStateResource(ctx, "doesntexist", "nope")
		require.NotNil(t, err)
		require.IsType(t, err, models.NotFound{})
	}
}

func DeleteStateResource(s store.Store, t *testing.T) func(t *testing.T) {
	return func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		sr := &models.StateResource{
			Name:      "name",
			Namespace: "namespace",
			Type:      models.StateResourceTypeActivityARN,
			URI:       "arn:activity",
		}
		require.Nil(t, s.SaveStateResource(ctx, *sr))
		stateResource, err := s.GetStateResource(ctx, sr.Name, sr.Namespace)
		require.Nil(t, err)
		require.Equal(t, sr.Name, stateResource.Name)

		require.Nil(t, s.DeleteStateResource(ctx, sr.Name, sr.Namespace))
		_, err = s.GetStateResource(ctx, sr.Name, sr.Namespace)
		require.Error(t, err)
		require.IsType(t, err, models.NotFound{})
	}
}

func SaveWorkflow(s store.Store, t *testing.T) func(t *testing.T) {
	return func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		wf := resources.KitchenSinkWorkflowDefinition(t)
		require.Nil(t, s.SaveWorkflowDefinition(ctx, *wf))
		tags := map[string]interface{}{"team": "infra", "tag2": "value2"}
		workflow := resources.NewWorkflow(wf, `["input"]`, "namespace", "queue", tags)
		require.Nil(t, s.SaveWorkflow(ctx, *workflow))
		// TODO: test behavior when workflow is invalid, e.g. breaks a length limit on a field / array
	}
}

func UpdateWorkflow(s store.Store, t *testing.T) func(t *testing.T) {
	return func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		wf := resources.KitchenSinkWorkflowDefinition(t)
		require.Nil(t, s.SaveWorkflowDefinition(ctx, *wf))
		tags := map[string]interface{}{"team": "infra", "tag2": "value2"}
		workflow := resources.NewWorkflow(wf, `["input"]`, "namespace", "queue", tags)
		require.Nil(t, s.SaveWorkflow(ctx, *workflow))

		updatedWorkflow, err := s.GetWorkflowByID(ctx, workflow.ID)
		require.Nil(t, err)
		updatedWorkflow.Status = models.WorkflowStatusSucceeded
		require.Nil(t, s.UpdateWorkflow(ctx, updatedWorkflow))

		savedWorkflow, err := s.GetWorkflowByID(ctx, workflow.ID)
		require.Nil(t, err)
		require.Equal(t, savedWorkflow.Status, models.WorkflowStatusSucceeded)
		require.WithinDuration(t, time.Time(savedWorkflow.CreatedAt), time.Now(), 1*time.Second)
		require.WithinDuration(t, time.Time(savedWorkflow.LastUpdated), time.Now(), 1*time.Second)
		require.True(t, time.Time(savedWorkflow.LastUpdated).After(time.Time(savedWorkflow.CreatedAt)))
		require.NotEqual(t, time.Time(savedWorkflow.LastUpdated), time.Time(savedWorkflow.CreatedAt))
	}
}

func UpdateLargeWorkflow(s store.Store, t *testing.T) func(t *testing.T) {
	return func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		wf := resources.KitchenSinkWorkflowDefinition(t)
		require.Nil(t, s.SaveWorkflowDefinition(ctx, *wf))
		tags := map[string]interface{}{"team": "infra", "tag2": "value2"}
		workflow := resources.NewWorkflow(wf, `["input"]`, "namespace", "queue", tags)
		require.Nil(t, s.SaveWorkflow(ctx, *workflow))

		updatedWorkflow, err := s.GetWorkflowByID(ctx, workflow.ID)
		jobs := []*models.Job{}
		for i := 0; i < 4000; i++ {
			text := fmt.Sprintf("%d-test-id", i)
			jobs = append(jobs, &models.Job{
				Container:    "test container",
				ID:           text,
				Name:         text,
				Output:       text,
				Status:       models.JobStatusCreated,
				StatusReason: text,
			})
		}
		updatedWorkflow.Jobs = jobs
		err = s.UpdateWorkflow(ctx, updatedWorkflow)
		require.Nil(t, err)

		savedWorkflow, err := s.GetWorkflowByID(ctx, workflow.ID)
		require.NoError(t, err)
		require.Equal(t, savedWorkflow.Status, updatedWorkflow.Status)
		// Large Workflows don't save Jobs in DynamoDB
		//require.Equal(t, len(savedWorkflow.Jobs), len(updatedWorkflow.Jobs))
	}
}

func DeleteWorkflow(s store.Store, t *testing.T) func(t *testing.T) {
	return func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		wf := resources.KitchenSinkWorkflowDefinition(t)
		require.Nil(t, s.SaveWorkflowDefinition(ctx, *wf))
		tags := map[string]interface{}{"team": "infra", "tag2": "value2"}
		workflow := resources.NewWorkflow(wf, `["input"]`, "namespace", "queue", tags)
		require.Nil(t, s.SaveWorkflow(ctx, *workflow))

		savedWorkflow, err := s.GetWorkflowByID(ctx, workflow.ID)
		require.Nil(t, err)
		require.Equal(t, savedWorkflow.Status, models.WorkflowStatusQueued)

		require.Nil(t, s.DeleteWorkflowByID(ctx, workflow.ID))

		savedWorkflow, err = s.GetWorkflowByID(ctx, workflow.ID)
		require.Error(t, err)
		require.IsType(t, models.NotFound{}, err)
	}
}

func GetWorkflowByID(s store.Store, t *testing.T) func(t *testing.T) {
	return func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		wf := resources.KitchenSinkWorkflowDefinition(t)
		require.Nil(t, s.SaveWorkflowDefinition(ctx, *wf))
		tags := map[string]interface{}{"team": "infra", "tag2": "value2"}
		workflow := resources.NewWorkflow(wf, `["input"]`, "namespace", "queue", tags)
		require.Nil(t, s.SaveWorkflow(ctx, *workflow))

		savedWorkflow, err := s.GetWorkflowByID(ctx, workflow.ID)
		require.Nil(t, err)

		expected := models.Workflow{
			WorkflowSummary: models.WorkflowSummary{
				ID:                 workflow.ID,
				WorkflowDefinition: wf,
				Input:              `["input"]`,
				Status:             models.WorkflowStatusQueued,
				Namespace:          "namespace",
				Queue:              "queue",
				Tags:               tags,
			},
		}
		require.Equal(t, savedWorkflow.Input, expected.Input)
		require.Equal(t, savedWorkflow.Status, expected.Status)
		require.Equal(t, savedWorkflow.Namespace, expected.Namespace)
		require.Equal(t, savedWorkflow.Queue, expected.Queue)
		require.Equal(t, savedWorkflow.Tags, expected.Tags)
		require.WithinDuration(t, time.Time(savedWorkflow.CreatedAt), time.Now(), 1*time.Second)
		require.Equal(t, time.Time(savedWorkflow.CreatedAt), time.Time(savedWorkflow.LastUpdated))
	}
}
