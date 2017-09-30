package main

import (
	"fmt"
	"log"

	oldresources "github.com/Clever/old-workflow-manager/resources"
	olddynamodb "github.com/Clever/old-workflow-manager/store/dynamodb"
	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/Clever/workflow-manager/store/dynamodb"
	"github.com/aws/aws-sdk-go/aws"
	ddb "github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	"github.com/go-openapi/strfmt"
	uuid "github.com/satori/go.uuid"
)

func oldWDToNewWD(oldWD *oldresources.WorkflowDefinition) *models.WorkflowDefinition {
	return &models.WorkflowDefinition{
		CreatedAt:    strfmt.DateTime(oldWD.CreatedAtTime),
		ID:           uuid.NewV4().String(),
		Manager:      models.Manager(string(oldWD.Manager)),
		Name:         oldWD.NameStr,
		Version:      int64(oldWD.VersionInt),
		StateMachine: stateMachineFromOldWD(oldWD),
	}
}

func stateMachineFromOldWD(oldWD *oldresources.WorkflowDefinition) *models.SLStateMachine {
	newWDStates := map[string]models.SLState{}
	for stateName, state := range oldWD.StatesMap {
		newRetry := []*models.SLRetrier{}
		for _, oldRetrier := range state.Retry() {
			if len(oldRetrier.ErrorEquals) != 1 || oldRetrier.MaxAttempts != nil {
				continue
			}
			newRetry = append(newRetry, &models.SLRetrier{
				ErrorEquals: []models.SLErrorEquals{models.SLErrorEquals(oldRetrier.ErrorEquals[0])},
				MaxAttempts: oldRetrier.MaxAttempts,
			})
		}
		newState := models.SLState{
			Type:     models.SLStateTypeTask,
			Next:     state.Next(),
			Resource: state.Resource(),
			End:      state.IsEnd(),
			Retry:    newRetry,
		}
		newWDStates[stateName] = newState
	}

	return &models.SLStateMachine{
		Comment: oldWD.Description,
		StartAt: oldWD.StartAtStr,
		States:  newWDStates,
		Version: "1.0",
	}
}

func migrateWorkflowDefinitions(ddbapi dynamodbiface.DynamoDBAPI) {
	for _, wdTable := range []string{"workflow-definitions", "latest-workflow-definitions"} {
		oldWDTable := fmt.Sprintf("%s-v2-%s", prefix, wdTable)
		newWDTable := fmt.Sprintf("%s-v3-%s", prefix, wdTable)
		totalWriteCount := 0
		if err := ddbapi.ScanPages(&ddb.ScanInput{
			ConsistentRead: aws.Bool(true),
			TableName:      aws.String(oldWDTable),
			Limit:          aws.Int64(batchWriteItemLimit),
		}, func(out *ddb.ScanOutput, lastPage bool) bool {
			writeRequests := []*ddb.WriteRequest{}
			for _, item := range out.Items {
				// decode into old resources type
				var oldWD oldresources.WorkflowDefinition
				if err := olddynamodb.DecodeWorkflowDefinition(item, &oldWD); err != nil {
					panic(err)
				}

				// convert to new models type
				newWD := oldWDToNewWD(&oldWD)

				// encode models type
				encodedWD, err := dynamodb.EncodeWorkflowDefinition(*newWD)
				if err != nil {
					panic(err)
				}

				// append a putitem request to batch
				writeRequests = append(writeRequests, &ddb.WriteRequest{
					PutRequest: &ddb.PutRequest{Item: encodedWD},
				})
			}

			if out, err := ddbapi.BatchWriteItem(&ddb.BatchWriteItemInput{
				RequestItems: map[string][]*ddb.WriteRequest{
					newWDTable: writeRequests,
				},
			}); err != nil {
				panic(err)
			} else if len(out.UnprocessedItems) > 0 {
				panic(len(out.UnprocessedItems))
			}
			totalWriteCount += len(writeRequests)
			log.Printf("wrote %d items to %s; total written: %d", len(writeRequests), newWDTable, totalWriteCount)

			return true
		}); err != nil {
			panic(err)
		}
	}
}
