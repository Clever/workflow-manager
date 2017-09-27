package dynamodb

import (
	"testing"

	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPageKeySerializationRoundTrip(t *testing.T) {
	pageKeyMap := map[string]interface{}{
		"name": "Usain Bolt",
		"100m": 9.58,
		"200m": 19.19,
	}
	pageKeyDynamoAttributeMap, err := dynamodbattribute.ConvertToMap(pageKeyMap)
	require.NoError(t, err)

	pageKey := NewPageKey(pageKeyDynamoAttributeMap)
	assert.Equal(t, pageKeyDynamoAttributeMap, map[string]*dynamodb.AttributeValue(*pageKey))

	pageKeyJSON, err := pageKey.ToJSON()
	require.NoError(t, err)

	pageKeyRoundTripped, err := ParsePageKey(pageKeyJSON)
	require.NoError(t, err)
	assert.Equal(t, pageKey, pageKeyRoundTripped)
}

func TestPageKeyHandlesEmptyValues(t *testing.T) {
	assert.Nil(t, NewPageKey(nil))

	parsedPageKey, err := ParsePageKey("")
	require.NoError(t, err)
	assert.Nil(t, parsedPageKey)
}
