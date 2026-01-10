package platform

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewDatabase_InvalidURL(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	pool, err := NewDatabase(context.Background(), &Config{DatabaseURL: "invalid-url"}, logger)
	assert.Error(t, err)
	assert.Nil(t, pool)
	assert.Contains(t, err.Error(), "unable to parse database url")
}
