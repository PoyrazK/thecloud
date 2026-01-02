package ws

import (
	"context"
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
)

type mockIdentityService struct {
	mock.Mock
}

func (m *mockIdentityService) CreateKey(ctx context.Context, userID uuid.UUID, name string) (*domain.ApiKey, error) {
	args := m.Called(ctx, userID, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ApiKey), args.Error(1)
}

func (m *mockIdentityService) ValidateApiKey(ctx context.Context, key string) (*domain.ApiKey, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ApiKey), args.Error(1)
}

func TestWebSocket_Lifecycle(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	hub := NewHub(logger)
	go hub.Run()

	mockId := new(mockIdentityService)
	handler := NewHandler(hub, mockId, logger)

	r := gin.New()
	r.GET("/ws", handler.ServeWS)

	server := httptest.NewServer(r)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws?api_key=valid-key"
	mockId.On("ValidateApiKey", mock.Anything, "valid-key").Return(&domain.ApiKey{Key: "valid-key", UserID: uuid.New()}, nil)

	dialer := websocket.Dialer{}
	conn, _, err := dialer.Dial(wsURL, nil)
	assert.NoError(t, err)
	defer conn.Close()

	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 1, hub.ClientCount())

	event := &domain.WSEvent{Type: domain.WSEventInstanceCreated, Timestamp: time.Now()}
	hub.BroadcastEvent(event)

	_, p, err := conn.ReadMessage()
	assert.NoError(t, err)
	assert.Contains(t, string(p), "INSTANCE_CREATED")

	conn.Close()
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 0, hub.ClientCount())
}

func TestWebSocket_AuthFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	hub := NewHub(logger)
	mockId := new(mockIdentityService)
	handler := NewHandler(hub, mockId, logger)

	r := gin.New()
	r.GET("/ws", handler.ServeWS)

	server := httptest.NewServer(r)
	defer server.Close()

	t.Run("Missing API Key", func(t *testing.T) {
		wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
		dialer := websocket.Dialer{}
		_, resp, err := dialer.Dial(wsURL, nil)
		assert.Error(t, err)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("Invalid API Key", func(t *testing.T) {
		wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws?api_key=invalid"
		mockId.On("ValidateApiKey", mock.Anything, "invalid").Return(nil, errors.New(errors.Unauthorized, "invalid key"))
		dialer := websocket.Dialer{}
		_, resp, err := dialer.Dial(wsURL, nil)
		assert.Error(t, err)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})
}
