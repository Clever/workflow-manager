// Code generated by go-swagger; DO NOT EDIT.

package models

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	strfmt "github.com/go-openapi/strfmt"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/swag"
)

// ResolvedByUserWrapper resolved by user wrapper
// swagger:model ResolvedByUserWrapper

type ResolvedByUserWrapper struct {

	// is set
	IsSet bool `json:"isSet,omitempty"`

	// value
	Value bool `json:"value,omitempty"`
}

/* polymorph ResolvedByUserWrapper isSet false */

/* polymorph ResolvedByUserWrapper value false */

// Validate validates this resolved by user wrapper
func (m *ResolvedByUserWrapper) Validate(formats strfmt.Registry) error {
	var res []error

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

// MarshalBinary interface implementation
func (m *ResolvedByUserWrapper) MarshalBinary() ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	return swag.WriteJSON(m)
}

// UnmarshalBinary interface implementation
func (m *ResolvedByUserWrapper) UnmarshalBinary(b []byte) error {
	var res ResolvedByUserWrapper
	if err := swag.ReadJSON(b, &res); err != nil {
		return err
	}
	*m = res
	return nil
}
