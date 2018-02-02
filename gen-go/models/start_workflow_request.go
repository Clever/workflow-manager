// Code generated by go-swagger; DO NOT EDIT.

package models

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	strfmt "github.com/go-openapi/strfmt"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/swag"
)

// StartWorkflowRequest start workflow request
// swagger:model StartWorkflowRequest
type StartWorkflowRequest struct {

	// input
	Input string `json:"input,omitempty"`

	// namespace
	Namespace string `json:"namespace,omitempty"`

	// queue
	Queue string `json:"queue,omitempty"`

	// tags: object with key-value pairs; keys and values should be strings
	Tags map[string]interface{} `json:"tags,omitempty"`

	// workflow definition
	WorkflowDefinition *WorkflowDefinitionRef `json:"workflowDefinition,omitempty"`
}

// Validate validates this start workflow request
func (m *StartWorkflowRequest) Validate(formats strfmt.Registry) error {
	var res []error

	if err := m.validateWorkflowDefinition(formats); err != nil {
		// prop
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (m *StartWorkflowRequest) validateWorkflowDefinition(formats strfmt.Registry) error {

	if swag.IsZero(m.WorkflowDefinition) { // not required
		return nil
	}

	if m.WorkflowDefinition != nil {

		if err := m.WorkflowDefinition.Validate(formats); err != nil {
			if ve, ok := err.(*errors.Validation); ok {
				return ve.ValidateName("workflowDefinition")
			}
			return err
		}
	}

	return nil
}

// MarshalBinary interface implementation
func (m *StartWorkflowRequest) MarshalBinary() ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	return swag.WriteJSON(m)
}

// UnmarshalBinary interface implementation
func (m *StartWorkflowRequest) UnmarshalBinary(b []byte) error {
	var res StartWorkflowRequest
	if err := swag.ReadJSON(b, &res); err != nil {
		return err
	}
	*m = res
	return nil
}
