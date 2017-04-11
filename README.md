# workflow-manager

Minimal Workflow orchestrator for AWS Batch

![](https://circleci.com/gh/Clever/workflow-manager/tree/master.svg?style=shield) [![GoDoc](https://godoc.org/github.com/Clever/workflow-manager?status.png)](http://godoc.org/github.com/Clever/workflow-manager)

Owned by eng-infra

## Development

### Overview of packages

* `main`: contains the api `handler` 

* `resources`: structs for the major concepts used by the `workflow-manager` namely

  * WorkflowDefinition
  * State
  * Job
  * Task

Also contains `KitchenSink` which is a workflow resource that can be used by other packages in tests. 

* `executor`: contains JobManager and a wrapper over `aws-go-sdk`'s Batch API
  * Yes this needs better naming but this is just easier to work with now, and should be easy to refactor.
  * Suggestions for better naming and organization welcome!
  * `JobManager` currently is currently a place where Workflows are converted to tasks and vice versa.
  *  `JobManager` also converts between the resources to details that can be consumed by the `BatchClient` 

* `store` defines an interface for the store and contains a very simple in-memory implementation

### Setting up a workflow

1. Setup a Batch environment in AWS with a `JobQueue` and a `ComputeEnvironment`. 

2. Create one or more `JobDefinitions` via the [AWS CLI](http://docs.aws.amazon.com/cli/latest/reference/batch/index.html) 
or [UI](https://console.aws.amazon.com/batch/home?region=us-east-1#/job-definitions/new) (with IAM Permissions)

3. Your app role should have the [batchcli](https://github.com/Clever/batchcli/tree/master/README.md#AWS_Policy) policy appended to it

4. Define a workflow json (see [example](./data/hello-workflow.post.json) with the resource as the `job-definition-name:version-number`

### Running a job

0. Run workflow-manager on your local machine (`ark start -l`) or on an ECS cluster (`ark start workflow-manager`).

1. Post the workflow using the API

```sh
curl -XPOST http://localhost:8080/workflows -d @data/hello-workflow.post.json
```

2. [optional]  Verify the workflow was created.

```sh
curl -XGET http://localhost:8080/workflows/hello-workflow
```

3. Start a job for this workflow

```sh
curl -XPOST http://localhost:8080/jobs/ -d '{
  "workflow": {
    "name": "hello-workflow",
    "revision": 0
    },
    "data": [
      "arg1",
      "arg2",
      "arg3"
    ]
  }'
```

4. Monitor the status of the job using

```sh
watch "curl -XGET http://localhost:8080/jobs/<job-id>"
```

OR

```sh
watch "curl -XGET http://localhost:8080/jobs?workflowName=hello-workflow"
```

5. Cancel a job

```sh
curl -XDELETE http://localhost:8080/jobs/<job-id> -d '{"reason": "why cancel a job"}'
```

### Updating the API

- Update swagger.yml with your endpoints. See the [Swagger spec](http://swagger.io/specification/) for additional details on defining your swagger file. 
- Run `make generate` to generate the supporting code. 
- Run `make build`, `make run`, or `make test` - This should fail with an error about having to implement the business logic. 

## Deploying

```
ark start workflow-manager
```
