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

// JobStatus job status
//
// swagger:model JobStatus
type JobStatus string

const (

	// JobStatusCreated captures enum value "created"
	JobStatusCreated JobStatus = "created"

	// JobStatusQueued captures enum value "queued"
	JobStatusQueued JobStatus = "queued"

	// JobStatusWaitingForDeps captures enum value "waiting_for_deps"
	JobStatusWaitingForDeps JobStatus = "waiting_for_deps"

	// JobStatusRunning captures enum value "running"
	JobStatusRunning JobStatus = "running"

	// JobStatusSucceeded captures enum value "succeeded"
	JobStatusSucceeded JobStatus = "succeeded"

	// JobStatusFailed captures enum value "failed"
	JobStatusFailed JobStatus = "failed"

	// JobStatusAbortedDepsFailed captures enum value "aborted_deps_failed"
	JobStatusAbortedDepsFailed JobStatus = "aborted_deps_failed"

	// JobStatusAbortedByUser captures enum value "aborted_by_user"
	JobStatusAbortedByUser JobStatus = "aborted_by_user"
)

// for schema
var jobStatusEnum []interface{}

func init() {
	var res []JobStatus
	if err := json.Unmarshal([]byte(`["created","queued","waiting_for_deps","running","succeeded","failed","aborted_deps_failed","aborted_by_user"]`), &res); err != nil {
		panic(err)
	}
	for _, v := range res {
		jobStatusEnum = append(jobStatusEnum, v)
	}
}

func (m JobStatus) validateJobStatusEnum(path, location string, value JobStatus) error {
	if err := validate.Enum(path, location, value, jobStatusEnum); err != nil {
		return err
	}
	return nil
}

// Validate validates this job status
func (m JobStatus) Validate(formats strfmt.Registry) error {
	var res []error

	// value enum
	if err := m.validateJobStatusEnum("", "body", m); err != nil {
		return err
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}
