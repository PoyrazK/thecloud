package httphandlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTraceIDGeneration(t *testing.T) {
	tests := []struct {
		name        string
		generator   func() string
		expectedLen int
		checkUnique bool
	}{
		{
			name:        "trace ID has correct length",
			generator:   generateTraceID,
			expectedLen: 32,
			checkUnique: true,
		},
		{
			name:        "span ID has correct length",
			generator:   generateSpanID,
			expectedLen: 16,
			checkUnique: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			id := tt.generator()
			assert.Len(t, id, tt.expectedLen, "ID should have expected length")

			if tt.checkUnique {
				id2 := tt.generator()
				assert.NotEqual(t, id, id2, "consecutive IDs should be unique")
			}
		})
	}
}
