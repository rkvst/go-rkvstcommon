// Code generated by mockery. DO NOT EDIT.

package mocks

import (
	context "context"

	azservicebus "github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"

	mock "github.com/stretchr/testify/mock"
)

// Handler is an autogenerated mock type for the Handler type
type Handler struct {
	mock.Mock
}

type Handler_Expecter struct {
	mock *mock.Mock
}

func (_m *Handler) EXPECT() *Handler_Expecter {
	return &Handler_Expecter{mock: &_m.Mock}
}

// Handle provides a mock function with given fields: _a0, _a1
func (_m *Handler) Handle(_a0 context.Context, _a1 *azservicebus.ReceivedMessage) error {
	ret := _m.Called(_a0, _a1)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *azservicebus.ReceivedMessage) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Handler_Handle_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Handle'
type Handler_Handle_Call struct {
	*mock.Call
}

// Handle is a helper method to define mock.On call
//   - _a0 context.Context
//   - _a1 *azservicebus.ReceivedMessage
func (_e *Handler_Expecter) Handle(_a0 interface{}, _a1 interface{}) *Handler_Handle_Call {
	return &Handler_Handle_Call{Call: _e.mock.On("Handle", _a0, _a1)}
}

func (_c *Handler_Handle_Call) Run(run func(_a0 context.Context, _a1 *azservicebus.ReceivedMessage)) *Handler_Handle_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(*azservicebus.ReceivedMessage))
	})
	return _c
}

func (_c *Handler_Handle_Call) Return(_a0 error) *Handler_Handle_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *Handler_Handle_Call) RunAndReturn(run func(context.Context, *azservicebus.ReceivedMessage) error) *Handler_Handle_Call {
	_c.Call.Return(run)
	return _c
}

// NewHandler creates a new instance of Handler. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewHandler(t interface {
	mock.TestingT
	Cleanup(func())
}) *Handler {
	mock := &Handler{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
