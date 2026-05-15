package postgres

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVolumeEncryptionRepository_SaveKey_Insert(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewVolumeEncryptionRepository(mock)
	volID := uuid.New()
	kmsKeyID := "key-12345"
	encryptedDEK := []byte("encrypted-data")
	algorithm := "AES-256-GCM"

	mock.ExpectExec("INSERT INTO volume_encryption_keys").
		WithArgs(volID, encryptedDEK, kmsKeyID, algorithm).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	err = repo.SaveKey(context.Background(), volID, kmsKeyID, encryptedDEK, algorithm)
	require.NoError(t, err)
}

func TestVolumeEncryptionRepository_SaveKey_Update(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewVolumeEncryptionRepository(mock)
	volID := uuid.New()
	kmsKeyID := "key-12345-new"
	encryptedDEK := []byte("new-encrypted-data")
	algorithm := "AES-256-GCM"

	mock.ExpectExec("INSERT INTO volume_encryption_keys").
		WithArgs(volID, encryptedDEK, kmsKeyID, algorithm).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	err = repo.SaveKey(context.Background(), volID, kmsKeyID, encryptedDEK, algorithm)
	require.NoError(t, err)
}

func TestVolumeEncryptionRepository_GetKey_Found(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewVolumeEncryptionRepository(mock)
	volID := uuid.New()
	encryptedDEK := []byte("encrypted-data")
	kmsKeyID := "key-12345"

	mock.ExpectQuery("SELECT encrypted_dek, kms_key_id FROM volume_encryption_keys WHERE volume_id = .+").
		WithArgs(volID).
		WillReturnRows(pgxmock.NewRows([]string{"encrypted_dek", "kms_key_id"}).
			AddRow(encryptedDEK, kmsKeyID))

	resultDEK, resultKMS, err := repo.GetKey(context.Background(), volID)
	require.NoError(t, err)
	assert.Equal(t, encryptedDEK, resultDEK)
	assert.Equal(t, kmsKeyID, resultKMS)
}

func TestVolumeEncryptionRepository_GetKey_NotFound(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewVolumeEncryptionRepository(mock)
	volID := uuid.New()

	mock.ExpectQuery("SELECT encrypted_dek, kms_key_id FROM volume_encryption_keys WHERE volume_id = .+").
		WithArgs(volID).
		WillReturnError(pgx.ErrNoRows)

	resultDEK, resultKMS, err := repo.GetKey(context.Background(), volID)
	require.Error(t, err)
	assert.NotErrorIs(t, err, pgx.ErrNoRows)
	assert.Nil(t, resultDEK)
	assert.Equal(t, "", resultKMS)
}

func TestVolumeEncryptionRepository_DeleteKey(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewVolumeEncryptionRepository(mock)
	volID := uuid.New()

	mock.ExpectExec("DELETE FROM volume_encryption_keys WHERE volume_id = .+").
		WithArgs(volID).
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	err = repo.DeleteKey(context.Background(), volID)
	require.NoError(t, err)
}

func TestVolumeEncryptionRepository_DeleteKey_NotFound(t *testing.T) {
	t.Parallel()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewVolumeEncryptionRepository(mock)
	volID := uuid.New()

	mock.ExpectExec("DELETE FROM volume_encryption_keys WHERE volume_id = .+").
		WithArgs(volID).
		WillReturnResult(pgxmock.NewResult("DELETE", 0))

	err = repo.DeleteKey(context.Background(), volID)
	require.NoError(t, err)
}
