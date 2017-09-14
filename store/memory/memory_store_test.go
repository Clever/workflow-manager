package memory

import (
	"testing"

	"github.com/Clever/workflow-manager/store"
	"github.com/Clever/workflow-manager/store/tests"
)

func TestMemoryStore(t *testing.T) {
	tests.RunStoreTests(t, func() store.Store { return New() })
}
