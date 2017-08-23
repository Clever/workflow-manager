// Code generated by MockGen. DO NOT EDIT.
// Source: interface.go

package server

import (
	context "context"
	models "github.com/Clever/workflow-manager/gen-go/models"
	gomock "github.com/golang/mock/gomock"
	reflect "reflect"
)

// MockController is a mock of Controller interface
type MockController struct {
	ctrl     *gomock.Controller
	recorder *MockControllerMockRecorder
}

// MockControllerMockRecorder is the mock recorder for MockController
type MockControllerMockRecorder struct {
	mock *MockController
}

// NewMockController creates a new mock instance
func NewMockController(ctrl *gomock.Controller) *MockController {
	mock := &MockController{ctrl: ctrl}
	mock.recorder = &MockControllerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (_m *MockController) EXPECT() *MockControllerMockRecorder {
	return _m.recorder
}

// HealthCheck mocks base method
func (_m *MockController) HealthCheck(ctx context.Context) error {
	ret := _m.ctrl.Call(_m, "HealthCheck", ctx)
	ret0, _ := ret[0].(error)
	return ret0
}

// HealthCheck indicates an expected call of HealthCheck
func (_mr *MockControllerMockRecorder) HealthCheck(arg0 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCallWithMethodType(_mr.mock, "HealthCheck", reflect.TypeOf((*MockController)(nil).HealthCheck), arg0)
}

// GetJobsForWorkflow mocks base method
func (_m *MockController) GetJobsForWorkflow(ctx context.Context, i *models.GetJobsForWorkflowInput) ([]models.Job, error) {
	ret := _m.ctrl.Call(_m, "GetJobsForWorkflow", ctx, i)
	ret0, _ := ret[0].([]models.Job)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetJobsForWorkflow indicates an expected call of GetJobsForWorkflow
func (_mr *MockControllerMockRecorder) GetJobsForWorkflow(arg0, arg1 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCallWithMethodType(_mr.mock, "GetJobsForWorkflow", reflect.TypeOf((*MockController)(nil).GetJobsForWorkflow), arg0, arg1)
}

// StartJobForWorkflow mocks base method
func (_m *MockController) StartJobForWorkflow(ctx context.Context, i *models.JobInput) (*models.Job, error) {
	ret := _m.ctrl.Call(_m, "StartJobForWorkflow", ctx, i)
	ret0, _ := ret[0].(*models.Job)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// StartJobForWorkflow indicates an expected call of StartJobForWorkflow
func (_mr *MockControllerMockRecorder) StartJobForWorkflow(arg0, arg1 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCallWithMethodType(_mr.mock, "StartJobForWorkflow", reflect.TypeOf((*MockController)(nil).StartJobForWorkflow), arg0, arg1)
}

// CancelJob mocks base method
func (_m *MockController) CancelJob(ctx context.Context, i *models.CancelJobInput) error {
	ret := _m.ctrl.Call(_m, "CancelJob", ctx, i)
	ret0, _ := ret[0].(error)
	return ret0
}

// CancelJob indicates an expected call of CancelJob
func (_mr *MockControllerMockRecorder) CancelJob(arg0, arg1 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCallWithMethodType(_mr.mock, "CancelJob", reflect.TypeOf((*MockController)(nil).CancelJob), arg0, arg1)
}

// GetJob mocks base method
func (_m *MockController) GetJob(ctx context.Context, jobId string) (*models.Job, error) {
	ret := _m.ctrl.Call(_m, "GetJob", ctx, jobId)
	ret0, _ := ret[0].(*models.Job)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetJob indicates an expected call of GetJob
func (_mr *MockControllerMockRecorder) GetJob(arg0, arg1 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCallWithMethodType(_mr.mock, "GetJob", reflect.TypeOf((*MockController)(nil).GetJob), arg0, arg1)
}

// PostStateResource mocks base method
func (_m *MockController) PostStateResource(ctx context.Context, i *models.NewStateResource) (*models.StateResource, error) {
	ret := _m.ctrl.Call(_m, "PostStateResource", ctx, i)
	ret0, _ := ret[0].(*models.StateResource)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// PostStateResource indicates an expected call of PostStateResource
func (_mr *MockControllerMockRecorder) PostStateResource(arg0, arg1 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCallWithMethodType(_mr.mock, "PostStateResource", reflect.TypeOf((*MockController)(nil).PostStateResource), arg0, arg1)
}

// DeleteStateResource mocks base method
func (_m *MockController) DeleteStateResource(ctx context.Context, i *models.DeleteStateResourceInput) error {
	ret := _m.ctrl.Call(_m, "DeleteStateResource", ctx, i)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteStateResource indicates an expected call of DeleteStateResource
func (_mr *MockControllerMockRecorder) DeleteStateResource(arg0, arg1 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCallWithMethodType(_mr.mock, "DeleteStateResource", reflect.TypeOf((*MockController)(nil).DeleteStateResource), arg0, arg1)
}

// GetStateResource mocks base method
func (_m *MockController) GetStateResource(ctx context.Context, i *models.GetStateResourceInput) (*models.StateResource, error) {
	ret := _m.ctrl.Call(_m, "GetStateResource", ctx, i)
	ret0, _ := ret[0].(*models.StateResource)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetStateResource indicates an expected call of GetStateResource
func (_mr *MockControllerMockRecorder) GetStateResource(arg0, arg1 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCallWithMethodType(_mr.mock, "GetStateResource", reflect.TypeOf((*MockController)(nil).GetStateResource), arg0, arg1)
}

// PutStateResource mocks base method
func (_m *MockController) PutStateResource(ctx context.Context, i *models.PutStateResourceInput) (*models.StateResource, error) {
	ret := _m.ctrl.Call(_m, "PutStateResource", ctx, i)
	ret0, _ := ret[0].(*models.StateResource)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// PutStateResource indicates an expected call of PutStateResource
func (_mr *MockControllerMockRecorder) PutStateResource(arg0, arg1 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCallWithMethodType(_mr.mock, "PutStateResource", reflect.TypeOf((*MockController)(nil).PutStateResource), arg0, arg1)
}

// GetWorkflows mocks base method
func (_m *MockController) GetWorkflows(ctx context.Context) ([]models.Workflow, error) {
	ret := _m.ctrl.Call(_m, "GetWorkflows", ctx)
	ret0, _ := ret[0].([]models.Workflow)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetWorkflows indicates an expected call of GetWorkflows
func (_mr *MockControllerMockRecorder) GetWorkflows(arg0 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCallWithMethodType(_mr.mock, "GetWorkflows", reflect.TypeOf((*MockController)(nil).GetWorkflows), arg0)
}

// NewWorkflow mocks base method
func (_m *MockController) NewWorkflow(ctx context.Context, i *models.NewWorkflowRequest) (*models.Workflow, error) {
	ret := _m.ctrl.Call(_m, "NewWorkflow", ctx, i)
	ret0, _ := ret[0].(*models.Workflow)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// NewWorkflow indicates an expected call of NewWorkflow
func (_mr *MockControllerMockRecorder) NewWorkflow(arg0, arg1 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCallWithMethodType(_mr.mock, "NewWorkflow", reflect.TypeOf((*MockController)(nil).NewWorkflow), arg0, arg1)
}

// GetWorkflowVersionsByName mocks base method
func (_m *MockController) GetWorkflowVersionsByName(ctx context.Context, i *models.GetWorkflowVersionsByNameInput) ([]models.Workflow, error) {
	ret := _m.ctrl.Call(_m, "GetWorkflowVersionsByName", ctx, i)
	ret0, _ := ret[0].([]models.Workflow)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetWorkflowVersionsByName indicates an expected call of GetWorkflowVersionsByName
func (_mr *MockControllerMockRecorder) GetWorkflowVersionsByName(arg0, arg1 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCallWithMethodType(_mr.mock, "GetWorkflowVersionsByName", reflect.TypeOf((*MockController)(nil).GetWorkflowVersionsByName), arg0, arg1)
}

// UpdateWorkflow mocks base method
func (_m *MockController) UpdateWorkflow(ctx context.Context, i *models.UpdateWorkflowInput) (*models.Workflow, error) {
	ret := _m.ctrl.Call(_m, "UpdateWorkflow", ctx, i)
	ret0, _ := ret[0].(*models.Workflow)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// UpdateWorkflow indicates an expected call of UpdateWorkflow
func (_mr *MockControllerMockRecorder) UpdateWorkflow(arg0, arg1 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCallWithMethodType(_mr.mock, "UpdateWorkflow", reflect.TypeOf((*MockController)(nil).UpdateWorkflow), arg0, arg1)
}

// GetWorkflowByNameAndVersion mocks base method
func (_m *MockController) GetWorkflowByNameAndVersion(ctx context.Context, i *models.GetWorkflowByNameAndVersionInput) (*models.Workflow, error) {
	ret := _m.ctrl.Call(_m, "GetWorkflowByNameAndVersion", ctx, i)
	ret0, _ := ret[0].(*models.Workflow)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetWorkflowByNameAndVersion indicates an expected call of GetWorkflowByNameAndVersion
func (_mr *MockControllerMockRecorder) GetWorkflowByNameAndVersion(arg0, arg1 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCallWithMethodType(_mr.mock, "GetWorkflowByNameAndVersion", reflect.TypeOf((*MockController)(nil).GetWorkflowByNameAndVersion), arg0, arg1)
}
