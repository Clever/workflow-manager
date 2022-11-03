package dynamodb

import (
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

// PageKey represents a dynamodb page start key.
type PageKey map[string]*dynamodb.AttributeValue

// NewPageKey returns a new PageKey object.
func NewPageKey(pageKeyMap map[string]*dynamodb.AttributeValue) *PageKey {
	if pageKeyMap == nil {
		return nil
	}

	pageKey := PageKey(pageKeyMap)
	return &pageKey
}

// ParsePageKey parses the given JSON representation of a PageKey.
func ParsePageKey(parseKeyJSON string) (*PageKey, error) {
	if parseKeyJSON == "" {
		return nil, nil
	}

	pageKeyMap := map[string]interface{}{}
	if err := json.Unmarshal([]byte(parseKeyJSON), &pageKeyMap); err != nil {
		return nil, fmt.Errorf("unable to de-serialize dynamodb page key: %v", err)
	}

	pageKey, err := dynamodbattribute.ConvertToMap(pageKeyMap)
	if err != nil {
		return nil, fmt.Errorf("unable to de-serialize dynamodb page key: %v", err)
	}

	return NewPageKey(pageKey), nil
}

// ToJSON returns a JSON string representation of the PageKey.
func (k *PageKey) ToJSON() (string, error) {
	pageKeyMap, err := k.toMap()
	if err != nil {
		return "", fmt.Errorf("unable to serialize dynamodb page key: %v", err)
	}

	pageKeyJSONBytes, err := json.Marshal(pageKeyMap)
	if err != nil {
		return "", fmt.Errorf("unable to serialize dynamodb page key: %v", err)
	}

	return string(pageKeyJSONBytes[:]), nil
}

// toMap returns a plain golang map respresentation of the PageKey.
func (k *PageKey) toMap() (map[string]interface{}, error) {
	pageKeyMap := map[string]interface{}{}
	err := dynamodbattribute.ConvertFromMap(*k, &pageKeyMap)
	return pageKeyMap, err
}
