package tests

import (
	"testing"

	"github.com/Clever/workflow-manager/resources"
	"github.com/Clever/workflow-manager/store"
	"github.com/stretchr/testify/require"
)

func UpdateWorkflow(store store.Store, t *testing.T) func(t *testing.T) {
	return func(t *testing.T) {
		t.Log("Update Workflow flow is supported")

		// create kitchensink workflow
		err := store.SaveWorkflow(resources.KitchenSinkWorkflow(t))
		require.Nil(t, err)

		// get kitchensink workflow
		def, err := store.LatestWorkflow("kitchensink")
		require.Nil(t, err)
		require.Equal(t, def.Version(), 0)
		require.NotNil(t, def.StartAt())
		require.Equal(t, def.StartAt().Name(), "start-state")

		// update kitchensink workflow
		def.Description = "update the description"
		wf, err := store.UpdateWorkflow(def)
		require.Nil(t, err)
		require.Equal(t, wf.Version(), def.Version()+1)

		// get kitchensink version
		def, err = store.LatestWorkflow("kitchensink")
		require.Nil(t, err)
		require.Equal(t, def.Version(), 1)
		require.Equal(t, def.Description, "update the description")
	}
}
