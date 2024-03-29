// Code generated by go-swagger; DO NOT EDIT.

package models

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"strconv"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag"
	"github.com/go-openapi/validate"
)

// SLRetrier s l retrier
//
// swagger:model SLRetrier
type SLRetrier struct {

	// backoff rate
	BackoffRate float64 `json:"BackoffRate,omitempty"`

	// error equals
	ErrorEquals []SLErrorEquals `json:"ErrorEquals"`

	// interval seconds
	IntervalSeconds int64 `json:"IntervalSeconds,omitempty"`

	// max attempts
	// Maximum: 2000
	// Minimum: 0
	MaxAttempts *int64 `json:"MaxAttempts,omitempty"`
}

// Validate validates this s l retrier
func (m *SLRetrier) Validate(formats strfmt.Registry) error {
	var res []error

	if err := m.validateErrorEquals(formats); err != nil {
		res = append(res, err)
	}

	if err := m.validateMaxAttempts(formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (m *SLRetrier) validateErrorEquals(formats strfmt.Registry) error {

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

func (m *SLRetrier) validateMaxAttempts(formats strfmt.Registry) error {

	if swag.IsZero(m.MaxAttempts) { // not required
		return nil
	}

	if err := validate.MinimumInt("MaxAttempts", "body", int64(*m.MaxAttempts), 0, false); err != nil {
		return err
	}

	if err := validate.MaximumInt("MaxAttempts", "body", int64(*m.MaxAttempts), 2000, false); err != nil {
		return err
	}

	return nil
}

// MarshalBinary interface implementation
func (m *SLRetrier) MarshalBinary() ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	return swag.WriteJSON(m)
}

// UnmarshalBinary interface implementation
func (m *SLRetrier) UnmarshalBinary(b []byte) error {
	var res SLRetrier
	if err := swag.ReadJSON(b, &res); err != nil {
		return err
	}
	*m = res
	return nil
}
