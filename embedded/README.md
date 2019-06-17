# Embedded workflow-manager

Embedded workflow manager implements a self-contained version of workflow-manager.
It allows applications to create a set of workflow definitions that are isolated to the application itself.
It also lets applications create workflow definitions where `Task`s are handled by functions within the application code.
It is backed by AWS Step Functions (SFN).

## Features

### Minimal dependencies

The only dependency is AWS Step Functions.

### Functions to handle states

When defining `Task` states, the only allowed `Resource` is a reference to a Go function.

### Workflow definitions in YAML

Workflow definitions are loaded from a YAML file at runtime.

### Callback states

Embedded workflow manager supports [callbacks](https://docs.aws.amazon.com/step-functions/latest/dg/connect-to-resource.html#connect-wait-token). It works by
- automatically creating a queue + dynamodb table for receving and storing task tokens.
- when defining a state machine, create an SQS callback state that contains the
  task token and a unique, application-specific key:

    ```
    Resource: arn:aws:states:::sqs:sendMessage.waitForTaskToken
    HeartbeatSeconds: 600
    Parameters:
      MessageBody:
        TaskToken.$: "$$.Task.Token"
        TaskKey: "$.TaskKey"
    ```
- use the `NewWithCallbacks` constructor to construct an embedded workflow
  manager object, which will return a `CallbackTaskFinalizer` interface for finishing
  asynchronous tasks:

    ```go
type CallbackTaskFinalizer interface {
	Failure(taskKey string, err error) error
	Success(taskKey string, result interface{}) error
}
    ```
    This interface can be passed to and used by any code that needs to finalize
    a callback task.


## Limitations

### Sync and search

Embedded workflow-manager does not sync the state of executions in Step Functions into a database of its own.
Workflows are retrieved directly from the SFN executions API, and passed through to client methods involving workflows (`GetWorkflows`, `GetWorkflowByID`, ...)  updated when they are pulled when they are retrieved via `GetWorkflowByID`.

`GetWorkflows` only supports listing workflows by workflow definition name.

### Versioning of workflow definitions

There is no versioning of workflow definitions--workflow definitions are considered unique by: (1) their name, (2) the application's name, (3) the environment the application is running in (e.g. production).


## Example

See example/ directory.
