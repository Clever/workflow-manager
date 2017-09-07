package tests

import (
	"testing"
	"time"

	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/Clever/workflow-manager/resources"
	"github.com/Clever/workflow-manager/store"
	"github.com/stretchr/testify/require"
)

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

func SaveJob(s store.Store, t *testing.T) func(t *testing.T) {
	return func(t *testing.T) {
		wf := resources.KitchenSinkWorkflowDefinition(t)
		require.Nil(t, s.SaveWorkflowDefinition(wf))
		job := resources.NewJob(wf, []string{"input"})
		require.Nil(t, s.SaveJob(*job))
		// TODO: test behavior when workflow is invalid, e.g. breaks a length limit on a field / array
	}
}

func UpdateJob(s store.Store, t *testing.T) func(t *testing.T) {
	return func(t *testing.T) {
		wf := resources.KitchenSinkWorkflowDefinition(t)
		require.Nil(t, s.SaveWorkflowDefinition(wf))
		job := resources.NewJob(wf, []string{"input"})
		require.Nil(t, s.SaveJob(*job))

		updatedJob, err := s.GetJob(job.ID)
		require.Nil(t, err)
		updatedJob.Status = resources.Succeeded
		require.Nil(t, s.UpdateJob(updatedJob))

		savedJob, err := s.GetJob(job.ID)
		require.Nil(t, err)
		require.Equal(t, savedJob.Status, resources.Succeeded)
		require.WithinDuration(t, savedJob.CreatedAt, time.Now(), 1*time.Second)
		require.WithinDuration(t, savedJob.LastUpdated, time.Now(), 1*time.Second)
		require.True(t, savedJob.LastUpdated.After(savedJob.CreatedAt))
		require.NotEqual(t, savedJob.LastUpdated, savedJob.CreatedAt)
	}
}

func GetJob(s store.Store, t *testing.T) func(t *testing.T) {
	return func(t *testing.T) {
		wf := resources.KitchenSinkWorkflowDefinition(t)
		require.Nil(t, s.SaveWorkflowDefinition(wf))
		job := resources.NewJob(wf, []string{"input"})
		require.Nil(t, s.SaveJob(*job))

		savedJob, err := s.GetJob(job.ID)
		require.Nil(t, err)
		require.WithinDuration(t, savedJob.CreatedAt, time.Now(), 1*time.Second)
		require.Equal(t, savedJob.CreatedAt, savedJob.LastUpdated)
	}
}

func GetJobsForWorkflowDefinition(s store.Store, t *testing.T) func(t *testing.T) {
	return func(t *testing.T) {
		wf1 := resources.KitchenSinkWorkflowDefinition(t)
		require.Nil(t, s.SaveWorkflowDefinition(wf1))
		wf2 := resources.KitchenSinkWorkflowDefinition(t)
		require.Nil(t, s.SaveWorkflowDefinition(wf2))
		var job1IDs, job2IDs []string
		for len(job1IDs) < 2 {
			job1 := resources.NewJob(wf1, []string{"input"})
			job1IDs = append([]string{job1.ID}, job1IDs...) // newest first
			require.Nil(t, s.SaveJob(*job1))

			job2 := resources.NewJob(wf2, []string{"input"})
			job2IDs = append([]string{job2.ID}, job2IDs...)
			require.Nil(t, s.SaveJob(*job2))
		}

		jobs, err := s.GetJobsForWorkflowDefinition(wf2.Name())
		require.Nil(t, err)
		require.Equal(t, len(jobs), len(job2IDs))
		var gotJob2IDs []string
		for _, j := range jobs {
			gotJob2IDs = append(gotJob2IDs, j.ID)
		}
		require.Equal(t, job2IDs, gotJob2IDs)
	}
}
