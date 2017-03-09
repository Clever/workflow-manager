package executor

// Executor defines the interface for submitting and managing tasks
type Executor interface {
	SubmitJob(name string, definition string, dependencies []string, input string) (string, error)
}
