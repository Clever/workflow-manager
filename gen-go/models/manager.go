package models

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"encoding/json"

	strfmt "github.com/go-openapi/strfmt"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/validate"
)

// Manager manager
// swagger:model Manager
type Manager string

const (
	ManagerBatch         Manager = "batch"
	ManagerStepFunctions Manager = "step-functions"
)

// for schema
var managerEnum []interface{}

func init() {
	var res []Manager
	if err := json.Unmarshal([]byte(`["batch","step-functions"]`), &res); err != nil {
		panic(err)
	}
	for _, v := range res {
		managerEnum = append(managerEnum, v)
	}
}

func (m Manager) validateManagerEnum(path, location string, value Manager) error {
	if err := validate.Enum(path, location, value, managerEnum); err != nil {
		return err
	}
	return nil
}

// Validate validates this manager
func (m Manager) Validate(formats strfmt.Registry) error {
	var res []error

	// value enum
	if err := m.validateManagerEnum("", "body", m); err != nil {
		return err
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}