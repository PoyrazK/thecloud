package noop

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNoopDNSBackend_CreateZone(t *testing.T) {
	t.Parallel()
	backend := NewNoopDNSBackend()
	err := backend.CreateZone(context.Background(), "example.com", []string{"ns1.example.com"})
	assert.NoError(t, err)
}

func TestNoopDNSBackend_DeleteZone(t *testing.T) {
	t.Parallel()
	backend := NewNoopDNSBackend()
	err := backend.DeleteZone(context.Background(), "example.com")
	assert.NoError(t, err)
}

func TestNoopDNSBackend_GetZone(t *testing.T) {
	t.Parallel()
	backend := NewNoopDNSBackend()
	result, err := backend.GetZone(context.Background(), "example.com")
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "example.com", result.Name)
	assert.Equal(t, "Native", result.Kind)
}

func TestNoopDNSBackend_AddRecords(t *testing.T) {
	t.Parallel()
	backend := NewNoopDNSBackend()
	err := backend.AddRecords(context.Background(), "example.com", nil)
	assert.NoError(t, err)
}

func TestNoopDNSBackend_UpdateRecords(t *testing.T) {
	t.Parallel()
	backend := NewNoopDNSBackend()
	err := backend.UpdateRecords(context.Background(), "example.com", nil)
	assert.NoError(t, err)
}

func TestNoopDNSBackend_DeleteRecords(t *testing.T) {
	t.Parallel()
	backend := NewNoopDNSBackend()
	err := backend.DeleteRecords(context.Background(), "example.com", "www", "A")
	assert.NoError(t, err)
}

func TestNoopDNSBackend_ListRecords(t *testing.T) {
	t.Parallel()
	backend := NewNoopDNSBackend()
	result, err := backend.ListRecords(context.Background(), "example.com")
	assert.NoError(t, err)
	assert.Empty(t, result)
}