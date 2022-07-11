// Code generated by MockGen. DO NOT EDIT.
// Source: go.opentelemetry.io/otel/metric/instrument/asyncint64 (interfaces: InstrumentProvider)

// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	instrument "go.opentelemetry.io/otel/metric/instrument"
	asyncint64 "go.opentelemetry.io/otel/metric/instrument/asyncint64"
)

// MockInstrumentProvider is a mock of InstrumentProvider interface.
type MockInstrumentProvider struct {
	ctrl     *gomock.Controller
	recorder *MockInstrumentProviderMockRecorder
}

// MockInstrumentProviderMockRecorder is the mock recorder for MockInstrumentProvider.
type MockInstrumentProviderMockRecorder struct {
	mock *MockInstrumentProvider
}

// NewMockInstrumentProvider creates a new mock instance.
func NewMockInstrumentProvider(ctrl *gomock.Controller) *MockInstrumentProvider {
	mock := &MockInstrumentProvider{ctrl: ctrl}
	mock.recorder = &MockInstrumentProviderMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockInstrumentProvider) EXPECT() *MockInstrumentProviderMockRecorder {
	return m.recorder
}

// Counter mocks base method.
func (m *MockInstrumentProvider) Counter(arg0 string, arg1 ...instrument.Option) (asyncint64.Counter, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0}
	for _, a := range arg1 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Counter", varargs...)
	ret0, _ := ret[0].(asyncint64.Counter)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Counter indicates an expected call of Counter.
func (mr *MockInstrumentProviderMockRecorder) Counter(arg0 interface{}, arg1 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0}, arg1...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Counter", reflect.TypeOf((*MockInstrumentProvider)(nil).Counter), varargs...)
}

// Gauge mocks base method.
func (m *MockInstrumentProvider) Gauge(arg0 string, arg1 ...instrument.Option) (asyncint64.Gauge, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0}
	for _, a := range arg1 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Gauge", varargs...)
	ret0, _ := ret[0].(asyncint64.Gauge)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Gauge indicates an expected call of Gauge.
func (mr *MockInstrumentProviderMockRecorder) Gauge(arg0 interface{}, arg1 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0}, arg1...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Gauge", reflect.TypeOf((*MockInstrumentProvider)(nil).Gauge), varargs...)
}

// UpDownCounter mocks base method.
func (m *MockInstrumentProvider) UpDownCounter(arg0 string, arg1 ...instrument.Option) (asyncint64.UpDownCounter, error) {
	m.ctrl.T.Helper()
	varargs := []interface{}{arg0}
	for _, a := range arg1 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "UpDownCounter", varargs...)
	ret0, _ := ret[0].(asyncint64.UpDownCounter)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// UpDownCounter indicates an expected call of UpDownCounter.
func (mr *MockInstrumentProviderMockRecorder) UpDownCounter(arg0 interface{}, arg1 ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{arg0}, arg1...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpDownCounter", reflect.TypeOf((*MockInstrumentProvider)(nil).UpDownCounter), varargs...)
}
