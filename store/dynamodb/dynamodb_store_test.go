package dynamodb

import (
	"os"
	"testing"

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
	s := New(svc, "test")
	if err := s.InitTables(); err != nil {
		t.Fatal(err)
	}
	t.Run("GetWorkflowDefinitions", tests.GetWorkflowDefinitions(s, t))
	t.Run("UpdateWorkflowDefinition", tests.UpdateWorkflowDefinition(s, t))
	t.Run("GetWorkflowDefinition", tests.GetWorkflowDefinition(s, t))
	t.Run("SaveWorkflowDefinition", tests.SaveWorkflowDefinition(s, t))
	t.Run("SaveStateResource", tests.SaveStateResource(s, t))
	t.Run("GetStateResource", tests.GetStateResource(s, t))
	t.Run("DeleteStateResource", tests.DeleteStateResource(s, t))
	t.Run("SaveJob", tests.SaveJob(s, t))
	t.Run("UpdateJob", tests.UpdateJob(s, t))
	t.Run("GetJob", tests.GetJob(s, t))
	t.Run("GetJobsForWorkflowDefinition", tests.GetJobsForWorkflowDefinition(s, t))
}
