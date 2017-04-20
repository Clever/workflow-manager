package tests

import (
	"testing"

	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/Clever/workflow-manager/resources"
	"github.com/Clever/workflow-manager/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func UpdateWorkflow(s store.Store, t *testing.T) func(t *testing.T) {
	return func(t *testing.T) {
		// create kitchensink workflow
		wf := resources.KitchenSinkWorkflow(t)
		err := s.SaveWorkflow(wf)
		require.Nil(t, err)

		// get kitchensink workflow
		def, err := s.LatestWorkflow(wf.Name())
		require.Nil(t, err)
		require.Equal(t, def.Version(), 0)
		require.NotNil(t, def.StartAt())
		require.Equal(t, def.StartAt().Name(), "start-state")

		// update kitchensink workflow
		def.Description = "update the description"
		wf, err = s.UpdateWorkflow(def)
		require.Nil(t, err)
		require.Equal(t, wf.Version(), def.Version()+1)

		// get kitchensink version
		def, err = s.LatestWorkflow(wf.Name())
		require.Nil(t, err)
		require.Equal(t, def.Version(), 1)
		require.Equal(t, def.Description, "update the description")
	}
}

func GetWorkflow(s store.Store, t *testing.T) func(t *testing.T) {
	return func(t *testing.T) {
		wf := resources.KitchenSinkWorkflow(t)
		require.Nil(t, s.SaveWorkflow(wf))
		gwf, err := s.GetWorkflow(wf.Name(), wf.Version())
		require.Nil(t, err)
		assert.Equal(t, wf.Name(), gwf.Name())
		assert.Equal(t, wf.Version(), gwf.Version())
		// TODO: deeper test of equality

		_, err = s.GetWorkflow("doesntexist", 1)
		require.NotNil(t, err)
		require.IsType(t, err, models.NotFound{})
	}
}

func SaveWorkflow(s store.Store, t *testing.T) func(t *testing.T) {
	return func(t *testing.T) {
		wf := resources.KitchenSinkWorkflow(t)
		require.Nil(t, s.SaveWorkflow(wf))

		err := s.SaveWorkflow(wf)
		require.NotNil(t, err)
		require.IsType(t, err, store.ConflictError{})

		// TODO: test behavior when workflow is invalid, e.g. breaks a length limit on a field / array
	}
}

func SaveJob(s store.Store, t *testing.T) func(t *testing.T) {
	return func(t *testing.T) {
		wf := resources.KitchenSinkWorkflow(t)
		require.Nil(t, s.SaveWorkflow(wf))
		// TODO: test behavior when workflow is invalid, e.g. breaks a length limit on a field / array
	}
}
