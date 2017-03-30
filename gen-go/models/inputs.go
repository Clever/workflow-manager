package models

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/validate"
)

// These imports may not be used depending on the input parameters
var _ = json.Marshal
var _ = fmt.Sprintf
var _ = url.QueryEscape
var _ = strconv.FormatInt
var _ = strings.Replace
var _ = validate.Maximum
var _ = strfmt.NewFormats

// HealthCheckInput holds the input parameters for a healthCheck operation.
type HealthCheckInput struct {
}

// Validate returns an error if any of the HealthCheckInput parameters don't satisfy the
// requirements from the swagger yml file.
func (i HealthCheckInput) Validate() error {
	return nil
}

// Path returns the URI path for the input.
func (i HealthCheckInput) Path() (string, error) {
	path := "/_health"
	urlVals := url.Values{}

	return path + "?" + urlVals.Encode(), nil
}

// GetJobsForWorkflowInput holds the input parameters for a getJobsForWorkflow operation.
type GetJobsForWorkflowInput struct {
	WorkflowName string
}

// ValidateGetJobsForWorkflowInput returns an error if the input parameter doesn't
// satisfy the requirements in the swagger yml file.
func ValidateGetJobsForWorkflowInput(workflowName string) error {

	return nil
}

// GetJobsForWorkflowInputPath returns the URI path for the input.
func GetJobsForWorkflowInputPath(workflowName string) (string, error) {
	path := "/jobs/{workflowName}"
	urlVals := url.Values{}

	pathworkflowName := workflowName
	if pathworkflowName == "" {
		err := fmt.Errorf("workflowName cannot be empty because it's a path parameter")
		if err != nil {
			return "", err
		}
	}
	path = strings.Replace(path, "{workflowName}", pathworkflowName, -1)

	return path + "?" + urlVals.Encode(), nil
}

// StartJobForWorkflowInput holds the input parameters for a startJobForWorkflow operation.
type StartJobForWorkflowInput struct {
	WorkflowName string
	Input        *JobInput
}

// Validate returns an error if any of the StartJobForWorkflowInput parameters don't satisfy the
// requirements from the swagger yml file.
func (i StartJobForWorkflowInput) Validate() error {

	if err := i.Input.Validate(nil); err != nil {
		return err
	}
	return nil
}

// Path returns the URI path for the input.
func (i StartJobForWorkflowInput) Path() (string, error) {
	path := "/jobs/{workflowName}"
	urlVals := url.Values{}

	pathworkflowName := i.WorkflowName
	if pathworkflowName == "" {
		err := fmt.Errorf("workflowName cannot be empty because it's a path parameter")
		if err != nil {
			return "", err
		}
	}
	path = strings.Replace(path, "{workflowName}", pathworkflowName, -1)

	return path + "?" + urlVals.Encode(), nil
}

// GetJobInput holds the input parameters for a GetJob operation.
type GetJobInput struct {
	WorkflowName string
	JobId        string
}

// Validate returns an error if any of the GetJobInput parameters don't satisfy the
// requirements from the swagger yml file.
func (i GetJobInput) Validate() error {

	return nil
}

// Path returns the URI path for the input.
func (i GetJobInput) Path() (string, error) {
	path := "/jobs/{workflowName}/{jobId}"
	urlVals := url.Values{}

	pathworkflowName := i.WorkflowName
	if pathworkflowName == "" {
		err := fmt.Errorf("workflowName cannot be empty because it's a path parameter")
		if err != nil {
			return "", err
		}
	}
	path = strings.Replace(path, "{workflowName}", pathworkflowName, -1)

	pathjobId := i.JobId
	if pathjobId == "" {
		err := fmt.Errorf("jobId cannot be empty because it's a path parameter")
		if err != nil {
			return "", err
		}
	}
	path = strings.Replace(path, "{jobId}", pathjobId, -1)

	return path + "?" + urlVals.Encode(), nil
}

// GetWorkflowByNameInput holds the input parameters for a getWorkflowByName operation.
type GetWorkflowByNameInput struct {
	Name string
}

// ValidateGetWorkflowByNameInput returns an error if the input parameter doesn't
// satisfy the requirements in the swagger yml file.
func ValidateGetWorkflowByNameInput(name string) error {

	return nil
}

// GetWorkflowByNameInputPath returns the URI path for the input.
func GetWorkflowByNameInputPath(name string) (string, error) {
	path := "/workflows/{name}"
	urlVals := url.Values{}

	pathname := name
	if pathname == "" {
		err := fmt.Errorf("name cannot be empty because it's a path parameter")
		if err != nil {
			return "", err
		}
	}
	path = strings.Replace(path, "{name}", pathname, -1)

	return path + "?" + urlVals.Encode(), nil
}

// UpdateWorkflowInput holds the input parameters for a updateWorkflow operation.
type UpdateWorkflowInput struct {
	NewWorkflowRequest *NewWorkflowRequest
	Name               string
}

// Validate returns an error if any of the UpdateWorkflowInput parameters don't satisfy the
// requirements from the swagger yml file.
func (i UpdateWorkflowInput) Validate() error {

	if err := i.NewWorkflowRequest.Validate(nil); err != nil {
		return err
	}

	return nil
}

// Path returns the URI path for the input.
func (i UpdateWorkflowInput) Path() (string, error) {
	path := "/workflows/{name}"
	urlVals := url.Values{}

	pathname := i.Name
	if pathname == "" {
		err := fmt.Errorf("name cannot be empty because it's a path parameter")
		if err != nil {
			return "", err
		}
	}
	path = strings.Replace(path, "{name}", pathname, -1)

	return path + "?" + urlVals.Encode(), nil
}
