package services

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockDialer struct {
	err error
}

func (m *mockDialer) DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	if m.err != nil {
		return nil, m.err
	}
	conn1, conn2 := net.Pipe()
	_ = conn2.Close()
	return conn1, nil
}

type MockLBProxyAdapter struct {
	mock.Mock
}

func (m *MockLBProxyAdapter) DeployProxy(ctx context.Context, lb *domain.LoadBalancer, targets []*domain.LBTarget) (string, error) {
	args := m.Called(ctx, lb, targets)
	return args.String(0), args.Error(1)
}

func (m *MockLBProxyAdapter) RemoveProxy(ctx context.Context, lbID uuid.UUID) error {
	return m.Called(ctx, lbID).Error(0)
}

func (m *MockLBProxyAdapter) UpdateProxyConfig(ctx context.Context, lb *domain.LoadBalancer, targets []*domain.LBTarget) error {
	return m.Called(ctx, lb, targets).Error(0)
}

// Since we are in package services, we need to mock Repos again locally since
// MockLBRepo in shared_test.go is in package services_test.
// Or we can alias them if they were exported, but they are not exported from services_test.
// So we must define local mocks or move this test to services_test and export methods in main code.

// Redefining mocks here locally for white-box testing.

type mockLBRepo struct {
	mock.Mock
}

func (m *mockLBRepo) Create(ctx context.Context, lb *domain.LoadBalancer) error {
	return m.Called(ctx, lb).Error(0)
}
func (m *mockLBRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.LoadBalancer, error) {
	args := m.Called(ctx, id)
	r0, _ := args.Get(0).(*domain.LoadBalancer)
	return r0, args.Error(1)
}
func (m *mockLBRepo) GetByName(ctx context.Context, name string) (*domain.LoadBalancer, error) {
	args := m.Called(ctx, name)
	r0, _ := args.Get(0).(*domain.LoadBalancer)
	return r0, args.Error(1)
}
func (m *mockLBRepo) GetByIdempotencyKey(ctx context.Context, key string) (*domain.LoadBalancer, error) {
	args := m.Called(ctx, key)
	r0, _ := args.Get(0).(*domain.LoadBalancer)
	return r0, args.Error(1)
}
func (m *mockLBRepo) List(ctx context.Context) ([]*domain.LoadBalancer, error) {
	args := m.Called(ctx)
	r0, _ := args.Get(0).([]*domain.LoadBalancer)
	return r0, args.Error(1)
}
func (m *mockLBRepo) ListAll(ctx context.Context) ([]*domain.LoadBalancer, error) {
	args := m.Called(ctx)
	r0, _ := args.Get(0).([]*domain.LoadBalancer)
	return r0, args.Error(1)
}
func (m *mockLBRepo) ListByStatus(ctx context.Context, status string, limit, offset int) ([]*domain.LoadBalancer, error) {
	args := m.Called(ctx, status, limit, offset)
	r0, _ := args.Get(0).([]*domain.LoadBalancer)
	return r0, args.Error(1)
}
func (m *mockLBRepo) Update(ctx context.Context, lb *domain.LoadBalancer) error {
	return m.Called(ctx, lb).Error(0)
}
func (m *mockLBRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *mockLBRepo) AddTarget(ctx context.Context, target *domain.LBTarget) error {
	return m.Called(ctx, target).Error(0)
}
func (m *mockLBRepo) RemoveTarget(ctx context.Context, lbID, instanceID uuid.UUID) error {
	return m.Called(ctx, lbID, instanceID).Error(0)
}
func (m *mockLBRepo) ListTargets(ctx context.Context, lbID uuid.UUID) ([]*domain.LBTarget, error) {
	args := m.Called(ctx, lbID)
	r0, _ := args.Get(0).([]*domain.LBTarget)
	return r0, args.Error(1)
}
func (m *mockLBRepo) UpdateTargetHealth(ctx context.Context, lbID, instanceID uuid.UUID, status string) error {
	return m.Called(ctx, lbID, instanceID, status).Error(0)
}
func (m *mockLBRepo) GetTargetsForInstance(ctx context.Context, instanceID uuid.UUID) ([]*domain.LBTarget, error) {
	args := m.Called(ctx, instanceID)
	r0, _ := args.Get(0).([]*domain.LBTarget)
	return r0, args.Error(1)
}

type mockInstRepo struct {
	mock.Mock
}

func (m *mockInstRepo) Create(ctx context.Context, inst *domain.Instance) error {
	return m.Called(ctx, inst).Error(0)
}
func (m *mockInstRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Instance, error) {
	args := m.Called(ctx, id)
	r0, _ := args.Get(0).(*domain.Instance)
	return r0, args.Error(1)
}
func (m *mockInstRepo) GetByName(ctx context.Context, name string) (*domain.Instance, error) {
	args := m.Called(ctx, name)
	r0, _ := args.Get(0).(*domain.Instance)
	return r0, args.Error(1)
}
func (m *mockInstRepo) List(ctx context.Context, tagFilter []string) ([]*domain.Instance, error) {
	args := m.Called(ctx, tagFilter)
	r0, _ := args.Get(0).([]*domain.Instance)
	return r0, args.Error(1)
}
func (m *mockInstRepo) ListAll(ctx context.Context) ([]*domain.Instance, error) {
	args := m.Called(ctx)
	r0, _ := args.Get(0).([]*domain.Instance)
	return r0, args.Error(1)
}
func (m *mockInstRepo) ListBySubnet(ctx context.Context, id uuid.UUID) ([]*domain.Instance, error) {
	args := m.Called(ctx, id)
	r0, _ := args.Get(0).([]*domain.Instance)
	return r0, args.Error(1)
}
func (m *mockInstRepo) Update(ctx context.Context, inst *domain.Instance) error {
	return m.Called(ctx, inst).Error(0)
}
func (m *mockInstRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

func TestLBWorkerProcessCreatingLBs(t *testing.T) {
	t.Parallel()
	lbRepo := new(mockLBRepo)
	instRepo := new(mockInstRepo)
	proxy := new(MockLBProxyAdapter)
	worker := NewLBWorker(lbRepo, instRepo, proxy)

	ctx := context.Background()
	lbID := uuid.New()
	userID := uuid.New()
	lb := &domain.LoadBalancer{
		ID:     lbID,
		UserID: userID,
		Status: domain.LBStatusCreating,
	}

	lbRepo.On("ListByStatus", mock.Anything, string(domain.LBStatusCreating), 100, 0).Return([]*domain.LoadBalancer{lb}, nil)
	lbRepo.On("ListTargets", mock.Anything, lbID).Return([]*domain.LBTarget{}, nil)
	proxy.On("DeployProxy", mock.Anything, lb, []*domain.LBTarget{}).Return("http://lb-url", nil)
	lbRepo.On("Update", mock.Anything, mock.MatchedBy(func(l *domain.LoadBalancer) bool {
		return l.ID == lbID && l.Status == domain.LBStatusActive
	})).Return(nil)

	worker.processCreatingLBs(ctx)

	lbRepo.AssertExpectations(t)
	proxy.AssertExpectations(t)
}

func TestLBWorkerProcessDeletingLBs(t *testing.T) {
	t.Parallel()
	lbRepo := new(mockLBRepo)
	instRepo := new(mockInstRepo)
	proxy := new(MockLBProxyAdapter)
	worker := NewLBWorker(lbRepo, instRepo, proxy)

	ctx := context.Background()
	lbID := uuid.New()
	lb := &domain.LoadBalancer{
		ID:     lbID,
		UserID: uuid.New(),
		Status: domain.LBStatusDeleted,
	}

	lbRepo.On("ListByStatus", mock.Anything, string(domain.LBStatusDeleted), 100, 0).Return([]*domain.LoadBalancer{lb}, nil)
	proxy.On("RemoveProxy", mock.Anything, lbID).Return(nil)
	lbRepo.On("Delete", mock.Anything, lbID).Return(nil)

	worker.processDeletingLBs(ctx)

	lbRepo.AssertExpectations(t)
	proxy.AssertExpectations(t)
}

func TestLBWorkerProcessActiveLBs(t *testing.T) {
	t.Parallel()
	lbRepo := new(mockLBRepo)
	instRepo := new(mockInstRepo)
	proxy := new(MockLBProxyAdapter)
	worker := NewLBWorker(lbRepo, instRepo, proxy)

	ctx := context.Background()
	lbID := uuid.New()
	lb := &domain.LoadBalancer{
		ID:     lbID,
		UserID: uuid.New(),
		Status: domain.LBStatusActive,
	}

	lbRepo.On("ListByStatus", mock.Anything, string(domain.LBStatusActive), 100, 0).Return([]*domain.LoadBalancer{lb}, nil)
	lbRepo.On("ListTargets", mock.Anything, lbID).Return([]*domain.LBTarget{}, nil)
	proxy.On("UpdateProxyConfig", mock.Anything, lb, []*domain.LBTarget{}).Return(nil)

	worker.processActiveLBs(ctx)

	proxy.AssertExpectations(t)
}

func TestLBWorkerProcessHealthChecks(t *testing.T) {
	t.Parallel()
	lbRepo := new(mockLBRepo)
	instRepo := new(mockInstRepo)
	proxy := new(MockLBProxyAdapter)
	worker := NewLBWorker(lbRepo, instRepo, proxy)

	ctx := context.Background()
	lbID := uuid.New()
	instID := uuid.New()
	lb := &domain.LoadBalancer{
		ID:     lbID,
		UserID: uuid.New(),
		Status: domain.LBStatusActive,
	}

	target := &domain.LBTarget{
		InstanceID: instID,
		Port:       80,
		Health:     "unhealthy",
	}

	inst := &domain.Instance{
		ID:    instID,
		Ports: "80:8080",
	}

	lbRepo.On("ListByStatus", mock.Anything, string(domain.LBStatusActive), 100, 0).Return([]*domain.LoadBalancer{lb}, nil)
	lbRepo.On("ListTargets", mock.Anything, lbID).Return([]*domain.LBTarget{target}, nil)
	instRepo.On("GetByID", mock.Anything, instID).Return(inst, nil)

	// Since we are mocking, we cannot easily mock net.Dial from within isPortOpen since it's hardcoded.
	// But we can test that it TRIES to update if health status changes.
	// However, isPortOpen will likely fail (return false) since 8080 is not open locally.
	// So status stays "unhealthy". UpdateTargetHealth won't be called.

	// If we want to test "healthy" transition we need DI for dialer or logic separation.
	// For now, let's test that it runs without error.

	worker.processHealthChecks(ctx)

	// If we want to verify healthy check, we can maybe simulate it, but tough without refactoring.
}

func TestLBWorkerIsPortOpen(t *testing.T) {
	t.Parallel()
	lbRepo := new(mockLBRepo)
	instRepo := new(mockInstRepo)
	proxy := new(MockLBProxyAdapter)
	worker := NewLBWorker(lbRepo, instRepo, proxy)

	worker.dialer = &mockDialer{}
	assert.True(t, worker.isPortOpen("8080"))

	worker.dialer = &mockDialer{err: fmt.Errorf("dial failed")}
	assert.False(t, worker.isPortOpen("8080"))
}

func TestLBWorkerCheckTargetHealthUpdates(t *testing.T) {
	t.Parallel()
	lbRepo := new(mockLBRepo)
	instRepo := new(mockInstRepo)
	proxy := new(MockLBProxyAdapter)
	worker := NewLBWorker(lbRepo, instRepo, proxy)
	worker.dialer = &mockDialer{}

	ctx := context.Background()
	lbID := uuid.New()
	instID := uuid.New()

	instRepo.On("GetByID", ctx, instID).Return(&domain.Instance{ID: instID, Ports: "8080:80"}, nil)
	lbRepo.On("UpdateTargetHealth", ctx, lbID, instID, "healthy").Return(nil).Once()

	changed := worker.checkTargetHealth(ctx, &domain.LoadBalancer{ID: lbID}, &domain.LBTarget{
		InstanceID: instID,
		Port:       80,
		Health:     "unhealthy",
	})

	assert.True(t, changed)
	lbRepo.AssertExpectations(t)
}

func TestRealDialerDialTimeout(t *testing.T) {
	t.Parallel()
	d := &realDialer{}
	conn, err := d.DialTimeout("tcp", "127.0.0.1:0", 10*time.Millisecond)
	if err == nil {
		_ = conn.Close()
	}
	assert.Error(t, err)
}

func TestLBWorkerRun(t *testing.T) {
	t.Parallel()
	lbRepo := new(mockLBRepo)
	instRepo := new(mockInstRepo)
	proxy := new(MockLBProxyAdapter)
	worker := NewLBWorker(lbRepo, instRepo, proxy)

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)

	cancel()
	worker.Run(ctx, &wg)
	wg.Wait()
}
