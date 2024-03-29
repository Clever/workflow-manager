// Code generated by go-swagger; DO NOT EDIT.

package models

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"strconv"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag"
)

// SLCatcher s l catcher
//
// swagger:model SLCatcher
type SLCatcher struct {

	// error equals
	ErrorEquals []SLErrorEquals `json:"ErrorEquals"`

	// next
	Next string `json:"Next,omitempty"`

	// result path
	ResultPath string `json:"ResultPath,omitempty"`
}

// Validate validates this s l catcher
func (m *SLCatcher) Validate(formats strfmt.Registry) error {
	var res []error

	if err := m.validateErrorEquals(formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (m *SLCatcher) validateErrorEquals(formats strfmt.Registry) error {

	if swag.IsZero(m.ErrorEquals) { // not required
		return nil
	}

	for i := 0; i < len(m.ErrorEquals); i++ {

		if err := m.ErrorEquals[i].Validate(formats); err != nil {
			if ve, ok := err.(*errors.Validation); ok {
				return ve.ValidateName("ErrorEquals" + "." + strconv.Itoa(i))
			}
			return err
		}

	}

	return nil
}

// MarshalBinary interface implementation
func (m *SLCatcher) MarshalBinary() ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	return swag.WriteJSON(m)
}

// UnmarshalBinary interface implementation
func (m *SLCatcher) UnmarshalBinary(b []byte) error {
	var res SLCatcher
	if err := swag.ReadJSON(b, &res); err != nil {
		return err
	}
	*m = res
	return nil
}
