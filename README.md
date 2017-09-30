# workflow-manager

Minimal Workflow orchestrator for AWS Batch and Step Functions

![](https://circleci.com/gh/Clever/workflow-manager/tree/master.svg?style=shield) [![GoDoc](https://godoc.org/github.com/Clever/workflow-manager?status.png)](http://godoc.org/github.com/Clever/workflow-manager)

Owned by eng-infra

## Development

### Overview of packages

* `main`: contains the api `handler`

* `resources`: structs for the major concepts used by the `workflow-manager`:

  * WorkflowDefinition:  describes a sequence of states with general information and instructions but without regard to specific running instances; can be reused
  * Workflow: a sequence of jobs that is kicked off with a particular input; based on the workflow's workflow definition
  * Job: a task to be run with specific resources and input as part of a kicked-off workflow; based on an individual state from the workflow definition
  * StateResource: maps information from each state in a workflow definition to more specific resources requried to actually run the corresponding job

Also contains `KitchenSink` which is a workflow definition resource that can be used by other packages in tests.

* `executor`: contains WorkflowManagers to handle jobs with AWS's Batch (`workflow_manager_batch`), Step Functions (`workflow_manager_sfn`), or a mix (`workflow_manager_multi`), as well as a wrapper around `aws-go-sdk`'s Batch API

  * Suggestions for naming and organization welcome!
  * `WorkflowManager` is the interface for creating, stopping and checking status for Workflows
  *  WorkflowManagers also converts  the resource listed for each state to details that can be consumed by the `BatchClient` (or other) backends

* `store` defines an interface for the dynamodb store and contains a very simple in-memory implementation

### Setting up a workflow with Batch

1. Setup a Batch environment in AWS with a `JobQueue` and a `ComputeEnvironment`.

2. Create one or more `JobDefinitions` via the [AWS CLI](http://docs.aws.amazon.com/cli/latest/reference/batch/index.html)
or [UI](https://console.aws.amazon.com/batch/home?region=us-east-1#/job-definitions/new) (with IAM Permissions)

3. Your app role should have the [batchcli](https://github.com/Clever/batchcli/tree/master/README.md#AWS_Policy) policy appended to it

4. Define a workflow definition json (see [example](./data/hello-workflow.post.json) with the resource as the `job-definition-name:version-number`

### Running a workflow

0. Run workflow-manager on your local machine (`ark start -l`) or on an ECS cluster (`ark start workflow-manager`).

1. Post the workflow using the API

```sh
curl -XPOST http://localhost:8080/workflow-definitions -d @data/hello-workflow.post.json
```

2. [optional]  Verify the workflow definition was created.

```sh
curl -XGET http://localhost:8080/workflow-definitions/hello-workflow
```

3. Start a workflow using this workflow definition

```sh
curl -XPOST http://localhost:8080/jobs/ -d '{
  "workflowDefinition": {
    "name": "hello-workflow",
    "version": 0
    },
    "input": [
      "arg1",
      "arg2",
      "arg3"
    ]
  }'
```

Look for the workflow ID to be displayed in the command line.

4. Monitor the status of the workflow using

```sh
watch "curl -XGET http://localhost:8080/workflows/<workflow-id>"
```

OR

```sh
watch "curl -XGET http://localhost:8080/workflows?workflowDefinitionName=hello-workflow"
```

5. Cancel a workflow

```sh
curl -XDELETE http://localhost:8080/workflows/<workflow-id> -d '{"reason": "try out cancelling a workflow"}'
```

### Updating the API

- Update swagger.yml with your endpoints. See the [Swagger spec](http://swagger.io/specification/) for additional details on defining your swagger file.
- Run `make generate` to generate the supporting code.
- Run `make build`, `make run`, or `make test` - any of these should fail with helpful errors about how to start implementing the business logic.

## Deploying

```
ark start workflow-manager
```
