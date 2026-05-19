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

func (c *computeNoOpReset) LaunchInstanceWithOptions(ctx context.Context, opts ports.CreateInstanceOptions) (string, []string, error) {
	return "", nil, nil
}
func (c *computeNoOpReset) StartInstance(ctx context.Context, id string) error  { return nil }
func (c *computeNoOpReset) StopInstance(ctx context.Context, id string) error   { return nil }
func (c *computeNoOpReset) PauseInstance(ctx context.Context, id string) error  { return nil }
func (c *computeNoOpReset) ResumeInstance(ctx context.Context, id string) error { return nil }
func (c *computeNoOpReset) DeleteInstance(ctx context.Context, id string) error { return nil }
func (c *computeNoOpReset) GetInstanceLogs(ctx context.Context, id string) (io.ReadCloser, error) {
	return nil, nil
}
func (c *computeNoOpReset) GetInstanceStats(ctx context.Context, id string) (io.ReadCloser, error) {
	return nil, nil
}
func (c *computeNoOpReset) GetInstancePort(ctx context.Context, id string, internalPort string) (int, error) {
	return 0, nil
}
func (c *computeNoOpReset) GetInstanceIP(ctx context.Context, id string) (string, error) {
	return "", nil
}
func (c *computeNoOpReset) GetConsoleURL(ctx context.Context, id string) (string, error) {
	return "", nil
}
func (c *computeNoOpReset) Exec(ctx context.Context, id string, cmd []string) (string, error) {
	return "", nil
}
func (c *computeNoOpReset) RunTask(ctx context.Context, opts ports.RunTaskOptions) (string, []string, error) {
	return "", nil, nil
}
func (c *computeNoOpReset) WaitTask(ctx context.Context, id string) (int64, error) { return 0, nil }
func (c *computeNoOpReset) CreateNetwork(ctx context.Context, name string) (string, error) {
	return "", nil
}
func (c *computeNoOpReset) DeleteNetwork(ctx context.Context, id string) error { return nil }
func (c *computeNoOpReset) AttachVolume(ctx context.Context, id, volumePath string) (string, string, error) {
	return "", "", nil
}
func (c *computeNoOpReset) DetachVolume(ctx context.Context, id, volumePath string) (string, error) {
	return "", nil
}
func (c *computeNoOpReset) Ping(ctx context.Context) error { return nil }
func (c *computeNoOpReset) Type() string                   { return "" }
func (c *computeNoOpReset) ResizeInstance(ctx context.Context, id string, cpu, memory int64) error {
	return nil
}
func (c *computeNoOpReset) CreateSnapshot(ctx context.Context, id, name string) error  { return nil }
func (c *computeNoOpReset) RestoreSnapshot(ctx context.Context, id, name string) error { return nil }
func (c *computeNoOpReset) DeleteSnapshot(ctx context.Context, id, name string) error  { return nil }
func (c *computeNoOpReset) ResetCircuitBreaker()                                       {}

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
