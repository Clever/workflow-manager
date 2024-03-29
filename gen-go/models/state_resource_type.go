// Code generated by go-swagger; DO NOT EDIT.

package models

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"encoding/json"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/validate"
)

// StateResourceType state resource type
//
// swagger:model StateResourceType
type StateResourceType string

const (

	// StateResourceTypeJobDefinitionARN captures enum value "JobDefinitionARN"
	StateResourceTypeJobDefinitionARN StateResourceType = "JobDefinitionARN"

	// StateResourceTypeActivityARN captures enum value "ActivityARN"
	StateResourceTypeActivityARN StateResourceType = "ActivityARN"

	// StateResourceTypeLambdaFunctionARN captures enum value "LambdaFunctionARN"
	StateResourceTypeLambdaFunctionARN StateResourceType = "LambdaFunctionARN"

	// StateResourceTypeTaskARN captures enum value "TaskARN"
	StateResourceTypeTaskARN StateResourceType = "TaskARN"
)

// for schema
var stateResourceTypeEnum []interface{}

func init() {
	var res []StateResourceType
	if err := json.Unmarshal([]byte(`["JobDefinitionARN","ActivityARN","LambdaFunctionARN","TaskARN"]`), &res); err != nil {
		panic(err)
	}
	for _, v := range res {
		stateResourceTypeEnum = append(stateResourceTypeEnum, v)
	}
}

func (m StateResourceType) validateStateResourceTypeEnum(path, location string, value StateResourceType) error {
	if err := validate.Enum(path, location, value, stateResourceTypeEnum); err != nil {
		return err
	}
	return nil
}

// Validate validates this state resource type
func (m StateResourceType) Validate(formats strfmt.Registry) error {
	var res []error

	// value enum
	if err := m.validateStateResourceTypeEnum("", "body", m); err != nil {
		return err
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}
