package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	oldresources "github.com/Clever/old-workflow-manager/resources"
	olddynamodb "github.com/Clever/old-workflow-manager/store/dynamodb"
	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/Clever/workflow-manager/store/dynamodb"
	"github.com/aws/aws-sdk-go/aws"
	ddb "github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	"github.com/go-openapi/strfmt"
)

func oldWFInputToNewWFInput(oldInput []string) string {
	if len(oldInput) == 1 {
		if len(oldInput[0]) == 0 {
			return "" // [""] -> ""
		} else if oldInput[0][0] == '{' { // already JSON
			return oldInput[0]
		}
	}
	input, _ := json.Marshal(oldInput)
	return string(input)
}

func oldWFJobsToNewWFJobs(oldJobs []*oldresources.Job) []*models.Job {
	newJobs := []*models.Job{}
	for _, oldJob := range oldJobs {
		newJobs = append(newJobs, oldJobToNewJob(oldJob))
	}
	return newJobs
}

func oldStateResourceToNewStateResource(oldSR oldresources.StateResource) *models.StateResource {
	return &models.StateResource{
		LastUpdated: strfmt.DateTime(oldSR.LastUpdated),
		Name:        oldSR.Name,
		Namespace:   oldSR.Namespace,
		Type:        models.StateResourceType(string(oldSR.Type)),
		URI:         oldSR.URI,
	}
}

func oldJobToNewJob(oldJob *oldresources.Job) *models.Job {
	return &models.Job{
		Attempts:      []*models.JobAttempt{}, // don't bother
		Container:     oldJob.ContainerId,
		CreatedAt:     strfmt.DateTime(oldJob.CreatedAt),
		ID:            oldJob.ID,
		Input:         oldWFInputToNewWFInput(oldJob.Input),
		Name:          oldJob.Name,
		Output:        oldWFInputToNewWFInput(oldJob.Output),
		Queue:         oldJob.QueueName,
		StartedAt:     strfmt.DateTime(oldJob.StartedAt),
		State:         oldJob.State,
		StateResource: oldStateResourceToNewStateResource(oldJob.StateResource),
		Status:        oldJobStatusToNewJobStatus(oldJob.Status),
		StatusReason:  oldJob.StatusReason,
		StoppedAt:     strfmt.DateTime(oldJob.StoppedAt),
	}
}

func oldJobStatusToNewJobStatus(oldStatus oldresources.JobStatus) models.JobStatus {
	return models.JobStatus(strings.ToLower(string(oldStatus)))
}

func oldWFStatusToNewWFStatus(oldStatus oldresources.WorkflowStatus) models.WorkflowStatus {
	return models.WorkflowStatus(strings.ToLower(string(oldStatus)))
}

func oldWFTagsToNewWFTags(oldTags map[string]string) map[string]interface{} {
	newTags := map[string]interface{}{}
	for k, v := range oldTags {
		newTags[k] = v
	}
	return newTags
}

func migrateWorkflows(ddbapi dynamodbiface.DynamoDBAPI) {
	oldTable := fmt.Sprintf("%s-v2-workflows", prefix)
	newTable := fmt.Sprintf("%s-v3-workflows", prefix)
	oldStore := olddynamodb.New(ddbapi, olddynamodb.TableConfig{
		// unused: PrefixStateResources:      fmt.Sprintf("%s-v2", prefix),
		PrefixWorkflowDefinitions: "workflow-manager-prod-v2", // read prod WDs
		// unused: PrefixWorkflows:           fmt.Sprintf("%s-v2", prefix),
	})
	totalWriteCount := 0
	totalSkippedCount := 0
	if err := ddbapi.ScanPages(&ddb.ScanInput{
		ConsistentRead: aws.Bool(true),
		TableName:      aws.String(oldTable),
		Limit:          aws.Int64(batchWriteItemLimit),
	}, func(out *ddb.ScanOutput, lastPage bool) bool {
		writeRequests := []*ddb.WriteRequest{}
		for _, item := range out.Items {
			// decode into old resources type
			oldWF, err := olddynamodb.DecodeWorkflow(item)
			if err != nil {
				panic(err)
			}
			// populate workflow definition. if we can't, skip
			if err := oldStore.AttachWorkflowDefinition(&oldWF); err != nil {
				totalSkippedCount++
			}

			// convert to new models type
			newWD := models.Workflow{
				CreatedAt:          strfmt.DateTime(oldWF.CreatedAt),
				ID:                 oldWF.ID,
				Input:              oldWFInputToNewWFInput(oldWF.Input),
				Jobs:               oldWFJobsToNewWFJobs(oldWF.Jobs),
				LastUpdated:        strfmt.DateTime(oldWF.LastUpdated),
				Namespace:          oldWF.Namespace,
				Queue:              oldWF.Queue,
				Status:             oldWFStatusToNewWFStatus(oldWF.Status),
				Tags:               oldWFTagsToNewWFTags(oldWF.Tags),
				WorkflowDefinition: oldWDToNewWD(&oldWF.WorkflowDefinition),
			}

			// encode models type
			encodedWD, err := dynamodb.EncodeWorkflow(newWD)
			if err != nil {
				panic(err)
			}

			// append a putitem request to batch
			writeRequests = append(writeRequests, &ddb.WriteRequest{
				PutRequest: &ddb.PutRequest{Item: encodedWD},
			})
		}

		requestItems := map[string][]*ddb.WriteRequest{
			newTable: writeRequests,
		}
		for {
			if out, err := ddbapi.BatchWriteItem(&ddb.BatchWriteItemInput{
				RequestItems: requestItems,
			}); err != nil {
				panic(err)
			} else if len(out.UnprocessedItems) > 0 {
				//panic(len(out.UnprocessedItems))
				requestItems = out.UnprocessedItems
			}
			break
		}
		totalWriteCount += len(writeRequests)
		log.Printf("wrote %d items to %s; total written: %d; skipped: %d", len(writeRequests), newTable, totalWriteCount, totalSkippedCount)

		return true
	}); err != nil {
		panic(err)
	}
}
