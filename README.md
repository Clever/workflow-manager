# workflow-manager

Minimal Workflow orchestrator for AWS Batch

Owned by eng-infra

## Developing

- Update swagger.yml with your endpoints. See the [Swagger spec](http://swagger.io/specification/) for additional details on defining your swagger file.

- Run `make generate` to generate the supporting code

- Run `make build`, `make run`, or `make test` - This should fail with an error about having to implement the business logic.

- Implement aforementioned business logic so that code will build

## Deploying

```
ark start workflow-manager
```
