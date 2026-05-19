package ws

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type mockIdentityService struct {
	mock.Mock
}

func (m *mockIdentityService) CreateKey(ctx context.Context, userID uuid.UUID, name string) (*domain.APIKey, error) {
	args := m.Called(ctx, userID, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.APIKey)
	return r0, args.Error(1)
}

func (m *mockIdentityService) ValidateAPIKey(ctx context.Context, key string) (*domain.APIKey, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.APIKey)
	return r0, args.Error(1)
}
func (m *mockIdentityService) GetAPIKeyByID(ctx context.Context, id uuid.UUID) (*domain.APIKey, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.APIKey)
	return r0, args.Error(1)
}
func (m *mockIdentityService) ListKeys(ctx context.Context, userID uuid.UUID) ([]*domain.APIKey, error) {
	args := m.Called(ctx, userID)
	r0, _ := args.Get(0).([]*domain.APIKey)
	return r0, args.Error(1)
}
func (m *mockIdentityService) RevokeKey(ctx context.Context, userID, id uuid.UUID) error {
	args := m.Called(ctx, userID, id)
	return args.Error(0)
}
func (m *mockIdentityService) RotateKey(ctx context.Context, userID, id uuid.UUID) (*domain.APIKey, error) {
	args := m.Called(ctx, userID, id)
	r0, _ := args.Get(0).(*domain.APIKey)
	return r0, args.Error(1)
}

func (m *mockIdentityService) CreateServiceAccount(ctx context.Context, tenantID uuid.UUID, name, role string) (*domain.ServiceAccountWithSecret, error) {
	args := m.Called(ctx, tenantID, name, role)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.ServiceAccountWithSecret)
	return r0, args.Error(1)
}

func (m *mockIdentityService) GetServiceAccount(ctx context.Context, id uuid.UUID) (*domain.ServiceAccount, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.ServiceAccount)
	return r0, args.Error(1)
}

func (m *mockIdentityService) ListServiceAccounts(ctx context.Context, tenantID uuid.UUID) ([]*domain.ServiceAccount, error) {
	args := m.Called(ctx, tenantID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).([]*domain.ServiceAccount)
	return r0, args.Error(1)
}

func (m *mockIdentityService) UpdateServiceAccount(ctx context.Context, sa *domain.ServiceAccount) error {
	args := m.Called(ctx, sa)
	return args.Error(0)
}

func (m *mockIdentityService) DeleteServiceAccount(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockIdentityService) ValidateClientCredentials(ctx context.Context, clientID, clientSecret string) (string, error) {
	args := m.Called(ctx, clientID, clientSecret)
	return args.String(0), args.Error(1)
}

func (m *mockIdentityService) ValidateAccessToken(ctx context.Context, token string) (*domain.ServiceAccountClaims, error) {
	args := m.Called(ctx, token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.ServiceAccountClaims)
	return r0, args.Error(1)
}

func (m *mockIdentityService) RotateServiceAccountSecret(ctx context.Context, saID uuid.UUID) (string, error) {
	args := m.Called(ctx, saID)
	return args.String(0), args.Error(1)
}

func (m *mockIdentityService) RevokeServiceAccountSecret(ctx context.Context, saID, secretID uuid.UUID) error {
	args := m.Called(ctx, saID, secretID)
	return args.Error(0)
}

func (m *mockIdentityService) ListServiceAccountSecrets(ctx context.Context, saID uuid.UUID) ([]*domain.ServiceAccountSecret, error) {
	args := m.Called(ctx, saID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).([]*domain.ServiceAccountSecret)
	return r0, args.Error(1)
}

func (m *mockIdentityService) TokenTTL() time.Duration {
	return time.Hour
}

func TestWebSocketLifecycle(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	hub := NewHub(logger)
	go hub.Run()

	mockID := new(mockIdentityService)
	// Use "*" so the existing dialer test (which doesn't set Origin) is
	// unaffected by the fail-closed default introduced for #249.
	handler := NewHandler(hub, mockID, logger, "*")

	r := gin.New()
	r.GET("/ws", handler.ServeWS)

	server := httptest.NewServer(r)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws?api_key=valid-key"
	mockID.On("ValidateAPIKey", mock.Anything, "valid-key").Return(&domain.APIKey{Key: "valid-key", UserID: uuid.New()}, nil)

	dialer := websocket.Dialer{}
	conn, resp, err := dialer.Dial(wsURL, nil)
	require.NoError(t, err)
	if resp != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	defer func() { _ = conn.Close() }()

	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 1, hub.ClientCount())

	event := &domain.WSEvent{Type: domain.WSEventInstanceCreated, Timestamp: time.Now()}
	hub.BroadcastEvent(event)

	_, p, err := conn.ReadMessage()
	require.NoError(t, err)
	assert.Contains(t, string(p), "INSTANCE_CREATED")

	_ = conn.Close()
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 0, hub.ClientCount())
}

func TestWebSocketAuthFailure(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	hub := NewHub(logger)
	mockID := new(mockIdentityService)
	handler := NewHandler(hub, mockID, logger, "*")

	r := gin.New()
	r.GET("/ws", handler.ServeWS)

	server := httptest.NewServer(r)
	defer server.Close()

	t.Run("Missing API Key", func(t *testing.T) {
		wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
		dialer := websocket.Dialer{}
		_, resp, err := dialer.Dial(wsURL, nil)
		require.Error(t, err)
		if resp != nil {
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
			_ = resp.Body.Close()
		}
	})

	t.Run("Invalid API Key", func(t *testing.T) {
		wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws?api_key=invalid"
		mockID.On("ValidateAPIKey", mock.Anything, "invalid").Return(nil, errors.New(errors.Unauthorized, "invalid key"))
		dialer := websocket.Dialer{}
		_, resp, err := dialer.Dial(wsURL, nil)
		require.Error(t, err)
		if resp != nil {
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
			_ = resp.Body.Close()
		}
	})
}

func TestHubShutdown(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	hub := NewHub(logger)
	go hub.Run()

	hub.Stop()

	select {
	case <-hub.Stopped():
	case <-time.After(time.Second):
		t.Fatal("hub.Run() did not exit within 1s after Stop()")
	}
}

func TestHubStopIdempotent(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	hub := NewHub(logger)
	go hub.Run()

	hub.Stop()
	hub.Stop() // safe to call twice
	hub.Stop() // and again

	select {
	case <-hub.Stopped():
	case <-time.After(time.Second):
		t.Fatal("hub.Run() did not exit within 1s after Stop()")
	}
}
