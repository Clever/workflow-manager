package memory

import (
	"testing"

	"github.com/Clever/workflow-manager/store/tests"
)

func TestMemoryStore(t *testing.T) {
	s := New()
	t.Run("UpdateWorkflow", tests.UpdateWorkflow(s, t))
	t.Run("GetWorkflow", tests.GetWorkflow(s, t))
	t.Run("SaveWorkflow", tests.SaveWorkflow(s, t))
}
