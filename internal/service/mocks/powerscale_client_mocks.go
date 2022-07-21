// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/dell/csm-metrics-powerscale/internal/service (interfaces: PowerScaleClient)

// Package mocks is a generated GoMock package.
package mocks

import (
	context "context"
	reflect "reflect"

	goisilon "github.com/dell/goisilon"
	gomock "github.com/golang/mock/gomock"
)

// MockPowerScaleClient is a mock of PowerScaleClient interface.
type MockPowerScaleClient struct {
	ctrl     *gomock.Controller
	recorder *MockPowerScaleClientMockRecorder
}

// MockPowerScaleClientMockRecorder is the mock recorder for MockPowerScaleClient.
type MockPowerScaleClientMockRecorder struct {
	mock *MockPowerScaleClient
}

// NewMockPowerScaleClient creates a new mock instance.
func NewMockPowerScaleClient(ctrl *gomock.Controller) *MockPowerScaleClient {
	mock := &MockPowerScaleClient{ctrl: ctrl}
	mock.recorder = &MockPowerScaleClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockPowerScaleClient) EXPECT() *MockPowerScaleClientMockRecorder {
	return m.recorder
}

// GetAllQuotas mocks base method.
func (m *MockPowerScaleClient) GetAllQuotas(arg0 context.Context) (goisilon.QuotaList, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetAllQuotas", arg0)
	ret0, _ := ret[0].(goisilon.QuotaList)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetAllQuotas indicates an expected call of GetAllQuotas.
func (mr *MockPowerScaleClientMockRecorder) GetAllQuotas(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAllQuotas", reflect.TypeOf((*MockPowerScaleClient)(nil).GetAllQuotas), arg0)
}

// GetFloatStatistics mocks base method.
func (m *MockPowerScaleClient) GetFloatStatistics(arg0 context.Context, arg1 []string) (goisilon.FloatStats, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetFloatStatistics", arg0, arg1)
	ret0, _ := ret[0].(goisilon.FloatStats)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetFloatStatistics indicates an expected call of GetFloatStatistics.
func (mr *MockPowerScaleClientMockRecorder) GetFloatStatistics(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetFloatStatistics", reflect.TypeOf((*MockPowerScaleClient)(nil).GetFloatStatistics), arg0, arg1)
}

// GetQuotaWithPath mocks base method.
func (m *MockPowerScaleClient) GetQuotaWithPath(arg0 context.Context, arg1 string) (goisilon.Quota, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetQuotaWithPath", arg0, arg1)
	ret0, _ := ret[0].(goisilon.Quota)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetQuotaWithPath indicates an expected call of GetQuotaWithPath.
func (mr *MockPowerScaleClientMockRecorder) GetQuotaWithPath(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetQuotaWithPath", reflect.TypeOf((*MockPowerScaleClient)(nil).GetQuotaWithPath), arg0, arg1)
}
