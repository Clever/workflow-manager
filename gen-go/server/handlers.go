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
	"gopkg.in/Clever/kayvee-go.v5/logger"
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
	bytes, err := json.Marshal(i)
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

// statusCodeForGetJobsForWorkflow returns the status code corresponding to the returned
// object. It returns -1 if the type doesn't correspond to anything.
func statusCodeForGetJobsForWorkflow(obj interface{}) int {

	switch obj.(type) {

	case *[]models.Job:
		return 200

	case *models.BadRequest:
		return 400

	case *models.InternalError:
		return 500

	case *models.NotFound:
		return 404

	case []models.Job:
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

func (h handler) GetJobsForWorkflowHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {

	workflowName, err := newGetJobsForWorkflowInput(r)
	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		http.Error(w, jsonMarshalNoError(models.BadRequest{Message: err.Error()}), http.StatusBadRequest)
		return
	}

	err = models.ValidateGetJobsForWorkflowInput(workflowName)

	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		http.Error(w, jsonMarshalNoError(models.BadRequest{Message: err.Error()}), http.StatusBadRequest)
		return
	}

	resp, err := h.GetJobsForWorkflow(ctx, workflowName)

	// Success types that return an array should never return nil so let's make this easier
	// for consumers by converting nil arrays to empty arrays
	if resp == nil {
		resp = []models.Job{}
	}

	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		if btErr, ok := err.(*errors.Error); ok {
			logger.FromContext(ctx).AddContext("stacktrace", string(btErr.Stack()))
		}
		statusCode := statusCodeForGetJobsForWorkflow(err)
		if statusCode == -1 {
			err = models.InternalError{Message: err.Error()}
			statusCode = 500
		}
		http.Error(w, jsonMarshalNoError(err), statusCode)
		return
	}

	respBytes, err := json.Marshal(resp)
	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		http.Error(w, jsonMarshalNoError(models.InternalError{Message: err.Error()}), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCodeForGetJobsForWorkflow(resp))
	w.Write(respBytes)

}

// newGetJobsForWorkflowInput takes in an http.Request an returns the workflowName parameter
// that it contains. It returns an error if the request doesn't contain the parameter.
func newGetJobsForWorkflowInput(r *http.Request) (string, error) {
	workflowName := mux.Vars(r)["workflowName"]
	if len(workflowName) == 0 {
		return "", errors.New("Parameter workflowName must be specified")
	}
	return workflowName, nil
}

// statusCodeForStartJobForWorkflow returns the status code corresponding to the returned
// object. It returns -1 if the type doesn't correspond to anything.
func statusCodeForStartJobForWorkflow(obj interface{}) int {

	switch obj.(type) {

	case *models.BadRequest:
		return 400

	case *models.InternalError:
		return 500

	case *models.Job:
		return 200

	case *models.NotFound:
		return 404

	case models.BadRequest:
		return 400

	case models.InternalError:
		return 500

	case models.Job:
		return 200

	case models.NotFound:
		return 404

	default:
		return -1
	}
}

func (h handler) StartJobForWorkflowHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {

	input, err := newStartJobForWorkflowInput(r)
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

	resp, err := h.StartJobForWorkflow(ctx, input)

	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		if btErr, ok := err.(*errors.Error); ok {
			logger.FromContext(ctx).AddContext("stacktrace", string(btErr.Stack()))
		}
		statusCode := statusCodeForStartJobForWorkflow(err)
		if statusCode == -1 {
			err = models.InternalError{Message: err.Error()}
			statusCode = 500
		}
		http.Error(w, jsonMarshalNoError(err), statusCode)
		return
	}

	respBytes, err := json.Marshal(resp)
	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		http.Error(w, jsonMarshalNoError(models.InternalError{Message: err.Error()}), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCodeForStartJobForWorkflow(resp))
	w.Write(respBytes)

}

// newStartJobForWorkflowInput takes in an http.Request an returns the input struct.
func newStartJobForWorkflowInput(r *http.Request) (*models.StartJobForWorkflowInput, error) {
	var input models.StartJobForWorkflowInput

	var err error
	_ = err

	workflowNameStr := mux.Vars(r)["workflowName"]
	if len(workflowNameStr) == 0 {
		return nil, errors.New("parameter must be specified")
	}
	workflowNameStrs := []string{workflowNameStr}

	if len(workflowNameStrs) > 0 {
		var workflowNameTmp string
		workflowNameStr := workflowNameStrs[0]
		workflowNameTmp, err = workflowNameStr, error(nil)
		if err != nil {
			return nil, err
		}
		input.WorkflowName = workflowNameTmp
	}

	data, err := ioutil.ReadAll(r.Body)

	if len(data) > 0 {
		input.Input = &models.JobInput{}
		if err := json.NewDecoder(bytes.NewReader(data)).Decode(input.Input); err != nil {
			return nil, err
		}
	}

	return &input, nil
}

// statusCodeForNewWorkflow returns the status code corresponding to the returned
// object. It returns -1 if the type doesn't correspond to anything.
func statusCodeForNewWorkflow(obj interface{}) int {

	switch obj.(type) {

	case *models.BadRequest:
		return 400

	case *models.InternalError:
		return 500

	case *models.Workflow:
		return 201

	case models.BadRequest:
		return 400

	case models.InternalError:
		return 500

	case models.Workflow:
		return 201

	default:
		return -1
	}
}

func (h handler) NewWorkflowHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {

	input, err := newNewWorkflowInput(r)
	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		http.Error(w, jsonMarshalNoError(models.BadRequest{Message: err.Error()}), http.StatusBadRequest)
		return
	}

	err = input.Validate(nil)

	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		http.Error(w, jsonMarshalNoError(models.BadRequest{Message: err.Error()}), http.StatusBadRequest)
		return
	}

	resp, err := h.NewWorkflow(ctx, input)

	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		if btErr, ok := err.(*errors.Error); ok {
			logger.FromContext(ctx).AddContext("stacktrace", string(btErr.Stack()))
		}
		statusCode := statusCodeForNewWorkflow(err)
		if statusCode == -1 {
			err = models.InternalError{Message: err.Error()}
			statusCode = 500
		}
		http.Error(w, jsonMarshalNoError(err), statusCode)
		return
	}

	respBytes, err := json.Marshal(resp)
	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		http.Error(w, jsonMarshalNoError(models.InternalError{Message: err.Error()}), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCodeForNewWorkflow(resp))
	w.Write(respBytes)

}

// newNewWorkflowInput takes in an http.Request an returns the input struct.
func newNewWorkflowInput(r *http.Request) (*models.NewWorkflowRequest, error) {
	var input models.NewWorkflowRequest

	var err error
	_ = err

	data, err := ioutil.ReadAll(r.Body)

	if len(data) > 0 {
		if err := json.NewDecoder(bytes.NewReader(data)).Decode(&input); err != nil {
			return nil, err
		}
	}

	return &input, nil
}

// statusCodeForGetWorkflowByName returns the status code corresponding to the returned
// object. It returns -1 if the type doesn't correspond to anything.
func statusCodeForGetWorkflowByName(obj interface{}) int {

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

func (h handler) GetWorkflowByNameHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {

	name, err := newGetWorkflowByNameInput(r)
	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		http.Error(w, jsonMarshalNoError(models.BadRequest{Message: err.Error()}), http.StatusBadRequest)
		return
	}

	err = models.ValidateGetWorkflowByNameInput(name)

	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		http.Error(w, jsonMarshalNoError(models.BadRequest{Message: err.Error()}), http.StatusBadRequest)
		return
	}

	resp, err := h.GetWorkflowByName(ctx, name)

	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		if btErr, ok := err.(*errors.Error); ok {
			logger.FromContext(ctx).AddContext("stacktrace", string(btErr.Stack()))
		}
		statusCode := statusCodeForGetWorkflowByName(err)
		if statusCode == -1 {
			err = models.InternalError{Message: err.Error()}
			statusCode = 500
		}
		http.Error(w, jsonMarshalNoError(err), statusCode)
		return
	}

	respBytes, err := json.Marshal(resp)
	if err != nil {
		logger.FromContext(ctx).AddContext("error", err.Error())
		http.Error(w, jsonMarshalNoError(models.InternalError{Message: err.Error()}), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCodeForGetWorkflowByName(resp))
	w.Write(respBytes)

}

// newGetWorkflowByNameInput takes in an http.Request an returns the name parameter
// that it contains. It returns an error if the request doesn't contain the parameter.
func newGetWorkflowByNameInput(r *http.Request) (string, error) {
	name := mux.Vars(r)["name"]
	if len(name) == 0 {
		return "", errors.New("Parameter name must be specified")
	}
	return name, nil
}
