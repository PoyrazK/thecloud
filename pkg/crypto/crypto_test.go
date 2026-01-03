package crypto

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCrypto_EncryptDecrypt(t *testing.T) {
	masterKey := []byte("very-secure-master-key-32-chars!")
	salt := []byte("some-salt")

	key, err := DeriveKey(masterKey, salt)
	require.NoError(t, err)
	assert.Len(t, key, 32)

	plaintext := []byte("this is a top secret message")

	// Encrypt
	ciphertext, err := Encrypt(plaintext, key)
	require.NoError(t, err)
	assert.NotEmpty(t, ciphertext)

	// Decrypt
	decrypted, err := Decrypt(ciphertext, key)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)

	// Different key should fail
	otherKey := make([]byte, 32)
	copy(otherKey, key)
	otherKey[0] ^= 0xFF // Flip one byte
	_, err = Decrypt(ciphertext, otherKey)
	assert.Error(t, err)
}

func TestCrypto_UniqueNonces(t *testing.T) {
	key := make([]byte, 32)
	plaintext := []byte("consistent message")

	c1, err := Encrypt(plaintext, key)
	require.NoError(t, err)

	c2, err := Encrypt(plaintext, key)
	require.NoError(t, err)

	// Even with same key and plaintext, ciphertexts should be different due to random nonces
	assert.NotEqual(t, c1, c2)
}

func TestDeriveKey_Consistency(t *testing.T) {
	masterKey := []byte("master")
	salt := []byte("salt")

	k1, err := DeriveKey(masterKey, salt)
	require.NoError(t, err)

	k2, err := DeriveKey(masterKey, salt)
	require.NoError(t, err)

	assert.Equal(t, hex.EncodeToString(k1), hex.EncodeToString(k2))
}
