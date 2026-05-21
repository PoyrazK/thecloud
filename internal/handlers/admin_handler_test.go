package httphandlers

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockAdminComputeFull struct {
	mock.Mock
}

func (m *mockAdminComputeFull) ResetCircuitBreaker() {
	m.Called()
}

func (m *mockAdminComputeFull) LaunchInstanceWithOptions(ctx context.Context, opts ports.CreateInstanceOptions) (string, []string, error) {
	args := m.Called(ctx, opts)
	if args.Get(1) == nil {
		return args.String(0), nil, args.Error(2)
	}
	return args.String(0), args.Get(1).([]string), args.Error(2)
}
func (m *mockAdminComputeFull) StartInstance(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
func (m *mockAdminComputeFull) StopInstance(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
func (m *mockAdminComputeFull) PauseInstance(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
func (m *mockAdminComputeFull) ResumeInstance(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
func (m *mockAdminComputeFull) DeleteInstance(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
func (m *mockAdminComputeFull) GetInstanceLogs(ctx context.Context, id string) (io.ReadCloser, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(io.ReadCloser), args.Error(1)
}
func (m *mockAdminComputeFull) GetInstanceStats(ctx context.Context, id string) (io.ReadCloser, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(io.ReadCloser), args.Error(1)
}
func (m *mockAdminComputeFull) GetInstancePort(ctx context.Context, id string, internalPort string) (int, error) {
	args := m.Called(ctx, id, internalPort)
	return args.Int(0), args.Error(1)
}
func (m *mockAdminComputeFull) GetInstanceIP(ctx context.Context, id string) (string, error) {
	args := m.Called(ctx, id)
	return args.String(0), args.Error(1)
}
func (m *mockAdminComputeFull) GetConsoleURL(ctx context.Context, id string) (string, error) {
	args := m.Called(ctx, id)
	return args.String(0), args.Error(1)
}
func (m *mockAdminComputeFull) Exec(ctx context.Context, id string, cmd []string) (string, error) {
	args := m.Called(ctx, id, cmd)
	return args.String(0), args.Error(1)
}
func (m *mockAdminComputeFull) RunTask(ctx context.Context, opts ports.RunTaskOptions) (string, []string, error) {
	args := m.Called(ctx, opts)
	return args.String(0), args.Get(1).([]string), args.Error(2)
}
func (m *mockAdminComputeFull) WaitTask(ctx context.Context, id string) (int64, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(int64), args.Error(1)
}
func (m *mockAdminComputeFull) CreateNetwork(ctx context.Context, name string) (string, error) {
	args := m.Called(ctx, name)
	return args.String(0), args.Error(1)
}
func (m *mockAdminComputeFull) DeleteNetwork(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
func (m *mockAdminComputeFull) AttachVolume(ctx context.Context, id string, volumePath string) (string, string, error) {
	args := m.Called(ctx, id, volumePath)
	return args.String(0), args.String(1), args.Error(2)
}
func (m *mockAdminComputeFull) DetachVolume(ctx context.Context, id string, volumePath string) (string, error) {
	args := m.Called(ctx, id, volumePath)
	return args.String(0), args.Error(1)
}
func (m *mockAdminComputeFull) Ping(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}
func (m *mockAdminComputeFull) Type() string {
	args := m.Called()
	return args.String(0)
}
func (m *mockAdminComputeFull) ResizeInstance(ctx context.Context, id string, cpu, memory int64) error {
	args := m.Called(ctx, id, cpu, memory)
	return args.Error(0)
}
func (m *mockAdminComputeFull) CreateSnapshot(ctx context.Context, id, name string) error {
	args := m.Called(ctx, id, name)
	return args.Error(0)
}
func (m *mockAdminComputeFull) RestoreSnapshot(ctx context.Context, id, name string) error {
	args := m.Called(ctx, id, name)
	return args.Error(0)
}
func (m *mockAdminComputeFull) DeleteSnapshot(ctx context.Context, id, name string) error {
	args := m.Called(ctx, id, name)
	return args.Error(0)
}
func (m *mockAdminComputeFull) StartPoolInstance(ctx context.Context, opts ports.RunTaskOptions) (string, []string, error) {
	args := m.Called(ctx, opts)
	return args.String(0), args.Get(1).([]string), args.Error(2)
}
func (m *mockAdminComputeFull) ExecInInstance(ctx context.Context, id string, cmd []string) (string, error) {
	args := m.Called(ctx, id, cmd)
	return args.String(0), args.Error(1)
}
func (m *mockAdminComputeFull) GetInstanceReady(ctx context.Context, id string) (bool, error) {
	args := m.Called(ctx, id)
	return args.Bool(0), args.Error(1)
}

const adminPath = "/admin/reset-circuit-breakers"

func setupAdminHandlerTest() (*mockAdminComputeFull, *AdminHandler, *gin.Engine) {
	gin.SetMode(gin.TestMode)
	svc := new(mockAdminComputeFull)
	handler := NewAdminHandler(svc)
	r := gin.New()
	return svc, handler, r
}

func TestAdminHandlerResetCircuitBreakers_WithResetSupport(t *testing.T) {
	t.Parallel()
	svc, handler, r := setupAdminHandlerTest()
	r.POST(adminPath, handler.ResetCircuitBreakers)

	svc.On("ResetCircuitBreaker").Return().Once()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, adminPath, nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"reset":true`)
	svc.AssertExpectations(t)
}

type computeNoOpReset struct{}

func (c *computeNoOpReset) LaunchInstanceWithOptions(_ context.Context, _ ports.CreateInstanceOptions) (string, []string, error) {
	return "", nil, nil
}
func (c *computeNoOpReset) StartInstance(_ context.Context, _ string) error  { return nil }
func (c *computeNoOpReset) StopInstance(_ context.Context, _ string) error   { return nil }
func (c *computeNoOpReset) PauseInstance(_ context.Context, _ string) error  { return nil }
func (c *computeNoOpReset) ResumeInstance(_ context.Context, _ string) error { return nil }
func (c *computeNoOpReset) DeleteInstance(_ context.Context, _ string) error { return nil }
func (c *computeNoOpReset) GetInstanceLogs(_ context.Context, _ string) (io.ReadCloser, error) {
	return nil, nil
}
func (c *computeNoOpReset) GetInstanceStats(_ context.Context, _ string) (io.ReadCloser, error) {
	return nil, nil
}
func (c *computeNoOpReset) GetInstancePort(_ context.Context, _ string, _ string) (int, error) {
	return 0, nil
}
func (c *computeNoOpReset) GetInstanceIP(_ context.Context, _ string) (string, error) {
	return "", nil
}
func (c *computeNoOpReset) GetConsoleURL(_ context.Context, _ string) (string, error) {
	return "", nil
}
func (c *computeNoOpReset) Exec(_ context.Context, _ string, _ []string) (string, error) {
	return "", nil
}
func (c *computeNoOpReset) RunTask(_ context.Context, _ ports.RunTaskOptions) (string, []string, error) {
	return "", nil, nil
}
func (c *computeNoOpReset) WaitTask(_ context.Context, _ string) (int64, error) { return 0, nil }
func (c *computeNoOpReset) CreateNetwork(_ context.Context, _ string) (string, error) {
	return "", nil
}
func (c *computeNoOpReset) DeleteNetwork(_ context.Context, _ string) error { return nil }
func (c *computeNoOpReset) AttachVolume(_ context.Context, _, _ string) (string, string, error) {
	return "", "", nil
}
func (c *computeNoOpReset) DetachVolume(_ context.Context, _, _ string) (string, error) {
	return "", nil
}
func (c *computeNoOpReset) Ping(_ context.Context) error { return nil }
func (c *computeNoOpReset) Type() string                 { return "" }
func (c *computeNoOpReset) ResizeInstance(_ context.Context, _ string, _, _ int64) error {
	return nil
}
func (c *computeNoOpReset) CreateSnapshot(_ context.Context, _, _ string) error  { return nil }
func (c *computeNoOpReset) RestoreSnapshot(_ context.Context, _, _ string) error { return nil }
func (c *computeNoOpReset) DeleteSnapshot(_ context.Context, _, _ string) error  { return nil }
func (c *computeNoOpReset) StartPoolInstance(_ context.Context, _ ports.RunTaskOptions) (string, []string, error) {
	return "", nil, nil
}
func (c *computeNoOpReset) ExecInInstance(_ context.Context, _ string, _ []string) (string, error) {
	return "", nil
}
func (c *computeNoOpReset) GetInstanceReady(_ context.Context, _ string) (bool, error) {
	return true, nil
}
func (c *computeNoOpReset) ResetCircuitBreaker() {}

// TestAdminHandlerResetCircuitBreakers_NopImplementation verifies that when
// ResetCircuitBreaker is a no-op (backend doesn't support reset), the handler
// still returns 200 with {"reset":true}. The handler always calls the method.
func TestAdminHandlerResetCircuitBreakers_NopImplementation(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	compute := &computeNoOpReset{}
	handler := NewAdminHandler(compute)
	r := gin.New()
	r.POST(adminPath, handler.ResetCircuitBreakers)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, adminPath, nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"reset":true`)
}
