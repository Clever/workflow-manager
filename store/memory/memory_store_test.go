package memory

import (
	"testing"

	"github.com/Clever/workflow-manager/store/tests"
)

func TestMemoryStore(t *testing.T) {
	s := New()
	// GetWorkflowDefinitions does a primitive length check of the number of workflows received
	// We should run it first for it to pass

	tests.RunStoreTests(s, t)
}
