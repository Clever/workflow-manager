package main

import (
	"github.com/aws/aws-sdk-go/aws/session"
	ddb "github.com/aws/aws-sdk-go/service/dynamodb"
)

const batchWriteItemLimit = int64(25)

const prefix = "workflow-manager-prod"

func main() {
	sess := session.New()
	ddbapi := ddb.New(sess)

	migrateWorkflowDefinitions(ddbapi)
	migrateWorkflows(ddbapi)
	migrateStateResources(ddbapi)
}
