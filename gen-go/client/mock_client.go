// Automatically generated by MockGen. DO NOT EDIT!
// Source: interface.go

package client

import (
	context "context"
	models "github.com/Clever/workflow-manager/gen-go/models"
	gomock "github.com/golang/mock/gomock"
)

// MockClient is a mock of Client interface
type MockClient struct {
	ctrl     *gomock.Controller
	recorder *MockClientMockRecorder
}

// MockClientMockRecorder is the mock recorder for MockClient
type MockClientMockRecorder struct {
	mock *MockClient
}

// NewMockClient creates a new mock instance
func NewMockClient(ctrl *gomock.Controller) *MockClient {
	mock := &MockClient{ctrl: ctrl}
	mock.recorder = &MockClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (_m *MockClient) EXPECT() *MockClientMockRecorder {
	return _m.recorder
}

// HealthCheck mocks base method
func (_m *MockClient) HealthCheck(ctx context.Context) error {
	ret := _m.ctrl.Call(_m, "HealthCheck", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// HealthCheck indicates an expected call of HealthCheck
func (_mr *MockClientMockRecorder) HealthCheck(arg0 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "HealthCheck", arg0)
}

// PostStateResource mocks base method
func (_m *MockClient) PostStateResource(ctx context.Context, i *models.NewStateResource) (*models.StateResource, error) {
	ret := _m.ctrl.Call(_m, "PostStateResource", ctx, i)
	ret0, _ := ret[0].(*models.StateResource)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// PostStateResource indicates an expected call of PostStateResource
func (_mr *MockClientMockRecorder) PostStateResource(arg0, arg1 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "PostStateResource", arg0, arg1)
}

// DeleteStateResource mocks base method
func (_m *MockClient) DeleteStateResource(ctx context.Context, i *models.DeleteStateResourceInput) error {
	ret := _m.ctrl.Call(_m, "DeleteStateResource", ctx, i)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteStateResource indicates an expected call of DeleteStateResource
func (_mr *MockClientMockRecorder) DeleteStateResource(arg0, arg1 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "DeleteStateResource", arg0, arg1)
}

// GetStateResource mocks base method
func (_m *MockClient) GetStateResource(ctx context.Context, i *models.GetStateResourceInput) (*models.StateResource, error) {
	ret := _m.ctrl.Call(_m, "GetStateResource", ctx, i)
	ret0, _ := ret[0].(*models.StateResource)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetStateResource indicates an expected call of GetStateResource
func (_mr *MockClientMockRecorder) GetStateResource(arg0, arg1 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "GetStateResource", arg0, arg1)
}

// PutStateResource mocks base method
func (_m *MockClient) PutStateResource(ctx context.Context, i *models.PutStateResourceInput) (*models.StateResource, error) {
	ret := _m.ctrl.Call(_m, "PutStateResource", ctx, i)
	ret0, _ := ret[0].(*models.StateResource)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// PutStateResource indicates an expected call of PutStateResource
func (_mr *MockClientMockRecorder) PutStateResource(arg0, arg1 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "PutStateResource", arg0, arg1)
}

// GetWorkflowDefinitions mocks base method
func (_m *MockClient) GetWorkflowDefinitions(ctx context.Context) ([]models.WorkflowDefinition, error) {
	ret := _m.ctrl.Call(_m, "GetWorkflowDefinitions", ctx)
	ret0, _ := ret[0].([]models.WorkflowDefinition)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetWorkflowDefinitions indicates an expected call of GetWorkflowDefinitions
func (_mr *MockClientMockRecorder) GetWorkflowDefinitions(arg0 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "GetWorkflowDefinitions", arg0)
}

// NewWorkflowDefinition mocks base method
func (_m *MockClient) NewWorkflowDefinition(ctx context.Context, i *models.NewWorkflowDefinitionRequest) (*models.WorkflowDefinition, error) {
	ret := _m.ctrl.Call(_m, "NewWorkflowDefinition", ctx, i)
	ret0, _ := ret[0].(*models.WorkflowDefinition)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// NewWorkflowDefinition indicates an expected call of NewWorkflowDefinition
func (_mr *MockClientMockRecorder) NewWorkflowDefinition(arg0, arg1 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "NewWorkflowDefinition", arg0, arg1)
}

// GetWorkflowDefinitionVersionsByName mocks base method
func (_m *MockClient) GetWorkflowDefinitionVersionsByName(ctx context.Context, i *models.GetWorkflowDefinitionVersionsByNameInput) ([]models.WorkflowDefinition, error) {
	ret := _m.ctrl.Call(_m, "GetWorkflowDefinitionVersionsByName", ctx, i)
	ret0, _ := ret[0].([]models.WorkflowDefinition)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetWorkflowDefinitionVersionsByName indicates an expected call of GetWorkflowDefinitionVersionsByName
func (_mr *MockClientMockRecorder) GetWorkflowDefinitionVersionsByName(arg0, arg1 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "GetWorkflowDefinitionVersionsByName", arg0, arg1)
}

// UpdateWorkflowDefinition mocks base method
func (_m *MockClient) UpdateWorkflowDefinition(ctx context.Context, i *models.UpdateWorkflowDefinitionInput) (*models.WorkflowDefinition, error) {
	ret := _m.ctrl.Call(_m, "UpdateWorkflowDefinition", ctx, i)
	ret0, _ := ret[0].(*models.WorkflowDefinition)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// UpdateWorkflowDefinition indicates an expected call of UpdateWorkflowDefinition
func (_mr *MockClientMockRecorder) UpdateWorkflowDefinition(arg0, arg1 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "UpdateWorkflowDefinition", arg0, arg1)
}

// GetWorkflowDefinitionByNameAndVersion mocks base method
func (_m *MockClient) GetWorkflowDefinitionByNameAndVersion(ctx context.Context, i *models.GetWorkflowDefinitionByNameAndVersionInput) (*models.WorkflowDefinition, error) {
	ret := _m.ctrl.Call(_m, "GetWorkflowDefinitionByNameAndVersion", ctx, i)
	ret0, _ := ret[0].(*models.WorkflowDefinition)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetWorkflowDefinitionByNameAndVersion indicates an expected call of GetWorkflowDefinitionByNameAndVersion
func (_mr *MockClientMockRecorder) GetWorkflowDefinitionByNameAndVersion(arg0, arg1 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "GetWorkflowDefinitionByNameAndVersion", arg0, arg1)
}

// GetWorkflows mocks base method
func (_m *MockClient) GetWorkflows(ctx context.Context, i *models.GetWorkflowsInput) ([]models.Workflow, error) {
	ret := _m.ctrl.Call(_m, "GetWorkflows", ctx, i)
	ret0, _ := ret[0].([]models.Workflow)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetWorkflows indicates an expected call of GetWorkflows
func (_mr *MockClientMockRecorder) GetWorkflows(arg0, arg1 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "GetWorkflows", arg0, arg1)
}

// NewGetWorkflowsIter mocks base method
func (_m *MockClient) NewGetWorkflowsIter(ctx context.Context, i *models.GetWorkflowsInput) (GetWorkflowsIter, error) {
	ret := _m.ctrl.Call(_m, "NewGetWorkflowsIter", ctx, i)
	ret0, _ := ret[0].(GetWorkflowsIter)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// NewGetWorkflowsIter indicates an expected call of NewGetWorkflowsIter
func (_mr *MockClientMockRecorder) NewGetWorkflowsIter(arg0, arg1 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "NewGetWorkflowsIter", arg0, arg1)
}

// StartWorkflow mocks base method
func (_m *MockClient) StartWorkflow(ctx context.Context, i *models.StartWorkflowRequest) (*models.Workflow, error) {
	ret := _m.ctrl.Call(_m, "StartWorkflow", ctx, i)
	ret0, _ := ret[0].(*models.Workflow)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// StartWorkflow indicates an expected call of StartWorkflow
func (_mr *MockClientMockRecorder) StartWorkflow(arg0, arg1 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "StartWorkflow", arg0, arg1)
}

// CancelWorkflow mocks base method
func (_m *MockClient) CancelWorkflow(ctx context.Context, i *models.CancelWorkflowInput) error {
	ret := _m.ctrl.Call(_m, "CancelWorkflow", ctx, i)
	ret0, _ := ret[0].(error)
	return ret0
}

// CancelWorkflow indicates an expected call of CancelWorkflow
func (_mr *MockClientMockRecorder) CancelWorkflow(arg0, arg1 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "CancelWorkflow", arg0, arg1)
}

// GetWorkflowByID mocks base method
func (_m *MockClient) GetWorkflowByID(ctx context.Context, workflowID string) (*models.Workflow, error) {
	ret := _m.ctrl.Call(_m, "GetWorkflowByID", ctx, workflowID)
	ret0, _ := ret[0].(*models.Workflow)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetWorkflowByID indicates an expected call of GetWorkflowByID
func (_mr *MockClientMockRecorder) GetWorkflowByID(arg0, arg1 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "GetWorkflowByID", arg0, arg1)
}

// ResumeWorkflowByID mocks base method
func (_m *MockClient) ResumeWorkflowByID(ctx context.Context, i *models.ResumeWorkflowByIDInput) (*models.Workflow, error) {
	ret := _m.ctrl.Call(_m, "ResumeWorkflowByID", ctx, i)
	ret0, _ := ret[0].(*models.Workflow)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ResumeWorkflowByID indicates an expected call of ResumeWorkflowByID
func (_mr *MockClientMockRecorder) ResumeWorkflowByID(arg0, arg1 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "ResumeWorkflowByID", arg0, arg1)
}

// MockGetWorkflowsIter is a mock of GetWorkflowsIter interface
type MockGetWorkflowsIter struct {
	ctrl     *gomock.Controller
	recorder *MockGetWorkflowsIterMockRecorder
}

// MockGetWorkflowsIterMockRecorder is the mock recorder for MockGetWorkflowsIter
type MockGetWorkflowsIterMockRecorder struct {
	mock *MockGetWorkflowsIter
}

// NewMockGetWorkflowsIter creates a new mock instance
func NewMockGetWorkflowsIter(ctrl *gomock.Controller) *MockGetWorkflowsIter {
	mock := &MockGetWorkflowsIter{ctrl: ctrl}
	mock.recorder = &MockGetWorkflowsIterMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (_m *MockGetWorkflowsIter) EXPECT() *MockGetWorkflowsIterMockRecorder {
	return _m.recorder
}

// Next mocks base method
func (_m *MockGetWorkflowsIter) Next(_param0 *models.Workflow) bool {
	ret := _m.ctrl.Call(_m, "Next", _param0)
	ret0, _ := ret[0].(bool)
	return ret0
}

// Next indicates an expected call of Next
func (_mr *MockGetWorkflowsIterMockRecorder) Next(arg0 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "Next", arg0)
}

// Err mocks base method
func (_m *MockGetWorkflowsIter) Err() error {
	ret := _m.ctrl.Call(_m, "Err")
	ret0, _ := ret[0].(error)
	return ret0
}

// Err indicates an expected call of Err
func (_mr *MockGetWorkflowsIterMockRecorder) Err() *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "Err")
}
