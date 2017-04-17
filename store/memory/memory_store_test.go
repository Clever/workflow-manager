package memory

import (
	"testing"

	"github.com/Clever/workflow-manager/store/tests"
)

func TestMemoryStore(t *testing.T) {
	store := New()
	t.Run("UpdateWorkflow", tests.UpdateWorkflow(store, t))
}
