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

// DeleteStateResourceInput holds the input parameters for a deleteStateResource operation.
type DeleteStateResourceInput struct {
	Namespace string
	Name      string
}

// Validate returns an error if any of the DeleteStateResourceInput parameters don't satisfy the
// requirements from the swagger yml file.
func (i DeleteStateResourceInput) Validate() error {

	return nil
}

// Path returns the URI path for the input.
func (i DeleteStateResourceInput) Path() (string, error) {
	path := "/state-resources/{namespace}/{name}"
	urlVals := url.Values{}

	pathnamespace := i.Namespace
	if pathnamespace == "" {
		err := fmt.Errorf("namespace cannot be empty because it's a path parameter")
		if err != nil {
			return "", err
		}
	}
	path = strings.Replace(path, "{namespace}", pathnamespace, -1)

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

// GetStateResourceInput holds the input parameters for a getStateResource operation.
type GetStateResourceInput struct {
	Namespace string
	Name      string
}

// Validate returns an error if any of the GetStateResourceInput parameters don't satisfy the
// requirements from the swagger yml file.
func (i GetStateResourceInput) Validate() error {

	return nil
}

// Path returns the URI path for the input.
func (i GetStateResourceInput) Path() (string, error) {
	path := "/state-resources/{namespace}/{name}"
	urlVals := url.Values{}

	pathnamespace := i.Namespace
	if pathnamespace == "" {
		err := fmt.Errorf("namespace cannot be empty because it's a path parameter")
		if err != nil {
			return "", err
		}
	}
	path = strings.Replace(path, "{namespace}", pathnamespace, -1)

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

// PutStateResourceInput holds the input parameters for a putStateResource operation.
type PutStateResourceInput struct {
	Namespace        string
	Name             string
	NewStateResource *NewStateResource
}

// Validate returns an error if any of the PutStateResourceInput parameters don't satisfy the
// requirements from the swagger yml file.
func (i PutStateResourceInput) Validate() error {

	if i.NewStateResource != nil {
		if err := i.NewStateResource.Validate(nil); err != nil {
			return err
		}
	}
	return nil
}

// Path returns the URI path for the input.
func (i PutStateResourceInput) Path() (string, error) {
	path := "/state-resources/{namespace}/{name}"
	urlVals := url.Values{}

	pathnamespace := i.Namespace
	if pathnamespace == "" {
		err := fmt.Errorf("namespace cannot be empty because it's a path parameter")
		if err != nil {
			return "", err
		}
	}
	path = strings.Replace(path, "{namespace}", pathnamespace, -1)

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

// GetWorkflowDefinitionsInput holds the input parameters for a getWorkflowDefinitions operation.
type GetWorkflowDefinitionsInput struct {
}

// Validate returns an error if any of the GetWorkflowDefinitionsInput parameters don't satisfy the
// requirements from the swagger yml file.
func (i GetWorkflowDefinitionsInput) Validate() error {
	return nil
}

// Path returns the URI path for the input.
func (i GetWorkflowDefinitionsInput) Path() (string, error) {
	path := "/workflow-definitions"
	urlVals := url.Values{}

	return path + "?" + urlVals.Encode(), nil
}

// GetWorkflowDefinitionVersionsByNameInput holds the input parameters for a getWorkflowDefinitionVersionsByName operation.
type GetWorkflowDefinitionVersionsByNameInput struct {
	Name   string
	Latest *bool
}

// Validate returns an error if any of the GetWorkflowDefinitionVersionsByNameInput parameters don't satisfy the
// requirements from the swagger yml file.
func (i GetWorkflowDefinitionVersionsByNameInput) Validate() error {

	return nil
}

// Path returns the URI path for the input.
func (i GetWorkflowDefinitionVersionsByNameInput) Path() (string, error) {
	path := "/workflow-definitions/{name}"
	urlVals := url.Values{}

	pathname := i.Name
	if pathname == "" {
		err := fmt.Errorf("name cannot be empty because it's a path parameter")
		if err != nil {
			return "", err
		}
	}
	path = strings.Replace(path, "{name}", pathname, -1)

	if i.Latest != nil {
		urlVals.Add("latest", strconv.FormatBool(*i.Latest))
	}

	return path + "?" + urlVals.Encode(), nil
}

// UpdateWorkflowDefinitionInput holds the input parameters for a updateWorkflowDefinition operation.
type UpdateWorkflowDefinitionInput struct {
	NewWorkflowDefinitionRequest *NewWorkflowDefinitionRequest
	Name                         string
}

// Validate returns an error if any of the UpdateWorkflowDefinitionInput parameters don't satisfy the
// requirements from the swagger yml file.
func (i UpdateWorkflowDefinitionInput) Validate() error {

	if i.NewWorkflowDefinitionRequest != nil {
		if err := i.NewWorkflowDefinitionRequest.Validate(nil); err != nil {
			return err
		}
	}

	return nil
}

// Path returns the URI path for the input.
func (i UpdateWorkflowDefinitionInput) Path() (string, error) {
	path := "/workflow-definitions/{name}"
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

// GetWorkflowDefinitionByNameAndVersionInput holds the input parameters for a getWorkflowDefinitionByNameAndVersion operation.
type GetWorkflowDefinitionByNameAndVersionInput struct {
	Name    string
	Version int64
}

// Validate returns an error if any of the GetWorkflowDefinitionByNameAndVersionInput parameters don't satisfy the
// requirements from the swagger yml file.
func (i GetWorkflowDefinitionByNameAndVersionInput) Validate() error {

	return nil
}

// Path returns the URI path for the input.
func (i GetWorkflowDefinitionByNameAndVersionInput) Path() (string, error) {
	path := "/workflow-definitions/{name}/{version}"
	urlVals := url.Values{}

	pathname := i.Name
	if pathname == "" {
		err := fmt.Errorf("name cannot be empty because it's a path parameter")
		if err != nil {
			return "", err
		}
	}
	path = strings.Replace(path, "{name}", pathname, -1)

	pathversion := strconv.FormatInt(i.Version, 10)
	if pathversion == "" {
		err := fmt.Errorf("version cannot be empty because it's a path parameter")
		if err != nil {
			return "", err
		}
	}
	path = strings.Replace(path, "{version}", pathversion, -1)

	return path + "?" + urlVals.Encode(), nil
}

// GetWorkflowsInput holds the input parameters for a getWorkflows operation.
type GetWorkflowsInput struct {
	Limit                  *int64
	OldestFirst            *bool
	PageToken              *string
	Status                 *string
	ResolvedByUser         *bool
	SummaryOnly            *bool
	WorkflowDefinitionName string
}

// Validate returns an error if any of the GetWorkflowsInput parameters don't satisfy the
// requirements from the swagger yml file.
func (i GetWorkflowsInput) Validate() error {

	if i.Limit != nil {
		if err := validate.MaximumInt("limit", "query", *i.Limit, int64(10000), false); err != nil {
			return err
		}
	}

	return nil
}

// Path returns the URI path for the input.
func (i GetWorkflowsInput) Path() (string, error) {
	path := "/workflows"
	urlVals := url.Values{}

	if i.Limit != nil {
		urlVals.Add("limit", strconv.FormatInt(*i.Limit, 10))
	}

	if i.OldestFirst != nil {
		urlVals.Add("oldestFirst", strconv.FormatBool(*i.OldestFirst))
	}

	if i.PageToken != nil {
		urlVals.Add("pageToken", *i.PageToken)
	}

	if i.Status != nil {
		urlVals.Add("status", *i.Status)
	}

	if i.ResolvedByUser != nil {
		urlVals.Add("resolvedByUser", strconv.FormatBool(*i.ResolvedByUser))
	}

	if i.SummaryOnly != nil {
		urlVals.Add("summaryOnly", strconv.FormatBool(*i.SummaryOnly))
	}

	urlVals.Add("workflowDefinitionName", i.WorkflowDefinitionName)

	return path + "?" + urlVals.Encode(), nil
}

// CancelWorkflowInput holds the input parameters for a CancelWorkflow operation.
type CancelWorkflowInput struct {
	WorkflowID string
	Reason     *CancelReason
}

// Validate returns an error if any of the CancelWorkflowInput parameters don't satisfy the
// requirements from the swagger yml file.
func (i CancelWorkflowInput) Validate() error {

	if i.Reason != nil {
		if err := i.Reason.Validate(nil); err != nil {
			return err
		}
	}
	return nil
}

// Path returns the URI path for the input.
func (i CancelWorkflowInput) Path() (string, error) {
	path := "/workflows/{workflowID}"
	urlVals := url.Values{}

	pathworkflowID := i.WorkflowID
	if pathworkflowID == "" {
		err := fmt.Errorf("workflowID cannot be empty because it's a path parameter")
		if err != nil {
			return "", err
		}
	}
	path = strings.Replace(path, "{workflowID}", pathworkflowID, -1)

	return path + "?" + urlVals.Encode(), nil
}

// GetWorkflowByIDInput holds the input parameters for a getWorkflowByID operation.
type GetWorkflowByIDInput struct {
	WorkflowID string
}

// ValidateGetWorkflowByIDInput returns an error if the input parameter doesn't
// satisfy the requirements in the swagger yml file.
func ValidateGetWorkflowByIDInput(workflowID string) error {

	return nil
}

// GetWorkflowByIDInputPath returns the URI path for the input.
func GetWorkflowByIDInputPath(workflowID string) (string, error) {
	path := "/workflows/{workflowID}"
	urlVals := url.Values{}

	pathworkflowID := workflowID
	if pathworkflowID == "" {
		err := fmt.Errorf("workflowID cannot be empty because it's a path parameter")
		if err != nil {
			return "", err
		}
	}
	path = strings.Replace(path, "{workflowID}", pathworkflowID, -1)

	return path + "?" + urlVals.Encode(), nil
}

// ResumeWorkflowByIDInput holds the input parameters for a resumeWorkflowByID operation.
type ResumeWorkflowByIDInput struct {
	WorkflowID string
	Overrides  *WorkflowDefinitionOverrides
}

// Validate returns an error if any of the ResumeWorkflowByIDInput parameters don't satisfy the
// requirements from the swagger yml file.
func (i ResumeWorkflowByIDInput) Validate() error {

	if i.Overrides != nil {
		if err := i.Overrides.Validate(nil); err != nil {
			return err
		}
	}
	return nil
}

// Path returns the URI path for the input.
func (i ResumeWorkflowByIDInput) Path() (string, error) {
	path := "/workflows/{workflowID}"
	urlVals := url.Values{}

	pathworkflowID := i.WorkflowID
	if pathworkflowID == "" {
		err := fmt.Errorf("workflowID cannot be empty because it's a path parameter")
		if err != nil {
			return "", err
		}
	}
	path = strings.Replace(path, "{workflowID}", pathworkflowID, -1)

	return path + "?" + urlVals.Encode(), nil
}

// ResolveWorkflowByIDInput holds the input parameters for a resolveWorkflowByID operation.
type ResolveWorkflowByIDInput struct {
	WorkflowID string
}

// ValidateResolveWorkflowByIDInput returns an error if the input parameter doesn't
// satisfy the requirements in the swagger yml file.
func ValidateResolveWorkflowByIDInput(workflowID string) error {

	return nil
}

// ResolveWorkflowByIDInputPath returns the URI path for the input.
func ResolveWorkflowByIDInputPath(workflowID string) (string, error) {
	path := "/workflows/{workflowID}/resolved"
	urlVals := url.Values{}

	pathworkflowID := workflowID
	if pathworkflowID == "" {
		err := fmt.Errorf("workflowID cannot be empty because it's a path parameter")
		if err != nil {
			return "", err
		}
	}
	path = strings.Replace(path, "{workflowID}", pathworkflowID, -1)

	return path + "?" + urlVals.Encode(), nil
}
