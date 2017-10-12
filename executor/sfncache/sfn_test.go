package sfncache

import (
	"testing"

	"github.com/Clever/workflow-manager/mocks/mock_sfniface"
	"github.com/aws/aws-sdk-go/service/sfn"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestDescribeStateMachineCache(t *testing.T) {
	mockController := gomock.NewController(t)
	defer mockController.Finish()
	expectedOutput := &sfn.DescribeStateMachineOutput{}
	mockSFNAPI := mock_sfniface.NewMockSFNAPI(mockController)
	mockSFNAPI.EXPECT().
		DescribeStateMachine(gomock.Any()).
		Return(expectedOutput, nil).
		Times(1)
	cachedSFN, err := New(mockSFNAPI)
	require.Nil(t, err)
	for i := 0; i < 1000; i++ {
		output, err := cachedSFN.DescribeStateMachine(&sfn.DescribeStateMachineInput{})
		require.Nil(t, err)
		require.Equal(t, expectedOutput, output)
	}
}
