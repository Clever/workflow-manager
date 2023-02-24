package client

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Clever/workflow-manager/gen-go/models"

	discovery "github.com/Clever/discovery-go"
	wcl "github.com/Clever/wag/logging/wagclientlogger"

	"github.com/afex/hystrix-go/hystrix"
)

var _ = json.Marshal
var _ = strings.Replace
var _ = strconv.FormatInt
var _ = bytes.Compare

// Version of the client.
const Version = "0.15.0"

// VersionHeader is sent with every request.
const VersionHeader = "X-Client-Version"

// WagClient is used to make requests to the workflow-manager service.
type WagClient struct {
	basePath    string
	requestDoer doer
	client      *http.Client
	// Keep the retry doer around so that we can set the number of retries
	retryDoer *retryDoer
	// Keep the circuit doer around so that we can turn it on / off
	circuitDoer    *circuitBreakerDoer
	defaultTimeout time.Duration
	logger         wcl.WagClientLogger
}

var _ Client = (*WagClient)(nil)

// New creates a new client. The base path, logger, and http transport are configurable.
// The logger provided should be specifically created for this wag client. If tracing is required,
// provide an instrumented transport using the wag clientconfig module. If no tracing is required, pass nil to use
// the default transport.
func New(basePath string, logger wcl.WagClientLogger, transport *http.RoundTripper) *WagClient {

	t := http.DefaultTransport
	if transport != nil {
		t = *transport
	}

	basePath = strings.TrimSuffix(basePath, "/")
	base := baseDoer{}

	// For the short-term don't use the default retry policy since its 5 retries can 5X
	// the traffic. Once we've enabled circuit breakers by default we can turn it on.
	retry := retryDoer{d: base, retryPolicy: SingleRetryPolicy{}}

	circuit := &circuitBreakerDoer{
		d: &retry,
		// TODO: INFRANG-4404 allow passing circuitBreakerOptions
		debug: true,
		// one circuit for each service + url pair
		circuitName: fmt.Sprintf("workflow-manager-%s", shortHash(basePath)),
		logger:      logger,
	}
	circuit.init()

	client := &WagClient{
		basePath:    basePath,
		requestDoer: circuit,
		client: &http.Client{
			Transport: t,
		},
		retryDoer:      &retry,
		circuitDoer:    circuit,
		defaultTimeout: 5 * time.Second,
		logger:         logger,
	}
	client.SetCircuitBreakerSettings(DefaultCircuitBreakerSettings)
	return client
}

// NewFromDiscovery creates a client from the discovery environment variables. This method requires
// the three env vars: SERVICE_WORKFLOW_MANAGER_HTTP_(HOST/PORT/PROTO) to be set. Otherwise it returns an error.
// The logger provided should be specifically created for this wag client. If tracing is required,
// provide an instrumented transport using the wag clientconfig module. If no tracing is required, pass nil to use
// the default transport.
func NewFromDiscovery(logger wcl.WagClientLogger, transport *http.RoundTripper) (*WagClient, error) {
	url, err := discovery.URL("workflow-manager", "default")
	if err != nil {
		url, err = discovery.URL("workflow-manager", "http") // Added fallback to maintain reverse compatibility
		if err != nil {
			return nil, err
		}
	}
	return New(url, logger, transport), nil
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
func (c *WagClient) SetLogger(l wcl.WagClientLogger) {
	c.logger = l
	c.circuitDoer.logger = l
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

// SetTimeout sets a timeout on all operations for the client. To make a single request with a shorter timeout
// than the default on the client, use context.WithTimeout as described here: https://godoc.org/golang.org/x/net/context#WithTimeout.
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

	req, err := http.NewRequestWithContext(ctx, "GET", path, bytes.NewBuffer(body))

	if err != nil {
		return err
	}

	return c.doHealthCheckRequest(ctx, req, headers)
}

func (c *WagClient) doHealthCheckRequest(ctx context.Context, req *http.Request, headers map[string]string) error {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Canonical-Resource", "healthCheck")
	req.Header.Set(VersionHeader, Version)

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

	resp, err := c.requestDoer.Do(c.client, req)
	retCode := 0
	if resp != nil {
		retCode = resp.StatusCode
	}

	// log all client failures and non-successful HT
	logData := map[string]interface{}{
		"backend":     "workflow-manager",
		"method":      req.Method,
		"uri":         req.URL,
		"status_code": retCode,
	}
	if err == nil && retCode > 399 {
		logData["message"] = resp.Status
		c.logger.Log(wcl.Error, "client-request-finished", logData)
	}
	if err != nil {
		logData["message"] = err.Error()
		c.logger.Log(wcl.Error, "client-request-finished", logData)
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
		bs, _ := ioutil.ReadAll(resp.Body)
		return models.UnknownResponse{StatusCode: int64(resp.StatusCode), Body: string(bs)}
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

	req, err := http.NewRequestWithContext(ctx, "POST", path, bytes.NewBuffer(body))

	if err != nil {
		return nil, err
	}

	return c.doPostStateResourceRequest(ctx, req, headers)
}

func (c *WagClient) doPostStateResourceRequest(ctx context.Context, req *http.Request, headers map[string]string) (*models.StateResource, error) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Canonical-Resource", "postStateResource")
	req.Header.Set(VersionHeader, Version)

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

	resp, err := c.requestDoer.Do(c.client, req)
	retCode := 0
	if resp != nil {
		retCode = resp.StatusCode
	}

	// log all client failures and non-successful HT
	logData := map[string]interface{}{
		"backend":     "workflow-manager",
		"method":      req.Method,
		"uri":         req.URL,
		"status_code": retCode,
	}
	if err == nil && retCode > 399 {
		logData["message"] = resp.Status
		c.logger.Log(wcl.Error, "client-request-finished", logData)
	}
	if err != nil {
		logData["message"] = err.Error()
		c.logger.Log(wcl.Error, "client-request-finished", logData)
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
		bs, _ := ioutil.ReadAll(resp.Body)
		return nil, models.UnknownResponse{StatusCode: int64(resp.StatusCode), Body: string(bs)}
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

	req, err := http.NewRequestWithContext(ctx, "DELETE", path, bytes.NewBuffer(body))

	if err != nil {
		return err
	}

	return c.doDeleteStateResourceRequest(ctx, req, headers)
}

func (c *WagClient) doDeleteStateResourceRequest(ctx context.Context, req *http.Request, headers map[string]string) error {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Canonical-Resource", "deleteStateResource")
	req.Header.Set(VersionHeader, Version)

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

	resp, err := c.requestDoer.Do(c.client, req)
	retCode := 0
	if resp != nil {
		retCode = resp.StatusCode
	}

	// log all client failures and non-successful HT
	logData := map[string]interface{}{
		"backend":     "workflow-manager",
		"method":      req.Method,
		"uri":         req.URL,
		"status_code": retCode,
	}
	if err == nil && retCode > 399 {
		logData["message"] = resp.Status
		c.logger.Log(wcl.Error, "client-request-finished", logData)
	}
	if err != nil {
		logData["message"] = err.Error()
		c.logger.Log(wcl.Error, "client-request-finished", logData)
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
		bs, _ := ioutil.ReadAll(resp.Body)
		return models.UnknownResponse{StatusCode: int64(resp.StatusCode), Body: string(bs)}
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

	req, err := http.NewRequestWithContext(ctx, "GET", path, bytes.NewBuffer(body))

	if err != nil {
		return nil, err
	}

	return c.doGetStateResourceRequest(ctx, req, headers)
}

func (c *WagClient) doGetStateResourceRequest(ctx context.Context, req *http.Request, headers map[string]string) (*models.StateResource, error) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Canonical-Resource", "getStateResource")
	req.Header.Set(VersionHeader, Version)

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

	resp, err := c.requestDoer.Do(c.client, req)
	retCode := 0
	if resp != nil {
		retCode = resp.StatusCode
	}

	// log all client failures and non-successful HT
	logData := map[string]interface{}{
		"backend":     "workflow-manager",
		"method":      req.Method,
		"uri":         req.URL,
		"status_code": retCode,
	}
	if err == nil && retCode > 399 {
		logData["message"] = resp.Status
		c.logger.Log(wcl.Error, "client-request-finished", logData)
	}
	if err != nil {
		logData["message"] = err.Error()
		c.logger.Log(wcl.Error, "client-request-finished", logData)
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
		bs, _ := ioutil.ReadAll(resp.Body)
		return nil, models.UnknownResponse{StatusCode: int64(resp.StatusCode), Body: string(bs)}
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

	req, err := http.NewRequestWithContext(ctx, "PUT", path, bytes.NewBuffer(body))

	if err != nil {
		return nil, err
	}

	return c.doPutStateResourceRequest(ctx, req, headers)
}

func (c *WagClient) doPutStateResourceRequest(ctx context.Context, req *http.Request, headers map[string]string) (*models.StateResource, error) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Canonical-Resource", "putStateResource")
	req.Header.Set(VersionHeader, Version)

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

	resp, err := c.requestDoer.Do(c.client, req)
	retCode := 0
	if resp != nil {
		retCode = resp.StatusCode
	}

	// log all client failures and non-successful HT
	logData := map[string]interface{}{
		"backend":     "workflow-manager",
		"method":      req.Method,
		"uri":         req.URL,
		"status_code": retCode,
	}
	if err == nil && retCode > 399 {
		logData["message"] = resp.Status
		c.logger.Log(wcl.Error, "client-request-finished", logData)
	}
	if err != nil {
		logData["message"] = err.Error()
		c.logger.Log(wcl.Error, "client-request-finished", logData)
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
		bs, _ := ioutil.ReadAll(resp.Body)
		return nil, models.UnknownResponse{StatusCode: int64(resp.StatusCode), Body: string(bs)}
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

	req, err := http.NewRequestWithContext(ctx, "GET", path, bytes.NewBuffer(body))

	if err != nil {
		return nil, err
	}

	return c.doGetWorkflowDefinitionsRequest(ctx, req, headers)
}

func (c *WagClient) doGetWorkflowDefinitionsRequest(ctx context.Context, req *http.Request, headers map[string]string) ([]models.WorkflowDefinition, error) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Canonical-Resource", "getWorkflowDefinitions")
	req.Header.Set(VersionHeader, Version)

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

	resp, err := c.requestDoer.Do(c.client, req)
	retCode := 0
	if resp != nil {
		retCode = resp.StatusCode
	}

	// log all client failures and non-successful HT
	logData := map[string]interface{}{
		"backend":     "workflow-manager",
		"method":      req.Method,
		"uri":         req.URL,
		"status_code": retCode,
	}
	if err == nil && retCode > 399 {
		logData["message"] = resp.Status
		c.logger.Log(wcl.Error, "client-request-finished", logData)
	}
	if err != nil {
		logData["message"] = err.Error()
		c.logger.Log(wcl.Error, "client-request-finished", logData)
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
		bs, _ := ioutil.ReadAll(resp.Body)
		return nil, models.UnknownResponse{StatusCode: int64(resp.StatusCode), Body: string(bs)}
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

	req, err := http.NewRequestWithContext(ctx, "POST", path, bytes.NewBuffer(body))

	if err != nil {
		return nil, err
	}

	return c.doNewWorkflowDefinitionRequest(ctx, req, headers)
}

func (c *WagClient) doNewWorkflowDefinitionRequest(ctx context.Context, req *http.Request, headers map[string]string) (*models.WorkflowDefinition, error) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Canonical-Resource", "newWorkflowDefinition")
	req.Header.Set(VersionHeader, Version)

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

	resp, err := c.requestDoer.Do(c.client, req)
	retCode := 0
	if resp != nil {
		retCode = resp.StatusCode
	}

	// log all client failures and non-successful HT
	logData := map[string]interface{}{
		"backend":     "workflow-manager",
		"method":      req.Method,
		"uri":         req.URL,
		"status_code": retCode,
	}
	if err == nil && retCode > 399 {
		logData["message"] = resp.Status
		c.logger.Log(wcl.Error, "client-request-finished", logData)
	}
	if err != nil {
		logData["message"] = err.Error()
		c.logger.Log(wcl.Error, "client-request-finished", logData)
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
		bs, _ := ioutil.ReadAll(resp.Body)
		return nil, models.UnknownResponse{StatusCode: int64(resp.StatusCode), Body: string(bs)}
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

	req, err := http.NewRequestWithContext(ctx, "GET", path, bytes.NewBuffer(body))

	if err != nil {
		return nil, err
	}

	return c.doGetWorkflowDefinitionVersionsByNameRequest(ctx, req, headers)
}

func (c *WagClient) doGetWorkflowDefinitionVersionsByNameRequest(ctx context.Context, req *http.Request, headers map[string]string) ([]models.WorkflowDefinition, error) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Canonical-Resource", "getWorkflowDefinitionVersionsByName")
	req.Header.Set(VersionHeader, Version)

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

	resp, err := c.requestDoer.Do(c.client, req)
	retCode := 0
	if resp != nil {
		retCode = resp.StatusCode
	}

	// log all client failures and non-successful HT
	logData := map[string]interface{}{
		"backend":     "workflow-manager",
		"method":      req.Method,
		"uri":         req.URL,
		"status_code": retCode,
	}
	if err == nil && retCode > 399 {
		logData["message"] = resp.Status
		c.logger.Log(wcl.Error, "client-request-finished", logData)
	}
	if err != nil {
		logData["message"] = err.Error()
		c.logger.Log(wcl.Error, "client-request-finished", logData)
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
		bs, _ := ioutil.ReadAll(resp.Body)
		return nil, models.UnknownResponse{StatusCode: int64(resp.StatusCode), Body: string(bs)}
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

	req, err := http.NewRequestWithContext(ctx, "PUT", path, bytes.NewBuffer(body))

	if err != nil {
		return nil, err
	}

	return c.doUpdateWorkflowDefinitionRequest(ctx, req, headers)
}

func (c *WagClient) doUpdateWorkflowDefinitionRequest(ctx context.Context, req *http.Request, headers map[string]string) (*models.WorkflowDefinition, error) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Canonical-Resource", "updateWorkflowDefinition")
	req.Header.Set(VersionHeader, Version)

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

	resp, err := c.requestDoer.Do(c.client, req)
	retCode := 0
	if resp != nil {
		retCode = resp.StatusCode
	}

	// log all client failures and non-successful HT
	logData := map[string]interface{}{
		"backend":     "workflow-manager",
		"method":      req.Method,
		"uri":         req.URL,
		"status_code": retCode,
	}
	if err == nil && retCode > 399 {
		logData["message"] = resp.Status
		c.logger.Log(wcl.Error, "client-request-finished", logData)
	}
	if err != nil {
		logData["message"] = err.Error()
		c.logger.Log(wcl.Error, "client-request-finished", logData)
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
		bs, _ := ioutil.ReadAll(resp.Body)
		return nil, models.UnknownResponse{StatusCode: int64(resp.StatusCode), Body: string(bs)}
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

	req, err := http.NewRequestWithContext(ctx, "GET", path, bytes.NewBuffer(body))

	if err != nil {
		return nil, err
	}

	return c.doGetWorkflowDefinitionByNameAndVersionRequest(ctx, req, headers)
}

func (c *WagClient) doGetWorkflowDefinitionByNameAndVersionRequest(ctx context.Context, req *http.Request, headers map[string]string) (*models.WorkflowDefinition, error) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Canonical-Resource", "getWorkflowDefinitionByNameAndVersion")
	req.Header.Set(VersionHeader, Version)

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

	resp, err := c.requestDoer.Do(c.client, req)
	retCode := 0
	if resp != nil {
		retCode = resp.StatusCode
	}

	// log all client failures and non-successful HT
	logData := map[string]interface{}{
		"backend":     "workflow-manager",
		"method":      req.Method,
		"uri":         req.URL,
		"status_code": retCode,
	}
	if err == nil && retCode > 399 {
		logData["message"] = resp.Status
		c.logger.Log(wcl.Error, "client-request-finished", logData)
	}
	if err != nil {
		logData["message"] = err.Error()
		c.logger.Log(wcl.Error, "client-request-finished", logData)
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
		bs, _ := ioutil.ReadAll(resp.Body)
		return nil, models.UnknownResponse{StatusCode: int64(resp.StatusCode), Body: string(bs)}
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

	req, err := http.NewRequestWithContext(ctx, "GET", path, bytes.NewBuffer(body))

	if err != nil {
		return nil, err
	}

	resp, _, err := c.doGetWorkflowsRequest(ctx, req, headers)
	return resp, err
}

type getWorkflowsIterImpl struct {
	c            *WagClient
	ctx          context.Context
	lastResponse []models.Workflow
	index        int
	err          error
	nextURL      string
	headers      map[string]string
	body         []byte
}

// NewgetWorkflowsIter constructs an iterator that makes calls to getWorkflows for
// each page.
func (c *WagClient) NewGetWorkflowsIter(ctx context.Context, i *models.GetWorkflowsInput) (GetWorkflowsIter, error) {
	path, err := i.Path()

	if err != nil {
		return nil, err
	}

	path = c.basePath + path

	headers := make(map[string]string)

	var body []byte

	return &getWorkflowsIterImpl{
		c:            c,
		ctx:          ctx,
		lastResponse: []models.Workflow{},
		nextURL:      path,
		headers:      headers,
		body:         body,
	}, nil
}

func (i *getWorkflowsIterImpl) refresh() error {
	req, err := http.NewRequestWithContext(i.ctx, "GET", i.nextURL, bytes.NewBuffer(i.body))

	if err != nil {
		i.err = err
		return err
	}

	resp, nextPage, err := i.c.doGetWorkflowsRequest(i.ctx, req, i.headers)
	if err != nil {
		i.err = err
		return err
	}

	i.lastResponse = resp
	i.index = 0
	if nextPage != "" {
		i.nextURL = i.c.basePath + nextPage
	} else {
		i.nextURL = ""
	}
	return nil
}

// Next retrieves the next resource from the iterator and assigns it to the
// provided pointer, fetching a new page if necessary. Returns true if it
// successfully retrieves a new resource.
func (i *getWorkflowsIterImpl) Next(v *models.Workflow) bool {
	if i.err != nil {
		return false
	} else if i.index < len(i.lastResponse) {
		*v = i.lastResponse[i.index]
		i.index++
		return true
	} else if i.nextURL == "" {
		return false
	}

	if err := i.refresh(); err != nil {
		return false
	}
	return i.Next(v)
}

// Err returns an error if one occurred when .Next was called.
func (i *getWorkflowsIterImpl) Err() error {
	return i.err
}

func (c *WagClient) doGetWorkflowsRequest(ctx context.Context, req *http.Request, headers map[string]string) ([]models.Workflow, string, error) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Canonical-Resource", "getWorkflows")
	req.Header.Set(VersionHeader, Version)

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

	resp, err := c.requestDoer.Do(c.client, req)
	retCode := 0
	if resp != nil {
		retCode = resp.StatusCode
	}

	// log all client failures and non-successful HT
	logData := map[string]interface{}{
		"backend":     "workflow-manager",
		"method":      req.Method,
		"uri":         req.URL,
		"status_code": retCode,
	}
	if err == nil && retCode > 399 {
		logData["message"] = resp.Status
		c.logger.Log(wcl.Error, "client-request-finished", logData)
	}
	if err != nil {
		logData["message"] = err.Error()
		c.logger.Log(wcl.Error, "client-request-finished", logData)
		return nil, "", err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {

	case 200:

		var output []models.Workflow
		if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
			return nil, "", err
		}

		return output, resp.Header.Get("X-Next-Page-Path"), nil

	case 400:

		var output models.BadRequest
		if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
			return nil, "", err
		}
		return nil, "", &output

	case 404:

		var output models.NotFound
		if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
			return nil, "", err
		}
		return nil, "", &output

	case 500:

		var output models.InternalError
		if err := json.NewDecoder(resp.Body).Decode(&output); err != nil {
			return nil, "", err
		}
		return nil, "", &output

	default:
		bs, _ := ioutil.ReadAll(resp.Body)
		return nil, "", models.UnknownResponse{StatusCode: int64(resp.StatusCode), Body: string(bs)}
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

	req, err := http.NewRequestWithContext(ctx, "POST", path, bytes.NewBuffer(body))

	if err != nil {
		return nil, err
	}

	return c.doStartWorkflowRequest(ctx, req, headers)
}

func (c *WagClient) doStartWorkflowRequest(ctx context.Context, req *http.Request, headers map[string]string) (*models.Workflow, error) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Canonical-Resource", "startWorkflow")
	req.Header.Set(VersionHeader, Version)

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

	resp, err := c.requestDoer.Do(c.client, req)
	retCode := 0
	if resp != nil {
		retCode = resp.StatusCode
	}

	// log all client failures and non-successful HT
	logData := map[string]interface{}{
		"backend":     "workflow-manager",
		"method":      req.Method,
		"uri":         req.URL,
		"status_code": retCode,
	}
	if err == nil && retCode > 399 {
		logData["message"] = resp.Status
		c.logger.Log(wcl.Error, "client-request-finished", logData)
	}
	if err != nil {
		logData["message"] = err.Error()
		c.logger.Log(wcl.Error, "client-request-finished", logData)
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
		bs, _ := ioutil.ReadAll(resp.Body)
		return nil, models.UnknownResponse{StatusCode: int64(resp.StatusCode), Body: string(bs)}
	}
}

// CancelWorkflow makes a DELETE request to /workflows/{workflowID}
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

	req, err := http.NewRequestWithContext(ctx, "DELETE", path, bytes.NewBuffer(body))

	if err != nil {
		return err
	}

	return c.doCancelWorkflowRequest(ctx, req, headers)
}

func (c *WagClient) doCancelWorkflowRequest(ctx context.Context, req *http.Request, headers map[string]string) error {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Canonical-Resource", "CancelWorkflow")
	req.Header.Set(VersionHeader, Version)

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

	resp, err := c.requestDoer.Do(c.client, req)
	retCode := 0
	if resp != nil {
		retCode = resp.StatusCode
	}

	// log all client failures and non-successful HT
	logData := map[string]interface{}{
		"backend":     "workflow-manager",
		"method":      req.Method,
		"uri":         req.URL,
		"status_code": retCode,
	}
	if err == nil && retCode > 399 {
		logData["message"] = resp.Status
		c.logger.Log(wcl.Error, "client-request-finished", logData)
	}
	if err != nil {
		logData["message"] = err.Error()
		c.logger.Log(wcl.Error, "client-request-finished", logData)
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
		bs, _ := ioutil.ReadAll(resp.Body)
		return models.UnknownResponse{StatusCode: int64(resp.StatusCode), Body: string(bs)}
	}
}

// GetWorkflowByID makes a GET request to /workflows/{workflowID}
//
// 200: *models.Workflow
// 400: *models.BadRequest
// 404: *models.NotFound
// 500: *models.InternalError
// default: client side HTTP errors, for example: context.DeadlineExceeded.
func (c *WagClient) GetWorkflowByID(ctx context.Context, workflowID string) (*models.Workflow, error) {
	headers := make(map[string]string)

	var body []byte
	path, err := models.GetWorkflowByIDInputPath(workflowID)

	if err != nil {
		return nil, err
	}

	path = c.basePath + path

	req, err := http.NewRequestWithContext(ctx, "GET", path, bytes.NewBuffer(body))

	if err != nil {
		return nil, err
	}

	return c.doGetWorkflowByIDRequest(ctx, req, headers)
}

func (c *WagClient) doGetWorkflowByIDRequest(ctx context.Context, req *http.Request, headers map[string]string) (*models.Workflow, error) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Canonical-Resource", "getWorkflowByID")
	req.Header.Set(VersionHeader, Version)

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

	resp, err := c.requestDoer.Do(c.client, req)
	retCode := 0
	if resp != nil {
		retCode = resp.StatusCode
	}

	// log all client failures and non-successful HT
	logData := map[string]interface{}{
		"backend":     "workflow-manager",
		"method":      req.Method,
		"uri":         req.URL,
		"status_code": retCode,
	}
	if err == nil && retCode > 399 {
		logData["message"] = resp.Status
		c.logger.Log(wcl.Error, "client-request-finished", logData)
	}
	if err != nil {
		logData["message"] = err.Error()
		c.logger.Log(wcl.Error, "client-request-finished", logData)
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
		bs, _ := ioutil.ReadAll(resp.Body)
		return nil, models.UnknownResponse{StatusCode: int64(resp.StatusCode), Body: string(bs)}
	}
}

// ResumeWorkflowByID makes a POST request to /workflows/{workflowID}
//
// 200: *models.Workflow
// 400: *models.BadRequest
// 404: *models.NotFound
// 500: *models.InternalError
// default: client side HTTP errors, for example: context.DeadlineExceeded.
func (c *WagClient) ResumeWorkflowByID(ctx context.Context, i *models.ResumeWorkflowByIDInput) (*models.Workflow, error) {
	headers := make(map[string]string)

	var body []byte
	path, err := i.Path()

	if err != nil {
		return nil, err
	}

	path = c.basePath + path

	if i.Overrides != nil {

		var err error
		body, err = json.Marshal(i.Overrides)

		if err != nil {
			return nil, err
		}

	}

	req, err := http.NewRequestWithContext(ctx, "POST", path, bytes.NewBuffer(body))

	if err != nil {
		return nil, err
	}

	return c.doResumeWorkflowByIDRequest(ctx, req, headers)
}

func (c *WagClient) doResumeWorkflowByIDRequest(ctx context.Context, req *http.Request, headers map[string]string) (*models.Workflow, error) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Canonical-Resource", "resumeWorkflowByID")
	req.Header.Set(VersionHeader, Version)

	for field, value := range headers {
		req.Header.Set(field, value)
	}

	// Add the opname for doers like tracing
	ctx = context.WithValue(ctx, opNameCtx{}, "resumeWorkflowByID")
	req = req.WithContext(ctx)
	// Don't add the timeout in a "doer" because we don't want to call "defer.cancel()"
	// until we've finished all the processing of the request object. Otherwise we'll cancel
	// our own request before we've finished it.
	if c.defaultTimeout != 0 {
		ctx, cancel := context.WithTimeout(req.Context(), c.defaultTimeout)
		defer cancel()
		req = req.WithContext(ctx)
	}

	resp, err := c.requestDoer.Do(c.client, req)
	retCode := 0
	if resp != nil {
		retCode = resp.StatusCode
	}

	// log all client failures and non-successful HT
	logData := map[string]interface{}{
		"backend":     "workflow-manager",
		"method":      req.Method,
		"uri":         req.URL,
		"status_code": retCode,
	}
	if err == nil && retCode > 399 {
		logData["message"] = resp.Status
		c.logger.Log(wcl.Error, "client-request-finished", logData)
	}
	if err != nil {
		logData["message"] = err.Error()
		c.logger.Log(wcl.Error, "client-request-finished", logData)
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
		bs, _ := ioutil.ReadAll(resp.Body)
		return nil, models.UnknownResponse{StatusCode: int64(resp.StatusCode), Body: string(bs)}
	}
}

// ResolveWorkflowByID makes a POST request to /workflows/{workflowID}/resolved
//
// 201: nil
// 400: *models.BadRequest
// 404: *models.NotFound
// 409: *models.Conflict
// 500: *models.InternalError
// default: client side HTTP errors, for example: context.DeadlineExceeded.
func (c *WagClient) ResolveWorkflowByID(ctx context.Context, workflowID string) error {
	headers := make(map[string]string)

	var body []byte
	path, err := models.ResolveWorkflowByIDInputPath(workflowID)

	if err != nil {
		return err
	}

	path = c.basePath + path

	req, err := http.NewRequestWithContext(ctx, "POST", path, bytes.NewBuffer(body))

	if err != nil {
		return err
	}

	return c.doResolveWorkflowByIDRequest(ctx, req, headers)
}

func (c *WagClient) doResolveWorkflowByIDRequest(ctx context.Context, req *http.Request, headers map[string]string) error {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Canonical-Resource", "resolveWorkflowByID")
	req.Header.Set(VersionHeader, Version)

	for field, value := range headers {
		req.Header.Set(field, value)
	}

	// Add the opname for doers like tracing
	ctx = context.WithValue(ctx, opNameCtx{}, "resolveWorkflowByID")
	req = req.WithContext(ctx)
	// Don't add the timeout in a "doer" because we don't want to call "defer.cancel()"
	// until we've finished all the processing of the request object. Otherwise we'll cancel
	// our own request before we've finished it.
	if c.defaultTimeout != 0 {
		ctx, cancel := context.WithTimeout(req.Context(), c.defaultTimeout)
		defer cancel()
		req = req.WithContext(ctx)
	}

	resp, err := c.requestDoer.Do(c.client, req)
	retCode := 0
	if resp != nil {
		retCode = resp.StatusCode
	}

	// log all client failures and non-successful HT
	logData := map[string]interface{}{
		"backend":     "workflow-manager",
		"method":      req.Method,
		"uri":         req.URL,
		"status_code": retCode,
	}
	if err == nil && retCode > 399 {
		logData["message"] = resp.Status
		c.logger.Log(wcl.Error, "client-request-finished", logData)
	}
	if err != nil {
		logData["message"] = err.Error()
		c.logger.Log(wcl.Error, "client-request-finished", logData)
		return err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {

	case 201:

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

	case 409:

		var output models.Conflict
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
		bs, _ := ioutil.ReadAll(resp.Body)
		return models.UnknownResponse{StatusCode: int64(resp.StatusCode), Body: string(bs)}
	}
}

func shortHash(s string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(s)))[0:6]
}
