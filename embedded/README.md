# Embedded workflow-manager

Embedded workflow manager implements a self-contained version of workflow-manager.
It allows applications to create a set of workflow definitions specific that are isolated to the application itself.
It also lets applications create workflow definitions where `Task`s are handled by functions within the application code.
It is backed by AWS Step Functions (SFN).

## Features

### Minimal dependencies

The only dependency is AWS Step Functions.

### Functions to handle states

When defining `Task` states, the only allowed `Resource` is a reference to a Go function.

### Workflow definitions in YAML

Workflow definitions are loaded from a YAML file at runtime.

## Limitations

### Sync and search

Embedded workflow-manager does not sync the state of executions in Step Functions into a database of its own.
Workflows are retrieved directly from the SFN executions API, and passed through to client methods involving workflows (`GetWorkflows`, `GetWorkflowByID`, ...)  updated when they are pulled when they are retrieved via `GetWorkflowByID`.

`GetWorkflows` only supports listing workflows by workflow definition name.

### Versioning of workflow definitions

There is no versioning of workflow definitions--workflow definitions are considered unique by: (1) their name, (2) the application's name, (3) the environment the application is running in (e.g. production).


## Example

See example/ directory.
