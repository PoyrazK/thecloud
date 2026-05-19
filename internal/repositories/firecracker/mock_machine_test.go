//go:build linux

package firecracker

import (
	"context"

	"github.com/stretchr/testify/mock"
)

type mockFirecrackerMachine struct {
	mock.Mock
}

func (m *mockFirecrackerMachine) Start(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockFirecrackerMachine) Shutdown(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockFirecrackerMachine) StopVMM() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockFirecrackerMachine) Wait(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}
