package store

import (
	"testing"

	"github.com/Clever/workflow-manager/resources"
	"github.com/stretchr/testify/assert"
)

func TestUpdateWorkflow(t *testing.T) {
	t.Log("Update Workflow flow is supported")

	// create kitchensink workflow
	store := NewMemoryStore()
	err := store.CreateWorkflow(resources.KitchenSinkWorkflow(t))
	assert.Nil(t, err)

	// get kitchensink workflow
	def, err := store.LatestWorkflow("kitchensink")
	assert.Nil(t, err)
	assert.Equal(t, def.Version(), 0)
	assert.Equal(t, def.StartAt().Name(), "start-state")

	// update kitchensink workflow
	def.Description = "update the description"
	wf, err := store.UpdateWorkflow(def)
	assert.Nil(t, err)
	assert.Equal(t, wf.Version(), def.Version()+1)

	// get kitchensink version
	def, err = store.LatestWorkflow("kitchensink")
	assert.Nil(t, err)
	assert.Equal(t, def.Version(), 1)
	assert.Equal(t, def.Description, "update the description")
}
