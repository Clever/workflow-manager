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
	t.Run("SaveJob", tests.SaveJob(s, t))
	t.Run("UpdateJob", tests.UpdateJob(s, t))
	t.Run("GetJob", tests.GetJob(s, t))
	t.Run("GetJobsForWorkflow", tests.GetJobsForWorkflow(s, t))
}
