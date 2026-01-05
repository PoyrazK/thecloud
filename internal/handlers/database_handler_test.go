package httphandlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockDatabaseService struct {
	mock.Mock
}

func (m *mockDatabaseService) CreateDatabase(ctx context.Context, name, engine, version string, vpcID *uuid.UUID) (*domain.Database, error) {
	args := m.Called(ctx, name, engine, version, vpcID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Database), args.Error(1)
}

func (m *mockDatabaseService) ListDatabases(ctx context.Context) ([]*domain.Database, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*domain.Database), args.Error(1)
}

func (m *mockDatabaseService) GetDatabase(ctx context.Context, id uuid.UUID) (*domain.Database, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Database), args.Error(1)
}

func (m *mockDatabaseService) DeleteDatabase(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockDatabaseService) GetConnectionString(ctx context.Context, id uuid.UUID) (string, error) {
	args := m.Called(ctx, id)
	return args.String(0), args.Error(1)
}

func TestDatabaseHandler_Create(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockDatabaseService)
	handler := NewDatabaseHandler(svc)

	r := gin.New()
	r.POST("/databases", handler.Create)

	db := &domain.Database{ID: uuid.New(), Name: "db-1"}
	svc.On("CreateDatabase", mock.Anything, "db-1", "postgres", "15", (*uuid.UUID)(nil)).Return(db, nil)

	body, _ := json.Marshal(map[string]interface{}{
		"name":    "db-1",
		"engine":  "postgres",
		"version": "15",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/databases", bytes.NewBuffer(body))
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestDatabaseHandler_List(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockDatabaseService)
	handler := NewDatabaseHandler(svc)

	r := gin.New()
	r.GET("/databases", handler.List)

	dbs := []*domain.Database{{ID: uuid.New(), Name: "db-1"}}
	svc.On("ListDatabases", mock.Anything).Return(dbs, nil)

	req := httptest.NewRequest(http.MethodGet, "/databases", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDatabaseHandler_Get(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockDatabaseService)
	handler := NewDatabaseHandler(svc)

	r := gin.New()
	r.GET("/databases/:id", handler.Get)

	id := uuid.New()
	db := &domain.Database{ID: id, Name: "db-1"}
	svc.On("GetDatabase", mock.Anything, id).Return(db, nil)

	req := httptest.NewRequest(http.MethodGet, "/databases/"+id.String(), nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDatabaseHandler_Delete(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockDatabaseService)
	handler := NewDatabaseHandler(svc)

	r := gin.New()
	r.DELETE("/databases/:id", handler.Delete)

	id := uuid.New()
	svc.On("DeleteDatabase", mock.Anything, id).Return(nil)

	req := httptest.NewRequest(http.MethodDelete, "/databases/"+id.String(), nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDatabaseHandler_GetConnectionString(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(mockDatabaseService)
	handler := NewDatabaseHandler(svc)

	r := gin.New()
	r.GET("/databases/:id/connection", handler.GetConnectionString)

	id := uuid.New()
	svc.On("GetConnectionString", mock.Anything, id).Return("postgres://user:pass@host:5432/db", nil)

	req := httptest.NewRequest(http.MethodGet, "/databases/"+id.String()+"/connection", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "postgres://user:pass@host:5432/db")
}
