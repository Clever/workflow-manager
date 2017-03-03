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
