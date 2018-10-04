package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/go-errors/errors"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag"
	"github.com/gorilla/mux"
	"gopkg.in/Clever/kayvee-go.v6/logger"
)

var _ = strconv.ParseInt
var _ = strfmt.Default
var _ = swag.ConvertInt32
var _ = errors.New
var _ = mux.Vars
var _ = bytes.Compare
var _ = ioutil.ReadAll

var formats = strfmt.Default
var _ = formats

// convertBase64 takes in a string and returns a strfmt.Base64 if the input
// is valid base64 and an error otherwise.
func convertBase64(input string) (strfmt.Base64, error) {
	temp, err := formats.Parse("byte", input)
	if err != nil {
		return strfmt.Base64{}, err
	}
	return *temp.(*strfmt.Base64), nil
}

// convertDateTime takes in a string and returns a strfmt.DateTime if the input
// is a valid DateTime and an error otherwise.
func convertDateTime(input string) (strfmt.DateTime, error) {
	temp, err := formats.Parse("date-time", input)
	if err != nil {
		return strfmt.DateTime{}, err
	}
	return *temp.(*strfmt.DateTime), nil
}

// convertDate takes in a string and returns a strfmt.Date if the input
// is a valid Date and an error otherwise.
func convertDate(input string) (strfmt.Date, error) {
	temp, err := formats.Parse("date", input)
	if err != nil {
		return strfmt.Date{}, err
	}
	return *temp.(*strfmt.Date), nil
}

func jsonMarshalNoError(i interface{}) string {
	bytes, err := json.MarshalIndent(i, "", "\t")
	if err != nil {
		// This should never happen
		return ""
	}
	return string(bytes)
}

// statusCodeForHealthCheck returns the status code corresponding to the returned
// object. It returns -1 if the type doesn't correspond to anything.
func statusCodeForHealthCheck(obj interface{}) int {

	switch obj.(type) {

	case *models.BadRequest:
		return 400

	case *models.InternalError:
		return 500

	case models.BadRequest:
		return 400

	case models.InternalError:
		return 500

	default:
		return -1
	}
}

func (h handler) HealthCheckHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {

	err := h.HealthCheck(ctx)

	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		if btErr, ok := err.(*errors.Error); ok {
			logger.FromContext(ctx).AddContext("stacktrace", string(btErr.Stack()))
		}
		statusCode := statusCodeForHealthCheck(err)
		if statusCode == -1 {
			err = models.InternalError{Message: err.Error()}
			statusCode = 500
		}
		http.Error(w, jsonMarshalNoError(err), statusCode)
		return
	}

	w.WriteHeader(200)
	w.Write([]byte(""))

}

// newHealthCheckInput takes in an http.Request an returns the input struct.
func newHealthCheckInput(r *http.Request) (*models.HealthCheckInput, error) {
	var input models.HealthCheckInput

	var err error
	_ = err

	return &input, nil
}

// statusCodeForPostStateResource returns the status code corresponding to the returned
// object. It returns -1 if the type doesn't correspond to anything.
func statusCodeForPostStateResource(obj interface{}) int {

	switch obj.(type) {

	case *models.BadRequest:
		return 400

	case *models.InternalError:
		return 500

	case *models.StateResource:
		return 201

	case models.BadRequest:
		return 400

	case models.InternalError:
		return 500

	case models.StateResource:
		return 201

	default:
		return -1
	}
}

func (h handler) PostStateResourceHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {

	input, err := newPostStateResourceInput(r)
	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		http.Error(w, jsonMarshalNoError(models.BadRequest{Message: err.Error()}), http.StatusBadRequest)
		return
	}

	if input != nil {
		err = input.Validate(nil)
	}

	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		http.Error(w, jsonMarshalNoError(models.BadRequest{Message: err.Error()}), http.StatusBadRequest)
		return
	}

	resp, err := h.PostStateResource(ctx, input)

	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		if btErr, ok := err.(*errors.Error); ok {
			logger.FromContext(ctx).AddContext("stacktrace", string(btErr.Stack()))
		}
		statusCode := statusCodeForPostStateResource(err)
		if statusCode == -1 {
			err = models.InternalError{Message: err.Error()}
			statusCode = 500
		}
		http.Error(w, jsonMarshalNoError(err), statusCode)
		return
	}

	respBytes, err := json.MarshalIndent(resp, "", "\t")
	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		http.Error(w, jsonMarshalNoError(models.InternalError{Message: err.Error()}), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCodeForPostStateResource(resp))
	w.Write(respBytes)

}

// newPostStateResourceInput takes in an http.Request an returns the input struct.
func newPostStateResourceInput(r *http.Request) (*models.NewStateResource, error) {
	var err error
	_ = err

	data, err := ioutil.ReadAll(r.Body)

	if len(data) > 0 {

		var input models.NewStateResource
		if err := json.NewDecoder(bytes.NewReader(data)).Decode(&input); err != nil {
			return nil, err
		}
		return &input, nil

	}

	return nil, nil
}

// statusCodeForDeleteStateResource returns the status code corresponding to the returned
// object. It returns -1 if the type doesn't correspond to anything.
func statusCodeForDeleteStateResource(obj interface{}) int {

	switch obj.(type) {

	case *models.BadRequest:
		return 400

	case *models.InternalError:
		return 500

	case *models.NotFound:
		return 404

	case models.BadRequest:
		return 400

	case models.InternalError:
		return 500

	case models.NotFound:
		return 404

	default:
		return -1
	}
}

func (h handler) DeleteStateResourceHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {

	input, err := newDeleteStateResourceInput(r)
	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		http.Error(w, jsonMarshalNoError(models.BadRequest{Message: err.Error()}), http.StatusBadRequest)
		return
	}

	err = input.Validate()

	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		http.Error(w, jsonMarshalNoError(models.BadRequest{Message: err.Error()}), http.StatusBadRequest)
		return
	}

	err = h.DeleteStateResource(ctx, input)

	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		if btErr, ok := err.(*errors.Error); ok {
			logger.FromContext(ctx).AddContext("stacktrace", string(btErr.Stack()))
		}
		statusCode := statusCodeForDeleteStateResource(err)
		if statusCode == -1 {
			err = models.InternalError{Message: err.Error()}
			statusCode = 500
		}
		http.Error(w, jsonMarshalNoError(err), statusCode)
		return
	}

	w.WriteHeader(200)
	w.Write([]byte(""))

}

// newDeleteStateResourceInput takes in an http.Request an returns the input struct.
func newDeleteStateResourceInput(r *http.Request) (*models.DeleteStateResourceInput, error) {
	var input models.DeleteStateResourceInput

	var err error
	_ = err

	namespaceStr := mux.Vars(r)["namespace"]
	if len(namespaceStr) == 0 {
		return nil, errors.New("path parameter 'namespace' must be specified")
	}
	namespaceStrs := []string{namespaceStr}

	if len(namespaceStrs) > 0 {
		var namespaceTmp string
		namespaceStr := namespaceStrs[0]
		namespaceTmp, err = namespaceStr, error(nil)
		if err != nil {
			return nil, err
		}
		input.Namespace = namespaceTmp
	}

	nameStr := mux.Vars(r)["name"]
	if len(nameStr) == 0 {
		return nil, errors.New("path parameter 'name' must be specified")
	}
	nameStrs := []string{nameStr}

	if len(nameStrs) > 0 {
		var nameTmp string
		nameStr := nameStrs[0]
		nameTmp, err = nameStr, error(nil)
		if err != nil {
			return nil, err
		}
		input.Name = nameTmp
	}

	return &input, nil
}

// statusCodeForGetStateResource returns the status code corresponding to the returned
// object. It returns -1 if the type doesn't correspond to anything.
func statusCodeForGetStateResource(obj interface{}) int {

	switch obj.(type) {

	case *models.BadRequest:
		return 400

	case *models.InternalError:
		return 500

	case *models.NotFound:
		return 404

	case *models.StateResource:
		return 200

	case models.BadRequest:
		return 400

	case models.InternalError:
		return 500

	case models.NotFound:
		return 404

	case models.StateResource:
		return 200

	default:
		return -1
	}
}

func (h handler) GetStateResourceHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {

	input, err := newGetStateResourceInput(r)
	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		http.Error(w, jsonMarshalNoError(models.BadRequest{Message: err.Error()}), http.StatusBadRequest)
		return
	}

	err = input.Validate()

	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		http.Error(w, jsonMarshalNoError(models.BadRequest{Message: err.Error()}), http.StatusBadRequest)
		return
	}

	resp, err := h.GetStateResource(ctx, input)

	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		if btErr, ok := err.(*errors.Error); ok {
			logger.FromContext(ctx).AddContext("stacktrace", string(btErr.Stack()))
		}
		statusCode := statusCodeForGetStateResource(err)
		if statusCode == -1 {
			err = models.InternalError{Message: err.Error()}
			statusCode = 500
		}
		http.Error(w, jsonMarshalNoError(err), statusCode)
		return
	}

	respBytes, err := json.MarshalIndent(resp, "", "\t")
	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		http.Error(w, jsonMarshalNoError(models.InternalError{Message: err.Error()}), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCodeForGetStateResource(resp))
	w.Write(respBytes)

}

// newGetStateResourceInput takes in an http.Request an returns the input struct.
func newGetStateResourceInput(r *http.Request) (*models.GetStateResourceInput, error) {
	var input models.GetStateResourceInput

	var err error
	_ = err

	namespaceStr := mux.Vars(r)["namespace"]
	if len(namespaceStr) == 0 {
		return nil, errors.New("path parameter 'namespace' must be specified")
	}
	namespaceStrs := []string{namespaceStr}

	if len(namespaceStrs) > 0 {
		var namespaceTmp string
		namespaceStr := namespaceStrs[0]
		namespaceTmp, err = namespaceStr, error(nil)
		if err != nil {
			return nil, err
		}
		input.Namespace = namespaceTmp
	}

	nameStr := mux.Vars(r)["name"]
	if len(nameStr) == 0 {
		return nil, errors.New("path parameter 'name' must be specified")
	}
	nameStrs := []string{nameStr}

	if len(nameStrs) > 0 {
		var nameTmp string
		nameStr := nameStrs[0]
		nameTmp, err = nameStr, error(nil)
		if err != nil {
			return nil, err
		}
		input.Name = nameTmp
	}

	return &input, nil
}

// statusCodeForPutStateResource returns the status code corresponding to the returned
// object. It returns -1 if the type doesn't correspond to anything.
func statusCodeForPutStateResource(obj interface{}) int {

	switch obj.(type) {

	case *models.BadRequest:
		return 400

	case *models.InternalError:
		return 500

	case *models.StateResource:
		return 201

	case models.BadRequest:
		return 400

	case models.InternalError:
		return 500

	case models.StateResource:
		return 201

	default:
		return -1
	}
}

func (h handler) PutStateResourceHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {

	input, err := newPutStateResourceInput(r)
	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		http.Error(w, jsonMarshalNoError(models.BadRequest{Message: err.Error()}), http.StatusBadRequest)
		return
	}

	err = input.Validate()

	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		http.Error(w, jsonMarshalNoError(models.BadRequest{Message: err.Error()}), http.StatusBadRequest)
		return
	}

	resp, err := h.PutStateResource(ctx, input)

	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		if btErr, ok := err.(*errors.Error); ok {
			logger.FromContext(ctx).AddContext("stacktrace", string(btErr.Stack()))
		}
		statusCode := statusCodeForPutStateResource(err)
		if statusCode == -1 {
			err = models.InternalError{Message: err.Error()}
			statusCode = 500
		}
		http.Error(w, jsonMarshalNoError(err), statusCode)
		return
	}

	respBytes, err := json.MarshalIndent(resp, "", "\t")
	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		http.Error(w, jsonMarshalNoError(models.InternalError{Message: err.Error()}), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCodeForPutStateResource(resp))
	w.Write(respBytes)

}

// newPutStateResourceInput takes in an http.Request an returns the input struct.
func newPutStateResourceInput(r *http.Request) (*models.PutStateResourceInput, error) {
	var input models.PutStateResourceInput

	var err error
	_ = err

	namespaceStr := mux.Vars(r)["namespace"]
	if len(namespaceStr) == 0 {
		return nil, errors.New("path parameter 'namespace' must be specified")
	}
	namespaceStrs := []string{namespaceStr}

	if len(namespaceStrs) > 0 {
		var namespaceTmp string
		namespaceStr := namespaceStrs[0]
		namespaceTmp, err = namespaceStr, error(nil)
		if err != nil {
			return nil, err
		}
		input.Namespace = namespaceTmp
	}

	nameStr := mux.Vars(r)["name"]
	if len(nameStr) == 0 {
		return nil, errors.New("path parameter 'name' must be specified")
	}
	nameStrs := []string{nameStr}

	if len(nameStrs) > 0 {
		var nameTmp string
		nameStr := nameStrs[0]
		nameTmp, err = nameStr, error(nil)
		if err != nil {
			return nil, err
		}
		input.Name = nameTmp
	}

	data, err := ioutil.ReadAll(r.Body)

	if len(data) > 0 {

		input.NewStateResource = &models.NewStateResource{}
		if err := json.NewDecoder(bytes.NewReader(data)).Decode(input.NewStateResource); err != nil {
			return nil, err
		}

	}

	return &input, nil
}

// statusCodeForGetWorkflowDefinitions returns the status code corresponding to the returned
// object. It returns -1 if the type doesn't correspond to anything.
func statusCodeForGetWorkflowDefinitions(obj interface{}) int {

	switch obj.(type) {

	case *[]models.WorkflowDefinition:
		return 200

	case *models.BadRequest:
		return 400

	case *models.InternalError:
		return 500

	case []models.WorkflowDefinition:
		return 200

	case models.BadRequest:
		return 400

	case models.InternalError:
		return 500

	default:
		return -1
	}
}

func (h handler) GetWorkflowDefinitionsHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {

	resp, err := h.GetWorkflowDefinitions(ctx)

	// Success types that return an array should never return nil so let's make this easier
	// for consumers by converting nil arrays to empty arrays
	if resp == nil {
		resp = []models.WorkflowDefinition{}
	}

	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		if btErr, ok := err.(*errors.Error); ok {
			logger.FromContext(ctx).AddContext("stacktrace", string(btErr.Stack()))
		}
		statusCode := statusCodeForGetWorkflowDefinitions(err)
		if statusCode == -1 {
			err = models.InternalError{Message: err.Error()}
			statusCode = 500
		}
		http.Error(w, jsonMarshalNoError(err), statusCode)
		return
	}

	respBytes, err := json.MarshalIndent(resp, "", "\t")
	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		http.Error(w, jsonMarshalNoError(models.InternalError{Message: err.Error()}), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCodeForGetWorkflowDefinitions(resp))
	w.Write(respBytes)

}

// newGetWorkflowDefinitionsInput takes in an http.Request an returns the input struct.
func newGetWorkflowDefinitionsInput(r *http.Request) (*models.GetWorkflowDefinitionsInput, error) {
	var input models.GetWorkflowDefinitionsInput

	var err error
	_ = err

	return &input, nil
}

// statusCodeForNewWorkflowDefinition returns the status code corresponding to the returned
// object. It returns -1 if the type doesn't correspond to anything.
func statusCodeForNewWorkflowDefinition(obj interface{}) int {

	switch obj.(type) {

	case *models.BadRequest:
		return 400

	case *models.InternalError:
		return 500

	case *models.WorkflowDefinition:
		return 201

	case models.BadRequest:
		return 400

	case models.InternalError:
		return 500

	case models.WorkflowDefinition:
		return 201

	default:
		return -1
	}
}

func (h handler) NewWorkflowDefinitionHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {

	input, err := newNewWorkflowDefinitionInput(r)
	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		http.Error(w, jsonMarshalNoError(models.BadRequest{Message: err.Error()}), http.StatusBadRequest)
		return
	}

	if input != nil {
		err = input.Validate(nil)
	}

	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		http.Error(w, jsonMarshalNoError(models.BadRequest{Message: err.Error()}), http.StatusBadRequest)
		return
	}

	resp, err := h.NewWorkflowDefinition(ctx, input)

	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		if btErr, ok := err.(*errors.Error); ok {
			logger.FromContext(ctx).AddContext("stacktrace", string(btErr.Stack()))
		}
		statusCode := statusCodeForNewWorkflowDefinition(err)
		if statusCode == -1 {
			err = models.InternalError{Message: err.Error()}
			statusCode = 500
		}
		http.Error(w, jsonMarshalNoError(err), statusCode)
		return
	}

	respBytes, err := json.MarshalIndent(resp, "", "\t")
	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		http.Error(w, jsonMarshalNoError(models.InternalError{Message: err.Error()}), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCodeForNewWorkflowDefinition(resp))
	w.Write(respBytes)

}

// newNewWorkflowDefinitionInput takes in an http.Request an returns the input struct.
func newNewWorkflowDefinitionInput(r *http.Request) (*models.NewWorkflowDefinitionRequest, error) {
	var err error
	_ = err

	data, err := ioutil.ReadAll(r.Body)

	if len(data) > 0 {

		var input models.NewWorkflowDefinitionRequest
		if err := json.NewDecoder(bytes.NewReader(data)).Decode(&input); err != nil {
			return nil, err
		}
		return &input, nil

	}

	return nil, nil
}

// statusCodeForGetWorkflowDefinitionVersionsByName returns the status code corresponding to the returned
// object. It returns -1 if the type doesn't correspond to anything.
func statusCodeForGetWorkflowDefinitionVersionsByName(obj interface{}) int {

	switch obj.(type) {

	case *[]models.WorkflowDefinition:
		return 200

	case *models.BadRequest:
		return 400

	case *models.InternalError:
		return 500

	case *models.NotFound:
		return 404

	case []models.WorkflowDefinition:
		return 200

	case models.BadRequest:
		return 400

	case models.InternalError:
		return 500

	case models.NotFound:
		return 404

	default:
		return -1
	}
}

func (h handler) GetWorkflowDefinitionVersionsByNameHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {

	input, err := newGetWorkflowDefinitionVersionsByNameInput(r)
	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		http.Error(w, jsonMarshalNoError(models.BadRequest{Message: err.Error()}), http.StatusBadRequest)
		return
	}

	err = input.Validate()

	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		http.Error(w, jsonMarshalNoError(models.BadRequest{Message: err.Error()}), http.StatusBadRequest)
		return
	}

	resp, err := h.GetWorkflowDefinitionVersionsByName(ctx, input)

	// Success types that return an array should never return nil so let's make this easier
	// for consumers by converting nil arrays to empty arrays
	if resp == nil {
		resp = []models.WorkflowDefinition{}
	}

	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		if btErr, ok := err.(*errors.Error); ok {
			logger.FromContext(ctx).AddContext("stacktrace", string(btErr.Stack()))
		}
		statusCode := statusCodeForGetWorkflowDefinitionVersionsByName(err)
		if statusCode == -1 {
			err = models.InternalError{Message: err.Error()}
			statusCode = 500
		}
		http.Error(w, jsonMarshalNoError(err), statusCode)
		return
	}

	respBytes, err := json.MarshalIndent(resp, "", "\t")
	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		http.Error(w, jsonMarshalNoError(models.InternalError{Message: err.Error()}), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCodeForGetWorkflowDefinitionVersionsByName(resp))
	w.Write(respBytes)

}

// newGetWorkflowDefinitionVersionsByNameInput takes in an http.Request an returns the input struct.
func newGetWorkflowDefinitionVersionsByNameInput(r *http.Request) (*models.GetWorkflowDefinitionVersionsByNameInput, error) {
	var input models.GetWorkflowDefinitionVersionsByNameInput

	var err error
	_ = err

	nameStr := mux.Vars(r)["name"]
	if len(nameStr) == 0 {
		return nil, errors.New("path parameter 'name' must be specified")
	}
	nameStrs := []string{nameStr}

	if len(nameStrs) > 0 {
		var nameTmp string
		nameStr := nameStrs[0]
		nameTmp, err = nameStr, error(nil)
		if err != nil {
			return nil, err
		}
		input.Name = nameTmp
	}

	latestStrs := r.URL.Query()["latest"]

	if len(latestStrs) == 0 {
		latestStrs = []string{"true"}
	}
	if len(latestStrs) > 0 {
		var latestTmp bool
		latestStr := latestStrs[0]
		latestTmp, err = strconv.ParseBool(latestStr)
		if err != nil {
			return nil, err
		}
		input.Latest = &latestTmp
	}

	return &input, nil
}

// statusCodeForUpdateWorkflowDefinition returns the status code corresponding to the returned
// object. It returns -1 if the type doesn't correspond to anything.
func statusCodeForUpdateWorkflowDefinition(obj interface{}) int {

	switch obj.(type) {

	case *models.BadRequest:
		return 400

	case *models.InternalError:
		return 500

	case *models.NotFound:
		return 404

	case *models.WorkflowDefinition:
		return 201

	case models.BadRequest:
		return 400

	case models.InternalError:
		return 500

	case models.NotFound:
		return 404

	case models.WorkflowDefinition:
		return 201

	default:
		return -1
	}
}

func (h handler) UpdateWorkflowDefinitionHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {

	input, err := newUpdateWorkflowDefinitionInput(r)
	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		http.Error(w, jsonMarshalNoError(models.BadRequest{Message: err.Error()}), http.StatusBadRequest)
		return
	}

	err = input.Validate()

	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		http.Error(w, jsonMarshalNoError(models.BadRequest{Message: err.Error()}), http.StatusBadRequest)
		return
	}

	resp, err := h.UpdateWorkflowDefinition(ctx, input)

	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		if btErr, ok := err.(*errors.Error); ok {
			logger.FromContext(ctx).AddContext("stacktrace", string(btErr.Stack()))
		}
		statusCode := statusCodeForUpdateWorkflowDefinition(err)
		if statusCode == -1 {
			err = models.InternalError{Message: err.Error()}
			statusCode = 500
		}
		http.Error(w, jsonMarshalNoError(err), statusCode)
		return
	}

	respBytes, err := json.MarshalIndent(resp, "", "\t")
	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		http.Error(w, jsonMarshalNoError(models.InternalError{Message: err.Error()}), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCodeForUpdateWorkflowDefinition(resp))
	w.Write(respBytes)

}

// newUpdateWorkflowDefinitionInput takes in an http.Request an returns the input struct.
func newUpdateWorkflowDefinitionInput(r *http.Request) (*models.UpdateWorkflowDefinitionInput, error) {
	var input models.UpdateWorkflowDefinitionInput

	var err error
	_ = err

	data, err := ioutil.ReadAll(r.Body)

	if len(data) > 0 {

		input.NewWorkflowDefinitionRequest = &models.NewWorkflowDefinitionRequest{}
		if err := json.NewDecoder(bytes.NewReader(data)).Decode(input.NewWorkflowDefinitionRequest); err != nil {
			return nil, err
		}

	}

	nameStr := mux.Vars(r)["name"]
	if len(nameStr) == 0 {
		return nil, errors.New("path parameter 'name' must be specified")
	}
	nameStrs := []string{nameStr}

	if len(nameStrs) > 0 {
		var nameTmp string
		nameStr := nameStrs[0]
		nameTmp, err = nameStr, error(nil)
		if err != nil {
			return nil, err
		}
		input.Name = nameTmp
	}

	return &input, nil
}

// statusCodeForGetWorkflowDefinitionByNameAndVersion returns the status code corresponding to the returned
// object. It returns -1 if the type doesn't correspond to anything.
func statusCodeForGetWorkflowDefinitionByNameAndVersion(obj interface{}) int {

	switch obj.(type) {

	case *models.BadRequest:
		return 400

	case *models.InternalError:
		return 500

	case *models.NotFound:
		return 404

	case *models.WorkflowDefinition:
		return 200

	case models.BadRequest:
		return 400

	case models.InternalError:
		return 500

	case models.NotFound:
		return 404

	case models.WorkflowDefinition:
		return 200

	default:
		return -1
	}
}

func (h handler) GetWorkflowDefinitionByNameAndVersionHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {

	input, err := newGetWorkflowDefinitionByNameAndVersionInput(r)
	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		http.Error(w, jsonMarshalNoError(models.BadRequest{Message: err.Error()}), http.StatusBadRequest)
		return
	}

	err = input.Validate()

	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		http.Error(w, jsonMarshalNoError(models.BadRequest{Message: err.Error()}), http.StatusBadRequest)
		return
	}

	resp, err := h.GetWorkflowDefinitionByNameAndVersion(ctx, input)

	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		if btErr, ok := err.(*errors.Error); ok {
			logger.FromContext(ctx).AddContext("stacktrace", string(btErr.Stack()))
		}
		statusCode := statusCodeForGetWorkflowDefinitionByNameAndVersion(err)
		if statusCode == -1 {
			err = models.InternalError{Message: err.Error()}
			statusCode = 500
		}
		http.Error(w, jsonMarshalNoError(err), statusCode)
		return
	}

	respBytes, err := json.MarshalIndent(resp, "", "\t")
	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		http.Error(w, jsonMarshalNoError(models.InternalError{Message: err.Error()}), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCodeForGetWorkflowDefinitionByNameAndVersion(resp))
	w.Write(respBytes)

}

// newGetWorkflowDefinitionByNameAndVersionInput takes in an http.Request an returns the input struct.
func newGetWorkflowDefinitionByNameAndVersionInput(r *http.Request) (*models.GetWorkflowDefinitionByNameAndVersionInput, error) {
	var input models.GetWorkflowDefinitionByNameAndVersionInput

	var err error
	_ = err

	nameStr := mux.Vars(r)["name"]
	if len(nameStr) == 0 {
		return nil, errors.New("path parameter 'name' must be specified")
	}
	nameStrs := []string{nameStr}

	if len(nameStrs) > 0 {
		var nameTmp string
		nameStr := nameStrs[0]
		nameTmp, err = nameStr, error(nil)
		if err != nil {
			return nil, err
		}
		input.Name = nameTmp
	}

	versionStr := mux.Vars(r)["version"]
	if len(versionStr) == 0 {
		return nil, errors.New("path parameter 'version' must be specified")
	}
	versionStrs := []string{versionStr}

	if len(versionStrs) > 0 {
		var versionTmp int64
		versionStr := versionStrs[0]
		versionTmp, err = swag.ConvertInt64(versionStr)
		if err != nil {
			return nil, err
		}
		input.Version = versionTmp
	}

	return &input, nil
}

// statusCodeForGetWorkflows returns the status code corresponding to the returned
// object. It returns -1 if the type doesn't correspond to anything.
func statusCodeForGetWorkflows(obj interface{}) int {

	switch obj.(type) {

	case *[]models.Workflow:
		return 200

	case *models.BadRequest:
		return 400

	case *models.InternalError:
		return 500

	case *models.NotFound:
		return 404

	case []models.Workflow:
		return 200

	case models.BadRequest:
		return 400

	case models.InternalError:
		return 500

	case models.NotFound:
		return 404

	default:
		return -1
	}
}

func (h handler) GetWorkflowsHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {

	input, err := newGetWorkflowsInput(r)
	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		http.Error(w, jsonMarshalNoError(models.BadRequest{Message: err.Error()}), http.StatusBadRequest)
		return
	}

	err = input.Validate()

	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		http.Error(w, jsonMarshalNoError(models.BadRequest{Message: err.Error()}), http.StatusBadRequest)
		return
	}

	resp, nextPageID, err := h.GetWorkflows(ctx, input)

	// Success types that return an array should never return nil so let's make this easier
	// for consumers by converting nil arrays to empty arrays
	if resp == nil {
		resp = []models.Workflow{}
	}

	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		if btErr, ok := err.(*errors.Error); ok {
			logger.FromContext(ctx).AddContext("stacktrace", string(btErr.Stack()))
		}
		statusCode := statusCodeForGetWorkflows(err)
		if statusCode == -1 {
			err = models.InternalError{Message: err.Error()}
			statusCode = 500
		}
		http.Error(w, jsonMarshalNoError(err), statusCode)
		return
	}

	respBytes, err := json.MarshalIndent(resp, "", "\t")
	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		http.Error(w, jsonMarshalNoError(models.InternalError{Message: err.Error()}), http.StatusInternalServerError)
		return
	}

	if !swag.IsZero(nextPageID) {
		input.PageToken = &nextPageID
		path, err := input.Path()
		if err != nil {
			logger.FromContext(ctx).AddContext("error", err.Error())
			http.Error(w, jsonMarshalNoError(models.InternalError{Message: err.Error()}), http.StatusInternalServerError)
			return
		}
		w.Header().Set("X-Next-Page-Path", path)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCodeForGetWorkflows(resp))
	w.Write(respBytes)

}

// newGetWorkflowsInput takes in an http.Request an returns the input struct.
func newGetWorkflowsInput(r *http.Request) (*models.GetWorkflowsInput, error) {
	var input models.GetWorkflowsInput

	var err error
	_ = err

	limitStrs := r.URL.Query()["limit"]

	if len(limitStrs) == 0 {
		limitStrs = []string{"10"}
	}
	if len(limitStrs) > 0 {
		var limitTmp int64
		limitStr := limitStrs[0]
		limitTmp, err = swag.ConvertInt64(limitStr)
		if err != nil {
			return nil, err
		}
		input.Limit = &limitTmp
	}

	oldestFirstStrs := r.URL.Query()["oldestFirst"]

	if len(oldestFirstStrs) > 0 {
		var oldestFirstTmp bool
		oldestFirstStr := oldestFirstStrs[0]
		oldestFirstTmp, err = strconv.ParseBool(oldestFirstStr)
		if err != nil {
			return nil, err
		}
		input.OldestFirst = &oldestFirstTmp
	}

	pageTokenStrs := r.URL.Query()["pageToken"]

	if len(pageTokenStrs) > 0 {
		var pageTokenTmp string
		pageTokenStr := pageTokenStrs[0]
		pageTokenTmp, err = pageTokenStr, error(nil)
		if err != nil {
			return nil, err
		}
		input.PageToken = &pageTokenTmp
	}

	statusStrs := r.URL.Query()["status"]

	if len(statusStrs) > 0 {
		var statusTmp string
		statusStr := statusStrs[0]
		statusTmp, err = statusStr, error(nil)
		if err != nil {
			return nil, err
		}
		input.Status = &statusTmp
	}

	resolvedByUserStrs := r.URL.Query()["resolvedByUser"]

	if len(resolvedByUserStrs) > 0 {
		var resolvedByUserTmp bool
		resolvedByUserStr := resolvedByUserStrs[0]
		resolvedByUserTmp, err = strconv.ParseBool(resolvedByUserStr)
		if err != nil {
			return nil, err
		}
		input.ResolvedByUser = &resolvedByUserTmp
	}

	summaryOnlyStrs := r.URL.Query()["summaryOnly"]

	if len(summaryOnlyStrs) == 0 {
		summaryOnlyStrs = []string{"false"}
	}
	if len(summaryOnlyStrs) > 0 {
		var summaryOnlyTmp bool
		summaryOnlyStr := summaryOnlyStrs[0]
		summaryOnlyTmp, err = strconv.ParseBool(summaryOnlyStr)
		if err != nil {
			return nil, err
		}
		input.SummaryOnly = &summaryOnlyTmp
	}

	workflowDefinitionNameStrs := r.URL.Query()["workflowDefinitionName"]
	if len(workflowDefinitionNameStrs) == 0 {
		return nil, errors.New("query parameter 'workflowDefinitionName' must be specified")
	}

	if len(workflowDefinitionNameStrs) > 0 {
		var workflowDefinitionNameTmp string
		workflowDefinitionNameStr := workflowDefinitionNameStrs[0]
		workflowDefinitionNameTmp, err = workflowDefinitionNameStr, error(nil)
		if err != nil {
			return nil, err
		}
		input.WorkflowDefinitionName = workflowDefinitionNameTmp
	}

	return &input, nil
}

// statusCodeForStartWorkflow returns the status code corresponding to the returned
// object. It returns -1 if the type doesn't correspond to anything.
func statusCodeForStartWorkflow(obj interface{}) int {

	switch obj.(type) {

	case *models.BadRequest:
		return 400

	case *models.InternalError:
		return 500

	case *models.NotFound:
		return 404

	case *models.Workflow:
		return 200

	case models.BadRequest:
		return 400

	case models.InternalError:
		return 500

	case models.NotFound:
		return 404

	case models.Workflow:
		return 200

	default:
		return -1
	}
}

func (h handler) StartWorkflowHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {

	input, err := newStartWorkflowInput(r)
	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		http.Error(w, jsonMarshalNoError(models.BadRequest{Message: err.Error()}), http.StatusBadRequest)
		return
	}

	if input != nil {
		err = input.Validate(nil)
	}

	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		http.Error(w, jsonMarshalNoError(models.BadRequest{Message: err.Error()}), http.StatusBadRequest)
		return
	}

	resp, err := h.StartWorkflow(ctx, input)

	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		if btErr, ok := err.(*errors.Error); ok {
			logger.FromContext(ctx).AddContext("stacktrace", string(btErr.Stack()))
		}
		statusCode := statusCodeForStartWorkflow(err)
		if statusCode == -1 {
			err = models.InternalError{Message: err.Error()}
			statusCode = 500
		}
		http.Error(w, jsonMarshalNoError(err), statusCode)
		return
	}

	respBytes, err := json.MarshalIndent(resp, "", "\t")
	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		http.Error(w, jsonMarshalNoError(models.InternalError{Message: err.Error()}), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCodeForStartWorkflow(resp))
	w.Write(respBytes)

}

// newStartWorkflowInput takes in an http.Request an returns the input struct.
func newStartWorkflowInput(r *http.Request) (*models.StartWorkflowRequest, error) {
	var err error
	_ = err

	data, err := ioutil.ReadAll(r.Body)

	if len(data) > 0 {

		var input models.StartWorkflowRequest
		if err := json.NewDecoder(bytes.NewReader(data)).Decode(&input); err != nil {
			return nil, err
		}
		return &input, nil

	}

	return nil, nil
}

// statusCodeForCancelWorkflow returns the status code corresponding to the returned
// object. It returns -1 if the type doesn't correspond to anything.
func statusCodeForCancelWorkflow(obj interface{}) int {

	switch obj.(type) {

	case *models.BadRequest:
		return 400

	case *models.InternalError:
		return 500

	case *models.NotFound:
		return 404

	case models.BadRequest:
		return 400

	case models.InternalError:
		return 500

	case models.NotFound:
		return 404

	default:
		return -1
	}
}

func (h handler) CancelWorkflowHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {

	input, err := newCancelWorkflowInput(r)
	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		http.Error(w, jsonMarshalNoError(models.BadRequest{Message: err.Error()}), http.StatusBadRequest)
		return
	}

	err = input.Validate()

	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		http.Error(w, jsonMarshalNoError(models.BadRequest{Message: err.Error()}), http.StatusBadRequest)
		return
	}

	err = h.CancelWorkflow(ctx, input)

	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		if btErr, ok := err.(*errors.Error); ok {
			logger.FromContext(ctx).AddContext("stacktrace", string(btErr.Stack()))
		}
		statusCode := statusCodeForCancelWorkflow(err)
		if statusCode == -1 {
			err = models.InternalError{Message: err.Error()}
			statusCode = 500
		}
		http.Error(w, jsonMarshalNoError(err), statusCode)
		return
	}

	w.WriteHeader(200)
	w.Write([]byte(""))

}

// newCancelWorkflowInput takes in an http.Request an returns the input struct.
func newCancelWorkflowInput(r *http.Request) (*models.CancelWorkflowInput, error) {
	var input models.CancelWorkflowInput

	var err error
	_ = err

	workflowIDStr := mux.Vars(r)["workflowID"]
	if len(workflowIDStr) == 0 {
		return nil, errors.New("path parameter 'workflowID' must be specified")
	}
	workflowIDStrs := []string{workflowIDStr}

	if len(workflowIDStrs) > 0 {
		var workflowIDTmp string
		workflowIDStr := workflowIDStrs[0]
		workflowIDTmp, err = workflowIDStr, error(nil)
		if err != nil {
			return nil, err
		}
		input.WorkflowID = workflowIDTmp
	}

	data, err := ioutil.ReadAll(r.Body)
	if len(data) == 0 {
		return nil, errors.New("request body is required, but was empty")
	}

	if len(data) > 0 {

		input.Reason = &models.CancelReason{}
		if err := json.NewDecoder(bytes.NewReader(data)).Decode(input.Reason); err != nil {
			return nil, err
		}

	}

	return &input, nil
}

// statusCodeForGetWorkflowByID returns the status code corresponding to the returned
// object. It returns -1 if the type doesn't correspond to anything.
func statusCodeForGetWorkflowByID(obj interface{}) int {

	switch obj.(type) {

	case *models.BadRequest:
		return 400

	case *models.InternalError:
		return 500

	case *models.NotFound:
		return 404

	case *models.Workflow:
		return 200

	case models.BadRequest:
		return 400

	case models.InternalError:
		return 500

	case models.NotFound:
		return 404

	case models.Workflow:
		return 200

	default:
		return -1
	}
}

func (h handler) GetWorkflowByIDHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {

	workflowID, err := newGetWorkflowByIDInput(r)
	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		http.Error(w, jsonMarshalNoError(models.BadRequest{Message: err.Error()}), http.StatusBadRequest)
		return
	}

	err = models.ValidateGetWorkflowByIDInput(workflowID)

	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		http.Error(w, jsonMarshalNoError(models.BadRequest{Message: err.Error()}), http.StatusBadRequest)
		return
	}

	resp, err := h.GetWorkflowByID(ctx, workflowID)

	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		if btErr, ok := err.(*errors.Error); ok {
			logger.FromContext(ctx).AddContext("stacktrace", string(btErr.Stack()))
		}
		statusCode := statusCodeForGetWorkflowByID(err)
		if statusCode == -1 {
			err = models.InternalError{Message: err.Error()}
			statusCode = 500
		}
		http.Error(w, jsonMarshalNoError(err), statusCode)
		return
	}

	respBytes, err := json.MarshalIndent(resp, "", "\t")
	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		http.Error(w, jsonMarshalNoError(models.InternalError{Message: err.Error()}), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCodeForGetWorkflowByID(resp))
	w.Write(respBytes)

}

// newGetWorkflowByIDInput takes in an http.Request an returns the workflowID parameter
// that it contains. It returns an error if the request doesn't contain the parameter.
func newGetWorkflowByIDInput(r *http.Request) (string, error) {
	workflowID := mux.Vars(r)["workflowID"]
	if len(workflowID) == 0 {
		return "", errors.New("Parameter workflowID must be specified")
	}
	return workflowID, nil
}

// statusCodeForResumeWorkflowByID returns the status code corresponding to the returned
// object. It returns -1 if the type doesn't correspond to anything.
func statusCodeForResumeWorkflowByID(obj interface{}) int {

	switch obj.(type) {

	case *models.BadRequest:
		return 400

	case *models.InternalError:
		return 500

	case *models.NotFound:
		return 404

	case *models.Workflow:
		return 200

	case models.BadRequest:
		return 400

	case models.InternalError:
		return 500

	case models.NotFound:
		return 404

	case models.Workflow:
		return 200

	default:
		return -1
	}
}

func (h handler) ResumeWorkflowByIDHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {

	input, err := newResumeWorkflowByIDInput(r)
	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		http.Error(w, jsonMarshalNoError(models.BadRequest{Message: err.Error()}), http.StatusBadRequest)
		return
	}

	err = input.Validate()

	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		http.Error(w, jsonMarshalNoError(models.BadRequest{Message: err.Error()}), http.StatusBadRequest)
		return
	}

	resp, err := h.ResumeWorkflowByID(ctx, input)

	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		if btErr, ok := err.(*errors.Error); ok {
			logger.FromContext(ctx).AddContext("stacktrace", string(btErr.Stack()))
		}
		statusCode := statusCodeForResumeWorkflowByID(err)
		if statusCode == -1 {
			err = models.InternalError{Message: err.Error()}
			statusCode = 500
		}
		http.Error(w, jsonMarshalNoError(err), statusCode)
		return
	}

	respBytes, err := json.MarshalIndent(resp, "", "\t")
	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		http.Error(w, jsonMarshalNoError(models.InternalError{Message: err.Error()}), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCodeForResumeWorkflowByID(resp))
	w.Write(respBytes)

}

// newResumeWorkflowByIDInput takes in an http.Request an returns the input struct.
func newResumeWorkflowByIDInput(r *http.Request) (*models.ResumeWorkflowByIDInput, error) {
	var input models.ResumeWorkflowByIDInput

	var err error
	_ = err

	workflowIDStr := mux.Vars(r)["workflowID"]
	if len(workflowIDStr) == 0 {
		return nil, errors.New("path parameter 'workflowID' must be specified")
	}
	workflowIDStrs := []string{workflowIDStr}

	if len(workflowIDStrs) > 0 {
		var workflowIDTmp string
		workflowIDStr := workflowIDStrs[0]
		workflowIDTmp, err = workflowIDStr, error(nil)
		if err != nil {
			return nil, err
		}
		input.WorkflowID = workflowIDTmp
	}

	data, err := ioutil.ReadAll(r.Body)
	if len(data) == 0 {
		return nil, errors.New("request body is required, but was empty")
	}

	if len(data) > 0 {

		input.Overrides = &models.WorkflowDefinitionOverrides{}
		if err := json.NewDecoder(bytes.NewReader(data)).Decode(input.Overrides); err != nil {
			return nil, err
		}

	}

	return &input, nil
}

// statusCodeForResolveWorkflowByID returns the status code corresponding to the returned
// object. It returns -1 if the type doesn't correspond to anything.
func statusCodeForResolveWorkflowByID(obj interface{}) int {

	switch obj.(type) {

	case *models.BadRequest:
		return 400

	case *models.Conflict:
		return 409

	case *models.InternalError:
		return 500

	case *models.NotFound:
		return 404

	case models.BadRequest:
		return 400

	case models.Conflict:
		return 409

	case models.InternalError:
		return 500

	case models.NotFound:
		return 404

	default:
		return -1
	}
}

func (h handler) ResolveWorkflowByIDHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {

	workflowID, err := newResolveWorkflowByIDInput(r)
	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		http.Error(w, jsonMarshalNoError(models.BadRequest{Message: err.Error()}), http.StatusBadRequest)
		return
	}

	err = models.ValidateResolveWorkflowByIDInput(workflowID)

	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		http.Error(w, jsonMarshalNoError(models.BadRequest{Message: err.Error()}), http.StatusBadRequest)
		return
	}

	err = h.ResolveWorkflowByID(ctx, workflowID)

	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		if btErr, ok := err.(*errors.Error); ok {
			logger.FromContext(ctx).AddContext("stacktrace", string(btErr.Stack()))
		}
		statusCode := statusCodeForResolveWorkflowByID(err)
		if statusCode == -1 {
			err = models.InternalError{Message: err.Error()}
			statusCode = 500
		}
		http.Error(w, jsonMarshalNoError(err), statusCode)
		return
	}

	w.WriteHeader(201)
	w.Write([]byte(""))

}

// newResolveWorkflowByIDInput takes in an http.Request an returns the workflowID parameter
// that it contains. It returns an error if the request doesn't contain the parameter.
func newResolveWorkflowByIDInput(r *http.Request) (string, error) {
	workflowID := mux.Vars(r)["workflowID"]
	if len(workflowID) == 0 {
		return "", errors.New("Parameter workflowID must be specified")
	}
	return workflowID, nil
}
