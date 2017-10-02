package main

import (
	"fmt"
	"log"

	olddynamodb "github.com/Clever/old-workflow-manager/store/dynamodb"
	"github.com/Clever/workflow-manager/store/dynamodb"
	"github.com/aws/aws-sdk-go/aws"
	ddb "github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
)

func migrateStateResources(ddbapi dynamodbiface.DynamoDBAPI) {
	oldTable := fmt.Sprintf("%s-v2-state-resources", prefix)
	newTable := fmt.Sprintf("%s-v3-state-resources", prefix)
	totalWriteCount := 0
	if err := ddbapi.ScanPages(&ddb.ScanInput{
		ConsistentRead: aws.Bool(true),
		TableName:      aws.String(oldTable),
		Limit:          aws.Int64(batchWriteItemLimit),
	}, func(out *ddb.ScanOutput, lastPage bool) bool {
		writeRequests := []*ddb.WriteRequest{}
		for _, item := range out.Items {
			// decode into old resources type
			oldSR, err := olddynamodb.DecodeStateResource(item)
			if err != nil {
				panic(err)
			}

			// convert to new models type
			newSR := oldStateResourceToNewStateResource(oldSR)

			// encode models type
			encodedSR, err := dynamodb.EncodeStateResource(*newSR)
			if err != nil {
				panic(err)
			}

			// append a putitem request to batch
			writeRequests = append(writeRequests, &ddb.WriteRequest{
				PutRequest: &ddb.PutRequest{Item: encodedSR},
			})
		}

		if out, err := ddbapi.BatchWriteItem(&ddb.BatchWriteItemInput{
			RequestItems: map[string][]*ddb.WriteRequest{
				newTable: writeRequests,
			},
		}); err != nil {
			panic(err)
		} else if len(out.UnprocessedItems) > 0 {
			panic(len(out.UnprocessedItems))
		}
		totalWriteCount += len(writeRequests)
		log.Printf("wrote %d items to %s; total written: %d", len(writeRequests), newTable, totalWriteCount)

		return true
	}); err != nil {
		panic(err)
	}
}
