package dynamodb

import (
	"os"
	"testing"

	"github.com/Clever/workflow-manager/store/tests"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
)

func TestDynamoDBStore(t *testing.T) {
	svc := dynamodb.New(session.Must(session.NewSessionWithOptions(session.Options{
		Config: aws.Config{
			//Region: aws.String("us-east-1")
			Endpoint: aws.String(os.Getenv("AWS_DYNAMO_ENDPOINT")),
		},
	})))
	s := New(svc, "test")
	if err := s.InitTables(); err != nil {
		t.Fatal(err)
	}
	t.Run("UpdateWorkflow", tests.UpdateWorkflow(s, t))
}
