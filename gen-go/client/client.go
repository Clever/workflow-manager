package client

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	discovery "github.com/Clever/discovery-go"
	"github.com/Clever/workflow-manager/gen-go/models"
	"github.com/afex/hystrix-go/hystrix"
	logger "gopkg.in/Clever/kayvee-go.v6/logger"
)

var _ = json.Marshal
var _ = strings.Replace
var _ = strconv.FormatInt
var _ = bytes.Compare

// WagClient is used to make requests to the workflow-manager service.
type WagClient struct {
	basePath    string
	requestDoer doer
	transport   *http.Transport
	timeout     time.Duration
	// Keep the retry doer around so that we can set the number of retries
	retryDoer *retryDoer
	// Keep the circuit doer around so that we can turn it on / off
	circuitDoer    *circuitBreakerDoer
	defaultTimeout time.Duration
	logger         logger.KayveeLogger
}

var _ Client = (*WagClient)(nil)

// New creates a new client. The base path and http transport are configurable.
func New(basePath string) *WagClient {
	base := baseDoer{}
	tracing := tracingDoer{d: base}
	// For the short-term don't use the default retry policy since its 5 retries can 5X
	// the traffic. Once we've enabled circuit breakers by default we can turn it on.
	retry := retryDoer{d: tracing, retryPolicy: SingleRetryPolicy{}}
	logger := logger.New("workflow-manager-wagclient")
	circuit := &circuitBreakerDoer{
		d:     &retry,
		debug: true,
		// one circuit for each service + url pair
		circuitName: fmt.Sprintf("workflow-manager-%s", shortHash(basePath)),
		logger:      logger,
	}
	circuit.init()
	client := &WagClient{
		requestDoer:    circuit,
		retryDoer:      &retry,
		circuitDoer:    circuit,
		defaultTimeout: 10 * time.Second,
		transport:      &http.Transport{},
		basePath:       basePath,
		logger:         logger,
	}
	client.SetCircuitBreakerSettings(DefaultCircuitBreakerSettings)
	return client
}

// NewFromDiscovery creates a client from the discovery environment variables. This method requires
// the three env vars: SERVICE_WORKFLOW_MANAGER_HTTP_(HOST/PORT/PROTO) to be set. Otherwise it returns an error.
func NewFromDiscovery() (*WagClient, error) {
	url, err := discovery.URL("workflow-manager", "default")
	if err != nil {
		url, err = discovery.URL("workflow-manager", "http") // Added fallback to maintain reverse compatibility
		if err != nil {
			return nil, err
		}
	}
	return New(url), nil
}

// SetRetryPolicy sets a the given retry policy for all requests.
func (c *WagClient) SetRetryPolicy(retryPolicy RetryPolicy) {
	c.retryDoer.retryPolicy = retryPolicy
}

// SetCircuitBreakerDebug puts the circuit
func (c *WagClient) SetCircuitBreakerDebug(b bool) {
	c.circuitDoer.debug = b
}

// SetLogger allows for setting a custom logger
func (c *WagClient) SetLogger(logger logger.KayveeLogger) {
	c.logger = logger
	c.circuitDoer.logger = logger
}

// CircuitBreakerSettings are the parameters that govern the client's circuit breaker.
type CircuitBreakerSettings struct {
	// MaxConcurrentRequests is the maximum number of concurrent requests
	// the client can make at the same time. Default: 100.
	MaxConcurrentRequests int
	// RequestVolumeThreshold is the minimum number of requests needed
	// before a circuit can be tripped due to health. Default: 20.
	RequestVolumeThreshold int
	// SleepWindow how long, in milliseconds, to wait after a circuit opens
	// before testing for recovery. Default: 5000.
	SleepWindow int
	// ErrorPercentThreshold is the threshold to place on the rolling error
	// rate. Once the error rate exceeds this percentage, the circuit opens.
	// Default: 90.
	ErrorPercentThreshold int
}

// DefaultCircuitBreakerSettings describes the default circuit parameters.
var DefaultCircuitBreakerSettings = CircuitBreakerSettings{
	MaxConcurrentRequests:  100,
	RequestVolumeThreshold: 20,
	SleepWindow:            5000,
	ErrorPercentThreshold:  90,
}

// SetCircuitBreakerSettings sets parameters on the circuit breaker. It must be
// called on application startup.
func (c *WagClient) SetCircuitBreakerSettings(settings CircuitBreakerSettings) {
	hystrix.ConfigureCommand(c.circuitDoer.circuitName, hystrix.CommandConfig{
		// redundant, with the timeout we set on the context, so set
		// this to something high and irrelevant
		Timeout:                100 * 1000,
		MaxConcurrentRequests:  settings.MaxConcurrentRequests,
		RequestVolumeThreshold: settings.RequestVolumeThreshold,
		SleepWindow:            settings.SleepWindow,
		ErrorPercentThreshold:  settings.ErrorPercentThreshold,
	})
}

// SetTimeout sets a timeout on all operations for the client. To make a single request
// with a timeout use context.WithTimeout as described here: https://godoc.org/golang.org/x/net/context#WithTimeout.
func (c *WagClient) SetTimeout(timeout time.Duration) {
	c.defaultTimeout = timeout
}

// HealthCheck makes a GET request to /_health
// Checks if the service is healthy
// 200: nil
// 400: *models.BadRequest
// 500: *models.InternalError
// default: client side HTTP errors, for example: context.DeadlineExceeded.
func (c *WagClient) HealthCheck(ctx context.Context) error {
	headers := make(map[string]string)

	var body []byte
	path := c.basePath + "/_health"

	req, err := http.NewRequest("GET", path, bytes.NewBuffer(body))

	if err != nil {
		return err
	}

	return c.doHealthCheckRequest(ctx, req, headers)
}

func (c *WagClient) doHealthCheckRequest(ctx context.Context, req *http.Request, headers map[string]string) error {
	client := &http.Client{Transport: c.transport}

	for field, value := range headers {
		req.Header.Set(field, value)
	}

	// Add the opname for doers like tracing
	ctx = context.WithValue(ctx, opNameCtx{}, "healthCheck")
	req = req.WithContext(ctx)
	// Don't add the timeout in a "doer" because we don't want to call "defer.cancel()"
	// until we've finished all the processing of the request object. Otherwise we'll cancel
	// our own request before we've finished it.
	if c.defaultTimeout != 0 {
		ctx, cancel := context.WithTimeout(req.Context(), c.defaultTimeout)
		defer cancel()
		req = req.WithContext(ctx)
	}
	resp, err := c.requestDoer.Do(client, req)
	retCode := 0
	if resp != nil {
		retCode = resp.StatusCode
	}

	// log all client failures and non-successful HT
	logData := logger.M{
		"backend":     "workflow-manager",
		"method":      req.Method,
		"uri":         req.URL,
		"status_code": retCode,
	}
	if err == nil && retCode > 399 {
		logData["message"] = resp.Status
		c.logger.ErrorD("client-request-finished", logData)
	}
	if err != nil {
		logData["message"] = err.Error()
		c.logger.ErrorD("client-request-finished", logData)
		return err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {

	case 200:

		return nil

	case 400:

		var output models.BadRequest
		if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
			return err
		}
		return &output

	case 500:

		var output models.InternalError
		if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
			return err
		}
		return &output

	default:
		return &models.InternalError{Message: "Unknown response"}
	}
}

// PostStateResource makes a POST request to /state-resources
//
// 201: *models.StateResource
// 400: *models.BadRequest
// 500: *models.InternalError
// default: client side HTTP errors, for example: context.DeadlineExceeded.
func (c *WagClient) PostStateResource(ctx context.Context, i *models.NewStateResource) (*models.StateResource, error) {
	headers := make(map[string]string)

	var body []byte
	path := c.basePath + "/state-resources"

	if i != nil {

		var err error
		body, err = json.Marshal(i)

		if err != nil {
			return nil, err
		}

	}

	req, err := http.NewRequest("POST", path, bytes.NewBuffer(body))

	if err != nil {
		return nil, err
	}

	return c.doPostStateResourceRequest(ctx, req, headers)
}

func (c *WagClient) doPostStateResourceRequest(ctx context.Context, req *http.Request, headers map[string]string) (*models.StateResource, error) {
	client := &http.Client{Transport: c.transport}

	for field, value := range headers {
		req.Header.Set(field, value)
	}

	// Add the opname for doers like tracing
	ctx = context.WithValue(ctx, opNameCtx{}, "postStateResource")
	req = req.WithContext(ctx)
	// Don't add the timeout in a "doer" because we don't want to call "defer.cancel()"
	// until we've finished all the processing of the request object. Otherwise we'll cancel
	// our own request before we've finished it.
	if c.defaultTimeout != 0 {
		ctx, cancel := context.WithTimeout(req.Context(), c.defaultTimeout)
		defer cancel()
		req = req.WithContext(ctx)
	}
	resp, err := c.requestDoer.Do(client, req)
	retCode := 0
	if resp != nil {
		retCode = resp.StatusCode
	}

	// log all client failures and non-successful HT
	logData := logger.M{
		"backend":     "workflow-manager",
		"method":      req.Method,
		"uri":         req.URL,
		"status_code": retCode,
	}
	if err == nil && retCode > 399 {
		logData["message"] = resp.Status
		c.logger.ErrorD("client-request-finished", logData)
	}
	if err != nil {
		logData["message"] = err.Error()
		c.logger.ErrorD("client-request-finished", logData)
		return nil, err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {

	case 201:

		var output models.StateResource
		if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
			return nil, err
		}

		return &output, nil

	case 400:

		var output models.BadRequest
		if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
			return nil, err
		}
		return nil, &output

	case 500:

		var output models.InternalError
		if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
			return nil, err
		}
		return nil, &output

	default:
		return nil, &models.InternalError{Message: "Unknown response"}
	}
}

// DeleteStateResource makes a DELETE request to /state-resources/{namespace}/{name}
//
// 200: nil
// 400: *models.BadRequest
// 404: *models.NotFound
// 500: *models.InternalError
// default: client side HTTP errors, for example: context.DeadlineExceeded.
func (c *WagClient) DeleteStateResource(ctx context.Context, i *models.DeleteStateResourceInput) error {
	headers := make(map[string]string)

	var body []byte
	path, err := i.Path()

	if err != nil {
		return err
	}

	path = c.basePath + path

	req, err := http.NewRequest("DELETE", path, bytes.NewBuffer(body))

	if err != nil {
		return err
	}

	return c.doDeleteStateResourceRequest(ctx, req, headers)
}

func (c *WagClient) doDeleteStateResourceRequest(ctx context.Context, req *http.Request, headers map[string]string) error {
	client := &http.Client{Transport: c.transport}

	for field, value := range headers {
		req.Header.Set(field, value)
	}

	// Add the opname for doers like tracing
	ctx = context.WithValue(ctx, opNameCtx{}, "deleteStateResource")
	req = req.WithContext(ctx)
	// Don't add the timeout in a "doer" because we don't want to call "defer.cancel()"
	// until we've finished all the processing of the request object. Otherwise we'll cancel
	// our own request before we've finished it.
	if c.defaultTimeout != 0 {
		ctx, cancel := context.WithTimeout(req.Context(), c.defaultTimeout)
		defer cancel()
		req = req.WithContext(ctx)
	}
	resp, err := c.requestDoer.Do(client, req)
	retCode := 0
	if resp != nil {
		retCode = resp.StatusCode
	}

	// log all client failures and non-successful HT
	logData := logger.M{
		"backend":     "workflow-manager",
		"method":      req.Method,
		"uri":         req.URL,
		"status_code": retCode,
	}
	if err == nil && retCode > 399 {
		logData["message"] = resp.Status
		c.logger.ErrorD("client-request-finished", logData)
	}
	if err != nil {
		logData["message"] = err.Error()
		c.logger.ErrorD("client-request-finished", logData)
		return err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {

	case 200:

		return nil

	case 400:

		var output models.BadRequest
		if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
			return err
		}
		return &output

	case 404:

		var output models.NotFound
		if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
			return err
		}
		return &output

	case 500:

		var output models.InternalError
		if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
			return err
		}
		return &output

	default:
		return &models.InternalError{Message: "Unknown response"}
	}
}

// GetStateResource makes a GET request to /state-resources/{namespace}/{name}
//
// 200: *models.StateResource
// 400: *models.BadRequest
// 404: *models.NotFound
// 500: *models.InternalError
// default: client side HTTP errors, for example: context.DeadlineExceeded.
func (c *WagClient) GetStateResource(ctx context.Context, i *models.GetStateResourceInput) (*models.StateResource, error) {
	headers := make(map[string]string)

	var body []byte
	path, err := i.Path()

	if err != nil {
		return nil, err
	}

	path = c.basePath + path

	req, err := http.NewRequest("GET", path, bytes.NewBuffer(body))

	if err != nil {
		return nil, err
	}

	return c.doGetStateResourceRequest(ctx, req, headers)
}

func (c *WagClient) doGetStateResourceRequest(ctx context.Context, req *http.Request, headers map[string]string) (*models.StateResource, error) {
	client := &http.Client{Transport: c.transport}

	for field, value := range headers {
		req.Header.Set(field, value)
	}

	// Add the opname for doers like tracing
	ctx = context.WithValue(ctx, opNameCtx{}, "getStateResource")
	req = req.WithContext(ctx)
	// Don't add the timeout in a "doer" because we don't want to call "defer.cancel()"
	// until we've finished all the processing of the request object. Otherwise we'll cancel
	// our own request before we've finished it.
	if c.defaultTimeout != 0 {
		ctx, cancel := context.WithTimeout(req.Context(), c.defaultTimeout)
		defer cancel()
		req = req.WithContext(ctx)
	}
	resp, err := c.requestDoer.Do(client, req)
	retCode := 0
	if resp != nil {
		retCode = resp.StatusCode
	}

	// log all client failures and non-successful HT
	logData := logger.M{
		"backend":     "workflow-manager",
		"method":      req.Method,
		"uri":         req.URL,
		"status_code": retCode,
	}
	if err == nil && retCode > 399 {
		logData["message"] = resp.Status
		c.logger.ErrorD("client-request-finished", logData)
	}
	if err != nil {
		logData["message"] = err.Error()
		c.logger.ErrorD("client-request-finished", logData)
		return nil, err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {

	case 200:

		var output models.StateResource
		if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
			return nil, err
		}

		return &output, nil

	case 400:

		var output models.BadRequest
		if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
			return nil, err
		}
		return nil, &output

	case 404:

		var output models.NotFound
		if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
			return nil, err
		}
		return nil, &output

	case 500:

		var output models.InternalError
		if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
			return nil, err
		}
		return nil, &output

	default:
		return nil, &models.InternalError{Message: "Unknown response"}
	}
}

// PutStateResource makes a PUT request to /state-resources/{namespace}/{name}
//
// 201: *models.StateResource
// 400: *models.BadRequest
// 500: *models.InternalError
// default: client side HTTP errors, for example: context.DeadlineExceeded.
func (c *WagClient) PutStateResource(ctx context.Context, i *models.PutStateResourceInput) (*models.StateResource, error) {
	headers := make(map[string]string)

	var body []byte
	path, err := i.Path()

	if err != nil {
		return nil, err
	}

	path = c.basePath + path

	if i.NewStateResource != nil {

		var err error
		body, err = json.Marshal(i.NewStateResource)

		if err != nil {
			return nil, err
		}

	}

	req, err := http.NewRequest("PUT", path, bytes.NewBuffer(body))

	if err != nil {
		return nil, err
	}

	return c.doPutStateResourceRequest(ctx, req, headers)
}

func (c *WagClient) doPutStateResourceRequest(ctx context.Context, req *http.Request, headers map[string]string) (*models.StateResource, error) {
	client := &http.Client{Transport: c.transport}

	for field, value := range headers {
		req.Header.Set(field, value)
	}

	// Add the opname for doers like tracing
	ctx = context.WithValue(ctx, opNameCtx{}, "putStateResource")
	req = req.WithContext(ctx)
	// Don't add the timeout in a "doer" because we don't want to call "defer.cancel()"
	// until we've finished all the processing of the request object. Otherwise we'll cancel
	// our own request before we've finished it.
	if c.defaultTimeout != 0 {
		ctx, cancel := context.WithTimeout(req.Context(), c.defaultTimeout)
		defer cancel()
		req = req.WithContext(ctx)
	}
	resp, err := c.requestDoer.Do(client, req)
	retCode := 0
	if resp != nil {
		retCode = resp.StatusCode
	}

	// log all client failures and non-successful HT
	logData := logger.M{
		"backend":     "workflow-manager",
		"method":      req.Method,
		"uri":         req.URL,
		"status_code": retCode,
	}
	if err == nil && retCode > 399 {
		logData["message"] = resp.Status
		c.logger.ErrorD("client-request-finished", logData)
	}
	if err != nil {
		logData["message"] = err.Error()
		c.logger.ErrorD("client-request-finished", logData)
		return nil, err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {

	case 201:

		var output models.StateResource
		if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
			return nil, err
		}

		return &output, nil

	case 400:

		var output models.BadRequest
		if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
			return nil, err
		}
		return nil, &output

	case 500:

		var output models.InternalError
		if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
			return nil, err
		}
		return nil, &output

	default:
		return nil, &models.InternalError{Message: "Unknown response"}
	}
}

// GetWorkflowDefinitions makes a GET request to /workflow-definitions
// Get the latest versions of all available WorkflowDefinitions
// 200: []models.WorkflowDefinition
// 400: *models.BadRequest
// 500: *models.InternalError
// default: client side HTTP errors, for example: context.DeadlineExceeded.
func (c *WagClient) GetWorkflowDefinitions(ctx context.Context) ([]models.WorkflowDefinition, error) {
	headers := make(map[string]string)

	var body []byte
	path := c.basePath + "/workflow-definitions"

	req, err := http.NewRequest("GET", path, bytes.NewBuffer(body))

	if err != nil {
		return nil, err
	}

	return c.doGetWorkflowDefinitionsRequest(ctx, req, headers)
}

func (c *WagClient) doGetWorkflowDefinitionsRequest(ctx context.Context, req *http.Request, headers map[string]string) ([]models.WorkflowDefinition, error) {
	client := &http.Client{Transport: c.transport}

	for field, value := range headers {
		req.Header.Set(field, value)
	}

	// Add the opname for doers like tracing
	ctx = context.WithValue(ctx, opNameCtx{}, "getWorkflowDefinitions")
	req = req.WithContext(ctx)
	// Don't add the timeout in a "doer" because we don't want to call "defer.cancel()"
	// until we've finished all the processing of the request object. Otherwise we'll cancel
	// our own request before we've finished it.
	if c.defaultTimeout != 0 {
		ctx, cancel := context.WithTimeout(req.Context(), c.defaultTimeout)
		defer cancel()
		req = req.WithContext(ctx)
	}
	resp, err := c.requestDoer.Do(client, req)
	retCode := 0
	if resp != nil {
		retCode = resp.StatusCode
	}

	// log all client failures and non-successful HT
	logData := logger.M{
		"backend":     "workflow-manager",
		"method":      req.Method,
		"uri":         req.URL,
		"status_code": retCode,
	}
	if err == nil && retCode > 399 {
		logData["message"] = resp.Status
		c.logger.ErrorD("client-request-finished", logData)
	}
	if err != nil {
		logData["message"] = err.Error()
		c.logger.ErrorD("client-request-finished", logData)
		return nil, err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {

	case 200:

		var output []models.WorkflowDefinition
		if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
			return nil, err
		}

		return output, nil

	case 400:

		var output models.BadRequest
		if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
			return nil, err
		}
		return nil, &output

	case 500:

		var output models.InternalError
		if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
			return nil, err
		}
		return nil, &output

	default:
		return nil, &models.InternalError{Message: "Unknown response"}
	}
}

// NewWorkflowDefinition makes a POST request to /workflow-definitions
//
// 201: *models.WorkflowDefinition
// 400: *models.BadRequest
// 500: *models.InternalError
// default: client side HTTP errors, for example: context.DeadlineExceeded.
func (c *WagClient) NewWorkflowDefinition(ctx context.Context, i *models.NewWorkflowDefinitionRequest) (*models.WorkflowDefinition, error) {
	headers := make(map[string]string)

	var body []byte
	path := c.basePath + "/workflow-definitions"

	if i != nil {

		var err error
		body, err = json.Marshal(i)

		if err != nil {
			return nil, err
		}

	}

	req, err := http.NewRequest("POST", path, bytes.NewBuffer(body))

	if err != nil {
		return nil, err
	}

	return c.doNewWorkflowDefinitionRequest(ctx, req, headers)
}

func (c *WagClient) doNewWorkflowDefinitionRequest(ctx context.Context, req *http.Request, headers map[string]string) (*models.WorkflowDefinition, error) {
	client := &http.Client{Transport: c.transport}

	for field, value := range headers {
		req.Header.Set(field, value)
	}

	// Add the opname for doers like tracing
	ctx = context.WithValue(ctx, opNameCtx{}, "newWorkflowDefinition")
	req = req.WithContext(ctx)
	// Don't add the timeout in a "doer" because we don't want to call "defer.cancel()"
	// until we've finished all the processing of the request object. Otherwise we'll cancel
	// our own request before we've finished it.
	if c.defaultTimeout != 0 {
		ctx, cancel := context.WithTimeout(req.Context(), c.defaultTimeout)
		defer cancel()
		req = req.WithContext(ctx)
	}
	resp, err := c.requestDoer.Do(client, req)
	retCode := 0
	if resp != nil {
		retCode = resp.StatusCode
	}

	// log all client failures and non-successful HT
	logData := logger.M{
		"backend":     "workflow-manager",
		"method":      req.Method,
		"uri":         req.URL,
		"status_code": retCode,
	}
	if err == nil && retCode > 399 {
		logData["message"] = resp.Status
		c.logger.ErrorD("client-request-finished", logData)
	}
	if err != nil {
		logData["message"] = err.Error()
		c.logger.ErrorD("client-request-finished", logData)
		return nil, err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {

	case 201:

		var output models.WorkflowDefinition
		if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
			return nil, err
		}

		return &output, nil

	case 400:

		var output models.BadRequest
		if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
			return nil, err
		}
		return nil, &output

	case 500:

		var output models.InternalError
		if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
			return nil, err
		}
		return nil, &output

	default:
		return nil, &models.InternalError{Message: "Unknown response"}
	}
}

// GetWorkflowDefinitionVersionsByName makes a GET request to /workflow-definitions/{name}
//
// 200: []models.WorkflowDefinition
// 400: *models.BadRequest
// 404: *models.NotFound
// 500: *models.InternalError
// default: client side HTTP errors, for example: context.DeadlineExceeded.
func (c *WagClient) GetWorkflowDefinitionVersionsByName(ctx context.Context, i *models.GetWorkflowDefinitionVersionsByNameInput) ([]models.WorkflowDefinition, error) {
	headers := make(map[string]string)

	var body []byte
	path, err := i.Path()

	if err != nil {
		return nil, err
	}

	path = c.basePath + path

	req, err := http.NewRequest("GET", path, bytes.NewBuffer(body))

	if err != nil {
		return nil, err
	}

	return c.doGetWorkflowDefinitionVersionsByNameRequest(ctx, req, headers)
}

func (c *WagClient) doGetWorkflowDefinitionVersionsByNameRequest(ctx context.Context, req *http.Request, headers map[string]string) ([]models.WorkflowDefinition, error) {
	client := &http.Client{Transport: c.transport}

	for field, value := range headers {
		req.Header.Set(field, value)
	}

	// Add the opname for doers like tracing
	ctx = context.WithValue(ctx, opNameCtx{}, "getWorkflowDefinitionVersionsByName")
	req = req.WithContext(ctx)
	// Don't add the timeout in a "doer" because we don't want to call "defer.cancel()"
	// until we've finished all the processing of the request object. Otherwise we'll cancel
	// our own request before we've finished it.
	if c.defaultTimeout != 0 {
		ctx, cancel := context.WithTimeout(req.Context(), c.defaultTimeout)
		defer cancel()
		req = req.WithContext(ctx)
	}
	resp, err := c.requestDoer.Do(client, req)
	retCode := 0
	if resp != nil {
		retCode = resp.StatusCode
	}

	// log all client failures and non-successful HT
	logData := logger.M{
		"backend":     "workflow-manager",
		"method":      req.Method,
		"uri":         req.URL,
		"status_code": retCode,
	}
	if err == nil && retCode > 399 {
		logData["message"] = resp.Status
		c.logger.ErrorD("client-request-finished", logData)
	}
	if err != nil {
		logData["message"] = err.Error()
		c.logger.ErrorD("client-request-finished", logData)
		return nil, err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {

	case 200:

		var output []models.WorkflowDefinition
		if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
			return nil, err
		}

		return output, nil

	case 400:

		var output models.BadRequest
		if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
			return nil, err
		}
		return nil, &output

	case 404:

		var output models.NotFound
		if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
			return nil, err
		}
		return nil, &output

	case 500:

		var output models.InternalError
		if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
			return nil, err
		}
		return nil, &output

	default:
		return nil, &models.InternalError{Message: "Unknown response"}
	}
}

// UpdateWorkflowDefinition makes a PUT request to /workflow-definitions/{name}
//
// 201: *models.WorkflowDefinition
// 400: *models.BadRequest
// 404: *models.NotFound
// 500: *models.InternalError
// default: client side HTTP errors, for example: context.DeadlineExceeded.
func (c *WagClient) UpdateWorkflowDefinition(ctx context.Context, i *models.UpdateWorkflowDefinitionInput) (*models.WorkflowDefinition, error) {
	headers := make(map[string]string)

	var body []byte
	path, err := i.Path()

	if err != nil {
		return nil, err
	}

	path = c.basePath + path

	if i.NewWorkflowDefinitionRequest != nil {

		var err error
		body, err = json.Marshal(i.NewWorkflowDefinitionRequest)

		if err != nil {
			return nil, err
		}

	}

	req, err := http.NewRequest("PUT", path, bytes.NewBuffer(body))

	if err != nil {
		return nil, err
	}

	return c.doUpdateWorkflowDefinitionRequest(ctx, req, headers)
}

func (c *WagClient) doUpdateWorkflowDefinitionRequest(ctx context.Context, req *http.Request, headers map[string]string) (*models.WorkflowDefinition, error) {
	client := &http.Client{Transport: c.transport}

	for field, value := range headers {
		req.Header.Set(field, value)
	}

	// Add the opname for doers like tracing
	ctx = context.WithValue(ctx, opNameCtx{}, "updateWorkflowDefinition")
	req = req.WithContext(ctx)
	// Don't add the timeout in a "doer" because we don't want to call "defer.cancel()"
	// until we've finished all the processing of the request object. Otherwise we'll cancel
	// our own request before we've finished it.
	if c.defaultTimeout != 0 {
		ctx, cancel := context.WithTimeout(req.Context(), c.defaultTimeout)
		defer cancel()
		req = req.WithContext(ctx)
	}
	resp, err := c.requestDoer.Do(client, req)
	retCode := 0
	if resp != nil {
		retCode = resp.StatusCode
	}

	// log all client failures and non-successful HT
	logData := logger.M{
		"backend":     "workflow-manager",
		"method":      req.Method,
		"uri":         req.URL,
		"status_code": retCode,
	}
	if err == nil && retCode > 399 {
		logData["message"] = resp.Status
		c.logger.ErrorD("client-request-finished", logData)
	}
	if err != nil {
		logData["message"] = err.Error()
		c.logger.ErrorD("client-request-finished", logData)
		return nil, err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {

	case 201:

		var output models.WorkflowDefinition
		if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
			return nil, err
		}

		return &output, nil

	case 400:

		var output models.BadRequest
		if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
			return nil, err
		}
		return nil, &output

	case 404:

		var output models.NotFound
		if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
			return nil, err
		}
		return nil, &output

	case 500:

		var output models.InternalError
		if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
			return nil, err
		}
		return nil, &output

	default:
		return nil, &models.InternalError{Message: "Unknown response"}
	}
}

// GetWorkflowDefinitionByNameAndVersion makes a GET request to /workflow-definitions/{name}/{version}
//
// 200: *models.WorkflowDefinition
// 400: *models.BadRequest
// 404: *models.NotFound
// 500: *models.InternalError
// default: client side HTTP errors, for example: context.DeadlineExceeded.
func (c *WagClient) GetWorkflowDefinitionByNameAndVersion(ctx context.Context, i *models.GetWorkflowDefinitionByNameAndVersionInput) (*models.WorkflowDefinition, error) {
	headers := make(map[string]string)

	var body []byte
	path, err := i.Path()

	if err != nil {
		return nil, err
	}

	path = c.basePath + path

	req, err := http.NewRequest("GET", path, bytes.NewBuffer(body))

	if err != nil {
		return nil, err
	}

	return c.doGetWorkflowDefinitionByNameAndVersionRequest(ctx, req, headers)
}

func (c *WagClient) doGetWorkflowDefinitionByNameAndVersionRequest(ctx context.Context, req *http.Request, headers map[string]string) (*models.WorkflowDefinition, error) {
	client := &http.Client{Transport: c.transport}

	for field, value := range headers {
		req.Header.Set(field, value)
	}

	// Add the opname for doers like tracing
	ctx = context.WithValue(ctx, opNameCtx{}, "getWorkflowDefinitionByNameAndVersion")
	req = req.WithContext(ctx)
	// Don't add the timeout in a "doer" because we don't want to call "defer.cancel()"
	// until we've finished all the processing of the request object. Otherwise we'll cancel
	// our own request before we've finished it.
	if c.defaultTimeout != 0 {
		ctx, cancel := context.WithTimeout(req.Context(), c.defaultTimeout)
		defer cancel()
		req = req.WithContext(ctx)
	}
	resp, err := c.requestDoer.Do(client, req)
	retCode := 0
	if resp != nil {
		retCode = resp.StatusCode
	}

	// log all client failures and non-successful HT
	logData := logger.M{
		"backend":     "workflow-manager",
		"method":      req.Method,
		"uri":         req.URL,
		"status_code": retCode,
	}
	if err == nil && retCode > 399 {
		logData["message"] = resp.Status
		c.logger.ErrorD("client-request-finished", logData)
	}
	if err != nil {
		logData["message"] = err.Error()
		c.logger.ErrorD("client-request-finished", logData)
		return nil, err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {

	case 200:

		var output models.WorkflowDefinition
		if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
			return nil, err
		}

		return &output, nil

	case 400:

		var output models.BadRequest
		if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
			return nil, err
		}
		return nil, &output

	case 404:

		var output models.NotFound
		if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
			return nil, err
		}
		return nil, &output

	case 500:

		var output models.InternalError
		if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
			return nil, err
		}
		return nil, &output

	default:
		return nil, &models.InternalError{Message: "Unknown response"}
	}
}

// GetWorkflows makes a GET request to /workflows
//
// 200: []models.Workflow
// 400: *models.BadRequest
// 404: *models.NotFound
// 500: *models.InternalError
// default: client side HTTP errors, for example: context.DeadlineExceeded.
func (c *WagClient) GetWorkflows(ctx context.Context, i *models.GetWorkflowsInput) ([]models.Workflow, error) {
	headers := make(map[string]string)

	var body []byte
	path, err := i.Path()

	if err != nil {
		return nil, err
	}

	path = c.basePath + path

	req, err := http.NewRequest("GET", path, bytes.NewBuffer(body))

	if err != nil {
		return nil, err
	}

	return c.doGetWorkflowsRequest(ctx, req, headers)
}

func (c *WagClient) doGetWorkflowsRequest(ctx context.Context, req *http.Request, headers map[string]string) ([]models.Workflow, error) {
	client := &http.Client{Transport: c.transport}

	for field, value := range headers {
		req.Header.Set(field, value)
	}

	// Add the opname for doers like tracing
	ctx = context.WithValue(ctx, opNameCtx{}, "getWorkflows")
	req = req.WithContext(ctx)
	// Don't add the timeout in a "doer" because we don't want to call "defer.cancel()"
	// until we've finished all the processing of the request object. Otherwise we'll cancel
	// our own request before we've finished it.
	if c.defaultTimeout != 0 {
		ctx, cancel := context.WithTimeout(req.Context(), c.defaultTimeout)
		defer cancel()
		req = req.WithContext(ctx)
	}
	resp, err := c.requestDoer.Do(client, req)
	retCode := 0
	if resp != nil {
		retCode = resp.StatusCode
	}

	// log all client failures and non-successful HT
	logData := logger.M{
		"backend":     "workflow-manager",
		"method":      req.Method,
		"uri":         req.URL,
		"status_code": retCode,
	}
	if err == nil && retCode > 399 {
		logData["message"] = resp.Status
		c.logger.ErrorD("client-request-finished", logData)
	}
	if err != nil {
		logData["message"] = err.Error()
		c.logger.ErrorD("client-request-finished", logData)
		return nil, err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {

	case 200:

		var output []models.Workflow
		if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
			return nil, err
		}

		return output, nil

	case 400:

		var output models.BadRequest
		if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
			return nil, err
		}
		return nil, &output

	case 404:

		var output models.NotFound
		if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
			return nil, err
		}
		return nil, &output

	case 500:

		var output models.InternalError
		if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
			return nil, err
		}
		return nil, &output

	default:
		return nil, &models.InternalError{Message: "Unknown response"}
	}
}

// StartWorkflow makes a POST request to /workflows
//
// 200: *models.Workflow
// 400: *models.BadRequest
// 404: *models.NotFound
// 500: *models.InternalError
// default: client side HTTP errors, for example: context.DeadlineExceeded.
func (c *WagClient) StartWorkflow(ctx context.Context, i *models.StartWorkflowRequest) (*models.Workflow, error) {
	headers := make(map[string]string)

	var body []byte
	path := c.basePath + "/workflows"

	if i != nil {

		var err error
		body, err = json.Marshal(i)

		if err != nil {
			return nil, err
		}

	}

	req, err := http.NewRequest("POST", path, bytes.NewBuffer(body))

	if err != nil {
		return nil, err
	}

	return c.doStartWorkflowRequest(ctx, req, headers)
}

func (c *WagClient) doStartWorkflowRequest(ctx context.Context, req *http.Request, headers map[string]string) (*models.Workflow, error) {
	client := &http.Client{Transport: c.transport}

	for field, value := range headers {
		req.Header.Set(field, value)
	}

	// Add the opname for doers like tracing
	ctx = context.WithValue(ctx, opNameCtx{}, "startWorkflow")
	req = req.WithContext(ctx)
	// Don't add the timeout in a "doer" because we don't want to call "defer.cancel()"
	// until we've finished all the processing of the request object. Otherwise we'll cancel
	// our own request before we've finished it.
	if c.defaultTimeout != 0 {
		ctx, cancel := context.WithTimeout(req.Context(), c.defaultTimeout)
		defer cancel()
		req = req.WithContext(ctx)
	}
	resp, err := c.requestDoer.Do(client, req)
	retCode := 0
	if resp != nil {
		retCode = resp.StatusCode
	}

	// log all client failures and non-successful HT
	logData := logger.M{
		"backend":     "workflow-manager",
		"method":      req.Method,
		"uri":         req.URL,
		"status_code": retCode,
	}
	if err == nil && retCode > 399 {
		logData["message"] = resp.Status
		c.logger.ErrorD("client-request-finished", logData)
	}
	if err != nil {
		logData["message"] = err.Error()
		c.logger.ErrorD("client-request-finished", logData)
		return nil, err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {

	case 200:

		var output models.Workflow
		if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
			return nil, err
		}

		return &output, nil

	case 400:

		var output models.BadRequest
		if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
			return nil, err
		}
		return nil, &output

	case 404:

		var output models.NotFound
		if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
			return nil, err
		}
		return nil, &output

	case 500:

		var output models.InternalError
		if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
			return nil, err
		}
		return nil, &output

	default:
		return nil, &models.InternalError{Message: "Unknown response"}
	}
}

// CancelWorkflow makes a DELETE request to /workflows/{workflowId}
//
// 200: nil
// 400: *models.BadRequest
// 404: *models.NotFound
// 500: *models.InternalError
// default: client side HTTP errors, for example: context.DeadlineExceeded.
func (c *WagClient) CancelWorkflow(ctx context.Context, i *models.CancelWorkflowInput) error {
	headers := make(map[string]string)

	var body []byte
	path, err := i.Path()

	if err != nil {
		return err
	}

	path = c.basePath + path

	if i.Reason != nil {

		var err error
		body, err = json.Marshal(i.Reason)

		if err != nil {
			return err
		}

	}

	req, err := http.NewRequest("DELETE", path, bytes.NewBuffer(body))

	if err != nil {
		return err
	}

	return c.doCancelWorkflowRequest(ctx, req, headers)
}

func (c *WagClient) doCancelWorkflowRequest(ctx context.Context, req *http.Request, headers map[string]string) error {
	client := &http.Client{Transport: c.transport}

	for field, value := range headers {
		req.Header.Set(field, value)
	}

	// Add the opname for doers like tracing
	ctx = context.WithValue(ctx, opNameCtx{}, "CancelWorkflow")
	req = req.WithContext(ctx)
	// Don't add the timeout in a "doer" because we don't want to call "defer.cancel()"
	// until we've finished all the processing of the request object. Otherwise we'll cancel
	// our own request before we've finished it.
	if c.defaultTimeout != 0 {
		ctx, cancel := context.WithTimeout(req.Context(), c.defaultTimeout)
		defer cancel()
		req = req.WithContext(ctx)
	}
	resp, err := c.requestDoer.Do(client, req)
	retCode := 0
	if resp != nil {
		retCode = resp.StatusCode
	}

	// log all client failures and non-successful HT
	logData := logger.M{
		"backend":     "workflow-manager",
		"method":      req.Method,
		"uri":         req.URL,
		"status_code": retCode,
	}
	if err == nil && retCode > 399 {
		logData["message"] = resp.Status
		c.logger.ErrorD("client-request-finished", logData)
	}
	if err != nil {
		logData["message"] = err.Error()
		c.logger.ErrorD("client-request-finished", logData)
		return err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {

	case 200:

		return nil

	case 400:

		var output models.BadRequest
		if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
			return err
		}
		return &output

	case 404:

		var output models.NotFound
		if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
			return err
		}
		return &output

	case 500:

		var output models.InternalError
		if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
			return err
		}
		return &output

	default:
		return &models.InternalError{Message: "Unknown response"}
	}
}

// GetWorkflowByID makes a GET request to /workflows/{workflowId}
//
// 200: *models.Workflow
// 400: *models.BadRequest
// 404: *models.NotFound
// 500: *models.InternalError
// default: client side HTTP errors, for example: context.DeadlineExceeded.
func (c *WagClient) GetWorkflowByID(ctx context.Context, workflowId string) (*models.Workflow, error) {
	headers := make(map[string]string)

	var body []byte
	path, err := models.GetWorkflowByIDInputPath(workflowId)

	if err != nil {
		return nil, err
	}

	path = c.basePath + path

	req, err := http.NewRequest("GET", path, bytes.NewBuffer(body))

	if err != nil {
		return nil, err
	}

	return c.doGetWorkflowByIDRequest(ctx, req, headers)
}

func (c *WagClient) doGetWorkflowByIDRequest(ctx context.Context, req *http.Request, headers map[string]string) (*models.Workflow, error) {
	client := &http.Client{Transport: c.transport}

	for field, value := range headers {
		req.Header.Set(field, value)
	}

	// Add the opname for doers like tracing
	ctx = context.WithValue(ctx, opNameCtx{}, "getWorkflowByID")
	req = req.WithContext(ctx)
	// Don't add the timeout in a "doer" because we don't want to call "defer.cancel()"
	// until we've finished all the processing of the request object. Otherwise we'll cancel
	// our own request before we've finished it.
	if c.defaultTimeout != 0 {
		ctx, cancel := context.WithTimeout(req.Context(), c.defaultTimeout)
		defer cancel()
		req = req.WithContext(ctx)
	}
	resp, err := c.requestDoer.Do(client, req)
	retCode := 0
	if resp != nil {
		retCode = resp.StatusCode
	}

	// log all client failures and non-successful HT
	logData := logger.M{
		"backend":     "workflow-manager",
		"method":      req.Method,
		"uri":         req.URL,
		"status_code": retCode,
	}
	if err == nil && retCode > 399 {
		logData["message"] = resp.Status
		c.logger.ErrorD("client-request-finished", logData)
	}
	if err != nil {
		logData["message"] = err.Error()
		c.logger.ErrorD("client-request-finished", logData)
		return nil, err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {

	case 200:

		var output models.Workflow
		if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
			return nil, err
		}

		return &output, nil

	case 400:

		var output models.BadRequest
		if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
			return nil, err
		}
		return nil, &output

	case 404:

		var output models.NotFound
		if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
			return nil, err
		}
		return nil, &output

	case 500:

		var output models.InternalError
		if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
			return nil, err
		}
		return nil, &output

	default:
		return nil, &models.InternalError{Message: "Unknown response"}
	}
}

func shortHash(s string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(s)))[0:6]
}
