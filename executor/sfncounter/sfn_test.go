package sfncounter

import (
	"context"
	"testing"

	"github.com/Clever/workflow-manager/mocks/mock_sfniface"
	"github.com/aws/aws-sdk-go/service/sfn"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestGetExecutionHistoryRequestCount(t *testing.T) {
	mockController := gomock.NewController(t)
	defer mockController.Finish()
	mockSFNAPI := mock_sfniface.NewMockSFNAPI(mockController)
	countedSFN := New(mockSFNAPI)
	// expect a request option to be added to the call to GetExecutionHistoryPagesWithContext
	mockSFNAPI.EXPECT().
		GetExecutionHistoryPagesWithContext(gomock.Eq(context.Background()), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).
		Times(1)
	err := countedSFN.GetExecutionHistoryPagesWithContext(context.Background(), &sfn.GetExecutionHistoryInput{}, func(*sfn.GetExecutionHistoryOutput, bool) bool {
		return true
	})
	require.Nil(t, err)
}
