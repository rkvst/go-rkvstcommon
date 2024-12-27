// Code generated by mockery v2.50.0. DO NOT EDIT.

package mocks

import (
	azservicebus "github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"
	azbus "github.com/datatrails/go-datatrails-common/azbus"

	context "context"

	mock "github.com/stretchr/testify/mock"
)

// BatchHandler is an autogenerated mock type for the BatchHandler type
type BatchHandler struct {
	mock.Mock
}

// Close provides a mock function with no fields
func (_m *BatchHandler) Close() {
	_m.Called()
}

// Handle provides a mock function with given fields: _a0, _a1, _a2
func (_m *BatchHandler) Handle(_a0 context.Context, _a1 azbus.Disposer, _a2 []*azservicebus.ReceivedMessage) error {
	ret := _m.Called(_a0, _a1, _a2)

	if len(ret) == 0 {
		panic("no return value specified for Handle")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, azbus.Disposer, []*azservicebus.ReceivedMessage) error); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Open provides a mock function with no fields
func (_m *BatchHandler) Open() error {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Open")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// NewBatchHandler creates a new instance of BatchHandler. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewBatchHandler(t interface {
	mock.TestingT
	Cleanup(func())
}) *BatchHandler {
	mock := &BatchHandler{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
