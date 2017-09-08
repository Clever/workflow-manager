package models

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	strfmt "github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag"

	"github.com/go-openapi/errors"
)

// JobInput job input
// swagger:model JobInput
type JobInput struct {

	// data
	Data []string `json:"data"`

	// namespace
	Namespace string `json:"namespace,omitempty"`

	// queue
	Queue string `json:"queue,omitempty"`

	// workflow
	Workflow *WorkflowRef `json:"workflow,omitempty"`
}

// Validate validates this job input
func (m *JobInput) Validate(formats strfmt.Registry) error {
	var res []error

	if err := m.validateData(formats); err != nil {
		// prop
		res = append(res, err)
	}

	if err := m.validateWorkflow(formats); err != nil {
		// prop
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (m *JobInput) validateData(formats strfmt.Registry) error {

	if swag.IsZero(m.Data) { // not required
		return nil
	}

	return nil
}

func (m *JobInput) validateWorkflow(formats strfmt.Registry) error {

	if swag.IsZero(m.Workflow) { // not required
		return nil
	}

	if m.Workflow != nil {

		if err := m.Workflow.Validate(formats); err != nil {
			return err
		}
	}

	return nil
}
