package dynamodb

import (
	"os"
	"strings"
	"testing"

	"github.com/Clever/workflow-manager/store"
	"github.com/Clever/workflow-manager/store/tests"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
)

func TestDynamoDBStore(t *testing.T) {
	svc := dynamodb.New(session.Must(session.NewSessionWithOptions(session.Options{
		Config: aws.Config{
			Region:      aws.String("doesntmatter"),
			Endpoint:    aws.String(os.Getenv("AWS_DYNAMO_ENDPOINT")),
			Credentials: credentials.NewStaticCredentials("id", "secret", "token"),
		},
	})))

	tests.RunStoreTests(t, func() store.Store {
		prefix := "workflow-manager-test"
		listTablesOutput, err := svc.ListTables(&dynamodb.ListTablesInput{})
		if err != nil {
			t.Fatal(err)
		}
		for _, tableName := range listTablesOutput.TableNames {
			if strings.HasPrefix(*tableName, prefix) {
				svc.DeleteTable(&dynamodb.DeleteTableInput{
					TableName: tableName,
				})
			}
		}
		s := New(svc, TableConfig{
			PrefixStateResources:      prefix,
			PrefixWorkflowDefinitions: prefix,
			PrefixWorkflows:           prefix,
		})
		// InitTables(false) since dynamodb local doesn't support TTLs
		if err := s.InitTables(false); err != nil {
			t.Fatal(err)
		}
		return s
	})
}
