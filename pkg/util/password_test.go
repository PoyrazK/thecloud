package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateRandomPassword(t *testing.T) {
	length := 16
	password, err := GenerateRandomPassword(length)
	assert.NoError(t, err)
	assert.Len(t, password, length)
	assert.NotEmpty(t, password)

	// Check for randomness (simple check)
	password2, err := GenerateRandomPassword(length)
	assert.NoError(t, err)
	assert.NotEqual(t, password, password2, "passwords should be random")
}

func TestGenerateRandomPassword_ZeroLength(t *testing.T) {
	password, err := GenerateRandomPassword(0)
	assert.NoError(t, err)
	assert.Equal(t, "", password)
}
