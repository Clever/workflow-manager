package tests

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
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
	t.Run("UpdateLargeWorkflow", UpdateLargeWorkflow(storeFactory(), t))
	t.Run("DeleteWorkflow", DeleteWorkflow(storeFactory(), t))
	t.Run("GetWorkflowByID", GetWorkflowByID(storeFactory(), t))
	t.Run("GetWorkflows", GetWorkflows(storeFactory(), t))
	t.Run("GetWorkflowsSummaryOnly", GetWorkflowsSummaryOnly(storeFactory(), t))
	t.Run("GetWorkflowsPagination", GetWorkflowsPagination(storeFactory(), t))
	t.Run("GetPendingWorkflowIDs", GetPendingWorkflowIDs(storeFactory(), t))
	t.Run("LockWorkflow/UnlockWorkflow", LockUnlockWorkflow(storeFactory(), t))
}

func UpdateWorkflowDefinition(s store.Store, t *testing.T) func(t *testing.T) {
	return func(t *testing.T) {
		// create kitchensink workflow
		wf := resources.KitchenSinkWorkflowDefinition(t)
		require.Nil(t, s.SaveWorkflowDefinition(*wf))

		// get kitchensink workflow
		wflatest, err := s.LatestWorkflowDefinition(wf.Name)
		require.Nil(t, err)
		require.Equal(t, wflatest.Version, int64(0))
		require.Equal(t, wflatest.StateMachine.StartAt, "start-state")
		require.WithinDuration(t, time.Time(wflatest.CreatedAt), time.Now(), 1*time.Second)

		// update kitchensink workflow
		wflatest.StateMachine.Comment = "update the description"
		wfupdated, err := s.UpdateWorkflowDefinition(wflatest)
		require.Nil(t, err)
		require.Equal(t, wfupdated.StateMachine.Comment, "update the description")
		require.Equal(t, wfupdated.Version, wflatest.Version+1)
		require.WithinDuration(t, time.Time(wfupdated.CreatedAt), time.Now(), 1*time.Second)
		require.True(t, time.Time(wfupdated.CreatedAt).After(time.Time(wflatest.CreatedAt)))

		// get kitchensink workflow
		wflatest2, err := s.LatestWorkflowDefinition(wf.Name)
		require.Nil(t, err)
		require.Equal(t, wflatest2.Version, wfupdated.Version)
		require.WithinDuration(t, time.Time(wflatest2.CreatedAt), time.Now(), 1*time.Second)
		require.Equal(t, time.Time(wflatest2.CreatedAt), time.Time(wfupdated.CreatedAt))
	}
}

func GetWorkflowDefinitions(s store.Store, t *testing.T) func(t *testing.T) {
	return func(t *testing.T) {
		numWfs := 2
		for wfNum := 0; wfNum < numWfs; wfNum++ {
			wf := resources.KitchenSinkWorkflowDefinition(t)
			require.Nil(t, s.SaveWorkflowDefinition(*wf))
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
		require.Nil(t, s.SaveWorkflowDefinition(*wf))
		gwf, err := s.GetWorkflowDefinition(wf.Name, int(wf.Version))
		require.Nil(t, err)
		require.Equal(t, wf.Name, gwf.Name)
		require.Equal(t, wf.Version, gwf.Version)
		require.WithinDuration(t, time.Time(gwf.CreatedAt), time.Now(), 1*time.Second)
		// TODO: deeper test of equality

		_, err = s.GetWorkflowDefinition("doesntexist", 1)
		require.NotNil(t, err)
		require.IsType(t, err, models.NotFound{})
	}
}

func SaveWorkflowDefinition(s store.Store, t *testing.T) func(t *testing.T) {
	return func(t *testing.T) {
		wf := resources.KitchenSinkWorkflowDefinition(t)
		require.Nil(t, s.SaveWorkflowDefinition(*wf))

		err := s.SaveWorkflowDefinition(*wf)
		require.NotNil(t, err)
		require.IsType(t, err, store.ConflictError{})

		// TODO: test behavior when workflow is invalid, e.g. breaks a length limit on a field / array
	}
}

func SaveStateResource(s store.Store, t *testing.T) func(t *testing.T) {
	return func(t *testing.T) {
		sr := &models.StateResource{
			Name:      "name",
			Namespace: "namespace",
			Type:      models.StateResourceTypeActivityARN,
			URI:       "arn:activity",
		}
		require.Nil(t, s.SaveStateResource(*sr))
		stateResource, err := s.GetStateResource(sr.Name, sr.Namespace)
		require.Nil(t, err)
		require.Equal(t, sr.Name, stateResource.Name)
		require.Equal(t, sr.Namespace, stateResource.Namespace)
		require.Equal(t, sr.URI, stateResource.URI)
	}
}

func GetStateResource(s store.Store, t *testing.T) func(t *testing.T) {
	return func(t *testing.T) {
		sr := &models.StateResource{
			Name:      "name",
			Namespace: "namespace",
			Type:      models.StateResourceTypeActivityARN,
			URI:       "arn:activity",
		}
		require.Nil(t, s.SaveStateResource(*sr))
		stateResource, err := s.GetStateResource(sr.Name, sr.Namespace)
		require.Nil(t, err)
		require.Equal(t, sr.Name, stateResource.Name)
		require.Equal(t, sr.Namespace, stateResource.Namespace)
		require.Equal(t, sr.URI, stateResource.URI)

		_, err = s.GetStateResource("doesntexist", "nope")
		require.NotNil(t, err)
		require.IsType(t, err, models.NotFound{})
	}
}

func DeleteStateResource(s store.Store, t *testing.T) func(t *testing.T) {
	return func(t *testing.T) {
		sr := &models.StateResource{
			Name:      "name",
			Namespace: "namespace",
			Type:      models.StateResourceTypeActivityARN,
			URI:       "arn:activity",
		}
		require.Nil(t, s.SaveStateResource(*sr))
		stateResource, err := s.GetStateResource(sr.Name, sr.Namespace)
		require.Nil(t, err)
		require.Equal(t, sr.Name, stateResource.Name)

		require.Nil(t, s.DeleteStateResource(sr.Name, sr.Namespace))
		_, err = s.GetStateResource(sr.Name, sr.Namespace)
		require.Error(t, err)
		require.IsType(t, err, models.NotFound{})
	}
}

func SaveWorkflow(s store.Store, t *testing.T) func(t *testing.T) {
	return func(t *testing.T) {
		wf := resources.KitchenSinkWorkflowDefinition(t)
		require.Nil(t, s.SaveWorkflowDefinition(*wf))
		tags := map[string]interface{}{"team": "infra", "tag2": "value2"}
		workflow := resources.NewWorkflow(wf, `["input"]`, "namespace", "queue", tags)
		require.Nil(t, s.SaveWorkflow(*workflow))
		// TODO: test behavior when workflow is invalid, e.g. breaks a length limit on a field / array
	}
}

func UpdateWorkflow(s store.Store, t *testing.T) func(t *testing.T) {
	return func(t *testing.T) {
		wf := resources.KitchenSinkWorkflowDefinition(t)
		require.Nil(t, s.SaveWorkflowDefinition(*wf))
		tags := map[string]interface{}{"team": "infra", "tag2": "value2"}
		workflow := resources.NewWorkflow(wf, `["input"]`, "namespace", "queue", tags)
		require.Nil(t, s.SaveWorkflow(*workflow))

		updatedWorkflow, err := s.GetWorkflowByID(workflow.ID)
		require.Nil(t, err)
		updatedWorkflow.Status = models.WorkflowStatusSucceeded
		require.Nil(t, s.UpdateWorkflow(updatedWorkflow))

		savedWorkflow, err := s.GetWorkflowByID(workflow.ID)
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
		wf := resources.KitchenSinkWorkflowDefinition(t)
		require.Nil(t, s.SaveWorkflowDefinition(*wf))
		tags := map[string]interface{}{"team": "infra", "tag2": "value2"}
		workflow := resources.NewWorkflow(wf, `["input"]`, "namespace", "queue", tags)
		require.Nil(t, s.SaveWorkflow(*workflow))

		updatedWorkflow, err := s.GetWorkflowByID(workflow.ID)
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
		err = s.UpdateWorkflow(updatedWorkflow)
		require.Nil(t, err)

		savedWorkflow, err := s.GetWorkflowByID(workflow.ID)
		require.NoError(t, err)
		require.Equal(t, savedWorkflow.Status, updatedWorkflow.Status)
		// Large Workflows don't save Jobs in DynamoDB
		//require.Equal(t, len(savedWorkflow.Jobs), len(updatedWorkflow.Jobs))
	}
}

func DeleteWorkflow(s store.Store, t *testing.T) func(t *testing.T) {
	return func(t *testing.T) {
		wf := resources.KitchenSinkWorkflowDefinition(t)
		require.Nil(t, s.SaveWorkflowDefinition(*wf))
		tags := map[string]interface{}{"team": "infra", "tag2": "value2"}
		workflow := resources.NewWorkflow(wf, `["input"]`, "namespace", "queue", tags)
		require.Nil(t, s.SaveWorkflow(*workflow))

		savedWorkflow, err := s.GetWorkflowByID(workflow.ID)
		require.Nil(t, err)
		require.Equal(t, savedWorkflow.Status, models.WorkflowStatusQueued)

		require.Nil(t, s.DeleteWorkflowByID(workflow.ID))

		savedWorkflow, err = s.GetWorkflowByID(workflow.ID)
		require.Error(t, err)
		require.IsType(t, models.NotFound{}, err)
	}
}

func GetWorkflowByID(s store.Store, t *testing.T) func(t *testing.T) {
	return func(t *testing.T) {
		wf := resources.KitchenSinkWorkflowDefinition(t)
		require.Nil(t, s.SaveWorkflowDefinition(*wf))
		tags := map[string]interface{}{"team": "infra", "tag2": "value2"}
		workflow := resources.NewWorkflow(wf, `["input"]`, "namespace", "queue", tags)
		require.Nil(t, s.SaveWorkflow(*workflow))

		savedWorkflow, err := s.GetWorkflowByID(workflow.ID)
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

func GetWorkflows(s store.Store, t *testing.T) func(t *testing.T) {
	return func(t *testing.T) {
		// Set up workflows:
		definition := resources.KitchenSinkWorkflowDefinition(t)
		require.NoError(t, s.SaveWorkflowDefinition(*definition))

		tags := map[string]interface{}{"team": "infra", "tag2": "value2"}

		runningWorkflow := resources.NewWorkflow(definition, `["input for running workflow"]`, "namespace", "queue", tags)
		runningWorkflow.Status = models.WorkflowStatusRunning
		require.NoError(t, s.SaveWorkflow(*runningWorkflow))

		// TODO: what happens when we mark a running/queued/etc workflow resolved?
		runningResolvedWorkflow := resources.NewWorkflow(definition, `["input for resolved running workflow"]`, "namespace", "queue", tags)
		runningResolvedWorkflow.Status = models.WorkflowStatusRunning
		runningResolvedWorkflow.ResolvedByUser = true
		require.NoError(t, s.SaveWorkflow(*runningResolvedWorkflow))

		failedWorkflow := resources.NewWorkflow(definition, `["input for failed workflow"]`, "namespace", "queue", tags)
		failedWorkflow.Status = models.WorkflowStatusFailed
		require.NoError(t, s.SaveWorkflow(*failedWorkflow))

		failedResolvedWorkflow := resources.NewWorkflow(definition, `["input for resolved failed workflow"]`, "namespace", "queue", tags)
		failedResolvedWorkflow.Status = models.WorkflowStatusFailed
		failedResolvedWorkflow.ResolvedByUser = true
		require.NoError(t, s.SaveWorkflow(*failedResolvedWorkflow))

		// Set up workflows for a separate definition that will be ignored by the query:
		otherWorkflowDefinition := resources.KitchenSinkWorkflowDefinition(t)
		require.NoError(t, s.SaveWorkflowDefinition(*otherWorkflowDefinition))

		otherDefinitionWorkflow := resources.NewWorkflow(
			otherWorkflowDefinition, `["input"]`, "namespace", "queue", tags,
		)
		otherDefinitionWorkflow.Status = models.WorkflowStatusRunning
		require.NoError(t, s.SaveWorkflow(*otherDefinitionWorkflow))

		// Verify results for query with no status or resolvedByUser filtering:
		workflows, _, err := s.GetWorkflows(&models.WorkflowQuery{
			WorkflowDefinitionName: aws.String(definition.Name),
			Limit: 10,
		})
		require.NoError(t, err)
		require.Len(t, workflows, 4)
		// ordered by createdAt, recent first
		require.Equal(t, failedResolvedWorkflow.ID, workflows[0].ID)
		require.Equal(t, failedWorkflow.ID, workflows[1].ID)
		require.Equal(t, runningResolvedWorkflow.ID, workflows[2].ID)
		require.Equal(t, runningWorkflow.ID, workflows[3].ID)

		// Verify ResolvedByUser is ignored when not set,
		//   with explicit inclusion of ResolvedByUserWrapper false
		workflows, _, err = s.GetWorkflows(&models.WorkflowQuery{
			WorkflowDefinitionName: aws.String(definition.Name),
			ResolvedByUserWrapper: &models.ResolvedByUserWrapper{
				IsSet: false,
				Value: true,
			},
			Limit: 10,
		})
		require.NoError(t, err)
		require.Len(t, workflows, 4)
		// ordered by createdAt, recent first
		require.Equal(t, failedResolvedWorkflow.ID, workflows[0].ID)
		require.Equal(t, failedWorkflow.ID, workflows[1].ID)
		require.Equal(t, runningResolvedWorkflow.ID, workflows[2].ID)
		require.Equal(t, runningWorkflow.ID, workflows[3].ID)

		// Verify results for query with running status filtering:
		workflows, _, err = s.GetWorkflows(&models.WorkflowQuery{
			WorkflowDefinitionName: aws.String(definition.Name),
			Status:                 models.WorkflowStatusRunning,
			Limit:                  10,
		})
		require.NoError(t, err)
		require.Len(t, workflows, 2)
		// ordered by createdAt, recent first
		require.Equal(t, runningResolvedWorkflow.ID, workflows[0].ID)
		require.Equal(t, runningWorkflow.ID, workflows[1].ID)

		// Verify results for query with resolvedByUser filtering for resolved false:
		workflows, _, err = s.GetWorkflows(&models.WorkflowQuery{
			WorkflowDefinitionName: aws.String(definition.Name),
			ResolvedByUserWrapper: &models.ResolvedByUserWrapper{
				IsSet: true,
				Value: false,
			},
			Limit: 10,
		})
		require.NoError(t, err)
		require.Len(t, workflows, 2)
		// ordered by createdAt, recent first
		require.Equal(t, failedWorkflow.ID, workflows[0].ID)
		require.Equal(t, runningWorkflow.ID, workflows[1].ID)

		// Verify results for query with resolvedByUser filtering for resolved true:
		workflows, _, err = s.GetWorkflows(&models.WorkflowQuery{
			WorkflowDefinitionName: aws.String(definition.Name),
			ResolvedByUserWrapper: &models.ResolvedByUserWrapper{
				IsSet: true,
				Value: true,
			},
			Limit: 10,
		})
		require.NoError(t, err)
		require.Len(t, workflows, 2)
		// ordered by createdAt, recent first
		require.Equal(t, failedResolvedWorkflow.ID, workflows[0].ID)
		require.Equal(t, runningResolvedWorkflow.ID, workflows[1].ID)

		// Verify error for query that has both status and resolvedByUser filtering:
		workflows, _, err = s.GetWorkflows(&models.WorkflowQuery{
			WorkflowDefinitionName: aws.String(definition.Name),
			Status:                 models.WorkflowStatusRunning,
			ResolvedByUserWrapper: &models.ResolvedByUserWrapper{
				IsSet: true,
				Value: true,
			},
			Limit: 10,
		})
		require.Error(t, err)
	}
}

func GetWorkflowsSummaryOnly(s store.Store, t *testing.T) func(t *testing.T) {
	return func(t *testing.T) {
		definition := resources.KitchenSinkWorkflowDefinition(t)
		require.NoError(t, s.SaveWorkflowDefinition(*definition))

		tags := map[string]interface{}{"team": "infra", "tag2": "value2"}
		workflow := resources.NewWorkflow(definition, `["input"]`, "ns", "queue", tags)
		workflow.Jobs = []*models.Job{{
			ID:    "job1",
			Input: `["job input"]`,
		}}
		workflow.Retries = []string{"x"}
		workflow.RetryFor = "y"
		workflow.StatusReason = "test reason"
		require.NoError(t, s.SaveWorkflow(*workflow))

		// Verify details are excluded if SummaryOnly == true:
		workflows, _, err := s.GetWorkflows(&models.WorkflowQuery{
			WorkflowDefinitionName: aws.String(definition.Name),
			SummaryOnly:            aws.Bool(true),
			Limit:                  10,
		})
		require.NoError(t, err)
		require.Nil(t, workflows[0].Jobs)
		require.Empty(t, workflows[0].StatusReason)

		definitionSummary := &models.WorkflowDefinition{
			Name:    definition.Name,
			Version: definition.Version,
		}
		require.Equal(t, definitionSummary, workflows[0].WorkflowDefinition)

		// All fields of WorkflowSummary should be present
		wsType := reflect.TypeOf(models.WorkflowSummary{})
		workflowVal := reflect.ValueOf(workflows[0])
		for i := 0; i < wsType.NumField(); i++ {
			name := wsType.Field(i).Name
			assert.NotNil(t, workflowVal.FieldByName(name).String(), "Field Name: %s", name)
			assert.NotEmpty(t, workflowVal.FieldByName(name).String(), "Field Name: %s", name)
		}

		// Verify details are included if SummaryOnly == false:
		workflows, _, err = s.GetWorkflows(&models.WorkflowQuery{
			WorkflowDefinitionName: aws.String(definition.Name),
			SummaryOnly:            aws.Bool(false),
			Limit:                  10,
		})
		require.NoError(t, err)
		require.Equal(t, workflow.Jobs, workflows[0].Jobs)
		require.Equal(t, definition.Name, workflows[0].WorkflowDefinition.Name)
		require.Equal(t, definition.Version, workflows[0].WorkflowDefinition.Version)
		require.Equal(t, workflow.StatusReason, workflows[0].StatusReason)
		require.Equal(
			t,
			len(definition.StateMachine.States),
			len(workflows[0].WorkflowDefinition.StateMachine.States),
		)
	}
}

func GetWorkflowsPagination(s store.Store, t *testing.T) func(t *testing.T) {
	return func(t *testing.T) {
		definition := resources.KitchenSinkWorkflowDefinition(t)
		require.NoError(t, s.SaveWorkflowDefinition(*definition))

		workflow1 := resources.NewWorkflow(definition, `["input"]`, "namespace", "queue", map[string]interface{}{})
		workflow1.Status = models.WorkflowStatusRunning
		require.NoError(t, s.SaveWorkflow(*workflow1))

		workflow2 := resources.NewWorkflow(definition, `["input"]`, "namespace", "queue", map[string]interface{}{})
		workflow2.Status = models.WorkflowStatusSucceeded
		require.NoError(t, s.SaveWorkflow(*workflow2))

		workflow3 := resources.NewWorkflow(definition, `["input"]`, "namespace", "queue", map[string]interface{}{})
		workflow3.Status = models.WorkflowStatusRunning
		require.NoError(t, s.SaveWorkflow(*workflow3))

		limit := 1
		getAllPages := func(query models.WorkflowQuery) []models.Workflow {
			nextPageToken := ""
			nextQuery := query
			workflows := []models.Workflow{}
			workflowsPage := []models.Workflow{}
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
		query := models.WorkflowQuery{
			WorkflowDefinitionName: aws.String(definition.Name),
			Limit:  int64(limit),
			Status: models.WorkflowStatusRunning,
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
		require.Nil(t, s.SaveWorkflowDefinition(*wf1))
		wf2 := resources.KitchenSinkWorkflowDefinition(t)
		require.Nil(t, s.SaveWorkflowDefinition(*wf2))
		tags := map[string]interface{}{"team": "infra", "tag2": "value2"}
		workflow1 := resources.NewWorkflow(wf1, `["input"]`, "namespace", "queue", tags)
		require.Nil(t, s.SaveWorkflow(*workflow1))
		workflow2 := resources.NewWorkflow(wf2, `["input"]`, "namespace", "queue", tags)
		require.Nil(t, s.SaveWorkflow(*workflow2))

		pendingWorkflowIDs, err := s.GetPendingWorkflowIDs()
		require.Nil(t, err)
		require.Equal(t, pendingWorkflowIDs, []string{workflow1.ID, workflow2.ID})

		workflow1.Status = models.WorkflowStatusRunning
		require.Nil(t, s.UpdateWorkflow(*workflow1))

		pendingWorkflowIDs, err = s.GetPendingWorkflowIDs()
		require.Nil(t, err)
		require.Equal(t, pendingWorkflowIDs, []string{workflow2.ID, workflow1.ID})

		workflow2.Status = models.WorkflowStatusSucceeded
		require.Nil(t, s.UpdateWorkflow(*workflow2))

		pendingWorkflowIDs, err = s.GetPendingWorkflowIDs()
		require.Nil(t, err)
		require.Equal(t, pendingWorkflowIDs, []string{workflow1.ID})

	}
}

func LockUnlockWorkflow(s store.Store, t *testing.T) func(t *testing.T) {
	return func(t *testing.T) {
		wfd1 := resources.KitchenSinkWorkflowDefinition(t)
		require.Nil(t, s.SaveWorkflowDefinition(*wfd1))
		tags := map[string]interface{}{"team": "infra", "tag2": "value2"}
		wf1 := resources.NewWorkflow(wfd1, `["input"]`, "namespace", "queue", tags)
		require.Nil(t, s.SaveWorkflow(*wf1))

		require.Nil(t, s.LockWorkflow(wf1.ID))
		require.Equal(t, s.LockWorkflow(wf1.ID), store.ErrWorkflowLocked)
		require.Nil(t, s.UnlockWorkflow(wf1.ID))
		require.Nil(t, s.UnlockWorkflow(wf1.ID)) // unlocking an unlocked workflow is ok
		require.Nil(t, s.LockWorkflow(wf1.ID))   // can reacquire the lock
	}
}
