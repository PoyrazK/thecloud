package sdk

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	snapshotDescription     = "test snapshot"
	snapshotAPIKey          = "test-api-key"
	snapshotContentType     = "Content-Type"
	snapshotApplicationJSON = "application/json"
	snapshotPath            = "/snapshots"
	snapshotPathPrefix      = "/snapshots/"
	snapshotExampleID       = "snap-1"
)

func TestClientCreateSnapshot(t *testing.T) {
	volumeID := uuid.New()
	volumeIDStr := volumeID.String()
	expectedSnapshot := domain.Snapshot{
		ID:          uuid.New(),
		VolumeID:    volumeID,
		Description: snapshotDescription,
		Status:      domain.SnapshotStatusAvailable,
		CreatedAt:   time.Now(),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/volumes" && r.Method == http.MethodGet {
			w.Header().Set(snapshotContentType, snapshotApplicationJSON)
			err := json.NewEncoder(w).Encode(Response[[]Volume]{
				Data: []Volume{{ID: volumeID, Name: "test-volume"}},
			})
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			return
		}
		assert.Equal(t, snapshotPath, r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		var req map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, volumeIDStr, req["volume_id"])
		assert.Equal(t, expectedSnapshot.Description, req["description"])

		w.Header().Set(snapshotContentType, snapshotApplicationJSON)
		_ = json.NewEncoder(w).Encode(Response[domain.Snapshot]{Data: expectedSnapshot})
	}))
	defer server.Close()

	client := NewClient(server.URL, snapshotAPIKey)
	snapshot, err := client.CreateSnapshot(context.Background(), volumeIDStr, snapshotDescription)

	require.NoError(t, err)
	assert.NotNil(t, snapshot)
	assert.Equal(t, expectedSnapshot.ID, snapshot.ID)
	assert.Equal(t, expectedSnapshot.VolumeID, snapshot.VolumeID)
}

func TestClientListSnapshots(t *testing.T) {
	expectedSnapshots := []*domain.Snapshot{
		{ID: uuid.New(), Description: snapshotExampleID},
		{ID: uuid.New(), Description: "snap-2"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, snapshotPath, r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)

		w.Header().Set(snapshotContentType, snapshotApplicationJSON)
		_ = json.NewEncoder(w).Encode(Response[[]*domain.Snapshot]{Data: expectedSnapshots})
	}))
	defer server.Close()

	client := NewClient(server.URL, snapshotAPIKey)
	snapshots, err := client.ListSnapshots()

	require.NoError(t, err)
	assert.Len(t, snapshots, 2)
	assert.Equal(t, expectedSnapshots[0].Description, snapshots[0].Description)
}

func TestClientGetSnapshot(t *testing.T) {
	id := uuid.New().String()
	expectedSnapshot := domain.Snapshot{ID: uuid.MustParse(id), Description: snapshotDescription}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, snapshotPathPrefix+id, r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)

		w.Header().Set(snapshotContentType, snapshotApplicationJSON)
		_ = json.NewEncoder(w).Encode(Response[domain.Snapshot]{Data: expectedSnapshot})
	}))
	defer server.Close()

	client := NewClient(server.URL, snapshotAPIKey)
	snapshot, err := client.GetSnapshot(id)

	require.NoError(t, err)
	assert.NotNil(t, snapshot)
	assert.Equal(t, expectedSnapshot.ID, snapshot.ID)
}

func TestClientDeleteSnapshot(t *testing.T) {
	id := uuid.New().String()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, snapshotPathPrefix+id, r.URL.Path)
		assert.Equal(t, http.MethodDelete, r.Method)

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL, snapshotAPIKey)
	err := client.DeleteSnapshot(id)

	require.NoError(t, err)
}

func TestClientRestoreSnapshot(t *testing.T) {
	id := uuid.New().String()
	newVolumeName := "restored-volume"
	expectedVolume := domain.Volume{
		ID:   uuid.New(),
		Name: newVolumeName,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, snapshotPathPrefix+id+"/restore", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		var req map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, newVolumeName, req["new_volume_name"])

		w.Header().Set(snapshotContentType, snapshotApplicationJSON)
		_ = json.NewEncoder(w).Encode(Response[domain.Volume]{Data: expectedVolume})
	}))
	defer server.Close()

	client := NewClient(server.URL, snapshotAPIKey)
	volume, err := client.RestoreSnapshot(id, newVolumeName)

	require.NoError(t, err)
	assert.NotNil(t, volume)
	assert.Equal(t, expectedVolume.ID, volume.ID)
	assert.Equal(t, expectedVolume.Name, volume.Name)
}

func TestClientSnapshotErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("boom"))
	}))
	defer server.Close()

	client := NewClient(server.URL, snapshotAPIKey)
	_, err := client.CreateSnapshot(context.Background(), uuid.New().String(), "snap")
	require.Error(t, err)

	_, err = client.ListSnapshots()
	require.Error(t, err)

	_, err = client.GetSnapshot(snapshotExampleID)
	require.Error(t, err)

	err = client.DeleteSnapshot(snapshotExampleID)
	require.Error(t, err)

	_, err = client.RestoreSnapshot(snapshotExampleID, "vol")
	require.Error(t, err)
}
