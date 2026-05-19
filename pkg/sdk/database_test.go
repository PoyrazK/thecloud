package sdk

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	dbTestName        = "test-db"
	dbID              = "db-123"
	dbAPIKey          = "test-api-key"
	dbContentType     = "Content-Type"
	dbApplicationJSON = "application/json"
	dbPathPrefix      = "/databases/"
)

func TestClientCreateDatabase(t *testing.T) {
	vpcID := "vpc-123"
	expectedDB := Database{
		ID:      "db-1",
		Name:    dbTestName,
		Engine:  "postgres",
		Version: "14",
		Status:  "CREATING",
		VpcID:   &vpcID,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/databases", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		var req CreateDatabaseInput
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, expectedDB.Name, req.Name)
		assert.Equal(t, expectedDB.Engine, req.Engine)
		assert.Equal(t, 10, req.AllocatedStorage)

		w.Header().Set(dbContentType, dbApplicationJSON)
		resp := Response[Database]{Data: expectedDB}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, dbAPIKey)
	db, err := client.CreateDatabase(dbTestName, "postgres", "14", &vpcID, 10)

	require.NoError(t, err)
	assert.NotNil(t, db)
	assert.Equal(t, expectedDB.ID, db.ID)
}

func TestClientListDatabases(t *testing.T) {
	expectedDBs := []*Database{
		{ID: "db-1", Name: "db-1"},
		{ID: "db-2", Name: "db-2"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/databases", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)

		w.Header().Set(dbContentType, dbApplicationJSON)
		resp := Response[[]*Database]{Data: expectedDBs}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, dbAPIKey)
	dbs, err := client.ListDatabases()

	require.NoError(t, err)
	assert.Len(t, dbs, 2)
}

func TestClientGetDatabase(t *testing.T) {
	id := dbID
	expectedDB := Database{
		ID:   id,
		Name: dbTestName,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, dbPathPrefix+id, r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)

		w.Header().Set(dbContentType, dbApplicationJSON)
		resp := Response[Database]{Data: expectedDB}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, dbAPIKey)
	db, err := client.GetDatabase(id)

	require.NoError(t, err)
	assert.NotNil(t, db)
	assert.Equal(t, expectedDB.ID, db.ID)
}

func TestClientDeleteDatabase(t *testing.T) {
	id := dbID

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, dbPathPrefix+id, r.URL.Path)
		assert.Equal(t, http.MethodDelete, r.Method)

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL, dbAPIKey)
	err := client.DeleteDatabase(id)

	require.NoError(t, err)
}

func TestClientGetDatabaseConnectionString(t *testing.T) {
	id := dbID
	connStr := "postgres://user:pass@host:5432/dbname"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, dbPathPrefix+id+"/connection", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)

		w.Header().Set(dbContentType, dbApplicationJSON)
		resp := Response[map[string]string]{Data: map[string]string{"connection_string": connStr}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, dbAPIKey)
	result, err := client.GetDatabaseConnectionString(id)

	require.NoError(t, err)
	assert.Equal(t, connStr, result)
}

func TestClientRotateDatabaseCredentials(t *testing.T) {
	id := dbID

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, dbPathPrefix+id+"/rotate-credentials", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":{"message":"database credentials rotated successfully"}}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, dbAPIKey)
	err := client.RotateDatabaseCredentials(id)

	require.NoError(t, err)
}

func TestClientDatabaseErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer server.Close()

	client := NewClient(server.URL, dbAPIKey)

	_, err := client.CreateDatabase("db", "postgres", "14", nil, 10)
	require.Error(t, err)

	_, err = client.ListDatabases()
	require.Error(t, err)

	_, err = client.GetDatabase("db-1")
	require.Error(t, err)

	err = client.DeleteDatabase("db-1")
	require.Error(t, err)

	_, err = client.GetDatabaseConnectionString("db-1")
	require.Error(t, err)

	err = client.RotateDatabaseCredentials("db-1")
	require.Error(t, err)
}
