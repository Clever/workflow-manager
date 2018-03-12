# workflow-manager

Orchestrator for [AWS Step Functions](https://aws.amazon.com/step-functions/).

![](https://circleci.com/gh/Clever/workflow-manager/tree/master.svg?style=shield) [![GoDoc](https://godoc.org/github.com/Clever/workflow-manager?status.png)](http://godoc.org/github.com/Clever/workflow-manager)

Owned by eng-infra

## Motivation

AWS Step Functions (SFN) is a service for running state machines as defined by the [States Language Spec](https://states-language.net/spec.html).

`workflow-manager` exposes a data model and API on top of SFN that aims to hide some of the details of SFN and add features on top of what SFN provides.

## Data model

The full data model can be viewed in workflow-manager's [swagger.yml](swagger.yml).

### Workflow Definitions

Workflow definitions are [state machines](http://docs.aws.amazon.com/step-functions/latest/dg/amazon-states-language-state-machine-structure.html): they receive input and process it through a series of [states](http://docs.aws.amazon.com/step-functions/latest/dg/amazon-states-language-states.html).
Workflow definitions additionally support:
- Versioning
- Shorthand for defining the `Resource` for a [`Task`](http://docs.aws.amazon.com/step-functions/latest/dg/amazon-states-language-task-state.html) state.
  SFN requires the `Resource` field to be a full Amazon ARN.
  Workflow manager only requires the [Activity Name](http://docs.aws.amazon.com/step-functions/latest/dg/concepts-activities.html) and takes care of expanding it to the full ARN.

The full schema for workflow definitions can be found [here](docs/definitions.md#workflowdefinition).

### Workflows

A workflow is created when you run a workflow definition with a particular input.
Workflows add to the concept of [Executions](http://docs.aws.amazon.com/step-functions/latest/dg/concepts-state-machine-executions.html) in SFN by additionally supporting these parameters on submission:
- `namespace`: this parameter will be used when expanding `Resource`s in workflow definitions to their full AWS ARN.
  This allows deployment / targeting of `Resource`s in different namespaces (i.e. environments).
- `queue`: workflows can be submitted into different named queues

Workflows store all of the data surrounding the execution of a workflow definition: initial input, the data passed between states, the final output, etc.

For more information, see the [full schema definition](docs/definitions.md#workflow) and the AWS documentation for [state machine data](http://docs.aws.amazon.com/step-functions/latest/dg/concepts-state-machine-data.html).

## Development

### Overview of packages

* `main`: contains the api `handler`

* `gen-go` / `gen-js`: (Go) server and (Go/JS) client code auto-generated from the swagger.yml specification.

* `docs`: auto-generated markdown documentation from the swagger.yml definition.

* [`executor`](https://godoc.org/github.com/Clever/workflow-manager/executor): contains the main `WorkflowManager` interface for creating, stopping and updating Workflows.
  This is where interactions with the SFN API occur.

* [`resources`](https://godoc.org/github.com/Clever/workflow-manager/resources): methods for initializing and working with the auto-generated types.

* [`store`](https://godoc.org/github.com/Clever/workflow-manager/store): Workflow Manager supports persisting its data model DynamoDB or in-memory data stores.

### Running a workflow at Clever

0. Run workflow-manager on your local machine (`ark start -l`)

1. Use the existing `sfncli-test` workflow definition.
  ```
  curl -XGET http://localhost:8080/workflow-definitions/sfncli-test:master
  ```

  ```
  # sample output:
[
	{
		"createdAt": "2017-10-25T18:11:03.350Z",
		"id": "5815bfaa-8f2a-49c6-8d69-3af91c4b960d",
		"manager": "step-functions",
		"name": "sfncli-test:master",
		"stateMachine": {
			"StartAt": "echo",
			"States": {
				"echo": {
					"End": true,
					"Resource": "echo",
					"Type": "Task"
				}
			},
			"TimeoutSeconds": 60,
			"Version": "1.0"
		}
	}
]
  ```

2. The `sfncli-test` workflow definition contains a single state that references an "echo" resource.
   You can run this resource by going to the `sfncli` repo and running that locally via `ark start -l`.


3. Submit a workflow by making an API call to your locally running workflow-manager.
   This can be done via `ark`:

   ```
   WORKFLOWMANAGER_URL=http://localhost:8080 ark submit sfncli-test:master '{"foo":true}'
   ```

4. Check on the status of the workflow. This can also be done via `ark`:

   ```
   WORKFLOWMANAGER_URL=http://localhost:8080 ark workflows --workflow-id <id> --follow/full
   ```
   
   
### Updating the Data Store at Clever

- If you need to add an index to the DynamoDB store, update the DynamoDB configuration in through the `infra` repo in addition to making code changes in this repo. The list of indices can be verified in the AWS console.
- The DynamoDB store ignores `Workflow.Jobs` in case the size of the Workflow > 400KB due to DynamoDB limits.

### Updating the API

- Update swagger.yml with your endpoints. See the [Swagger spec](http://swagger.io/specification/) for additional details on defining your swagger file.
- Run `make generate` to generate the supporting code.
- Run `make build`, `make run`, or `make test` - any of these should fail with helpful errors about how to start implementing the business logic.

