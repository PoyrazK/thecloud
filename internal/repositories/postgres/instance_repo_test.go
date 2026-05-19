package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v3"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	theclouderrors "github.com/poyrazk/thecloud/internal/errors"
	"github.com/poyrazk/thecloud/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testInstanceName = "inst-1"
	testInstanceImg  = "ubuntu:latest"
	testInstanceType = "basic-2"
	selectQuery      = "(?s)SELECT.+FROM instances.*"
)

func TestInstanceRepositoryCreate(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewInstanceRepository(mock)
		inst := &domain.Instance{
			ID:           uuid.New(),
			UserID:       uuid.New(),
			TenantID:     uuid.New(),
			Name:         testInstanceName,
			Image:        testInstanceImg,
			ContainerID:  "cid-1",
			Status:       domain.StatusRunning,
			Ports:        "80:80",
			VpcID:        nil,
			SubnetID:     nil,
			PrivateIP:    testutil.TestIPHost,
			OvsPort:      "ovs-1",
			InstanceType: testInstanceType,
			Version:      1,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		mock.ExpectExec("(?s)INSERT INTO instances.*").
			WithArgs(inst.ID, inst.UserID, inst.TenantID, inst.Name, inst.Image, inst.ContainerID, string(inst.Status), inst.Ports, inst.VpcID, inst.SubnetID, inst.PrivateIP, inst.OvsPort, inst.InstanceType, inst.VolumeBinds, inst.Env, inst.Cmd, inst.CPULimit, inst.MemoryLimit, inst.DiskLimit, inst.Metadata, inst.Labels, inst.SSHKeyID, inst.Version, inst.CreatedAt, inst.UpdatedAt).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))

		err = repo.Create(context.Background(), inst)
		require.NoError(t, err)
	})

	t.Run("db error", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewInstanceRepository(mock)
		inst := &domain.Instance{ID: uuid.New()}

		mock.ExpectExec("(?s)INSERT INTO instances.*").
			WillReturnError(errors.New(testDBError))

		err = repo.Create(context.Background(), inst)
		require.Error(t, err)
	})
}

func TestInstanceRepositoryGetByID(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewInstanceRepository(mock)
		id := uuid.New()
		userID := uuid.New()
		tenantID := uuid.New()
		ctx := appcontext.WithTenantID(appcontext.WithUserID(context.Background(), userID), tenantID)
		now := time.Now()

		mock.ExpectQuery(selectQuery).
			WithArgs(id, tenantID).
			WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "tenant_id", "name", "image", "container_id", "status", "ports", "vpc_id", "subnet_id", "private_ip", "ovs_port", "instance_type", "volume_binds", "env", "cmd", "cpu_limit", "memory_limit", "disk_limit", "metadata", "labels", "ssh_key_id", "version", "created_at", "updated_at"}).
				AddRow(id, userID, tenantID, testInstanceName, testInstanceImg, "cid-1", string(domain.StatusRunning), "80:80", nil, nil, testutil.TestIPHost, "ovs-1", testInstanceType, []string{}, []string{}, []string{}, int64(0), int64(0), int64(0), map[string]string{}, map[string]string{}, nil, 1, now, now))

		inst, err := repo.GetByID(ctx, id)
		require.NoError(t, err)
		if assert.NotNil(t, inst) {
			assert.Equal(t, id, inst.ID)
		}
	})

	t.Run("not found", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewInstanceRepository(mock)
		id := uuid.New()
		userID := uuid.New()
		tenantID := uuid.New()
		ctx := appcontext.WithTenantID(appcontext.WithUserID(context.Background(), userID), tenantID)

		mock.ExpectQuery(selectQuery).
			WithArgs(id, tenantID).
			WillReturnError(pgx.ErrNoRows)

		inst, err := repo.GetByID(ctx, id)
		require.Error(t, err)
		assert.Nil(t, inst)
		var target *theclouderrors.Error
		if errors.As(err, &target) {
			assert.Equal(t, theclouderrors.NotFound, target.Type)
		}
	})
}

func TestInstanceRepositoryGetByName(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewInstanceRepository(mock)
		id := uuid.New()
		userID := uuid.New()
		tenantID := uuid.New()
		ctx := appcontext.WithTenantID(appcontext.WithUserID(context.Background(), userID), tenantID)
		name := testInstanceName
		now := time.Now()

		mock.ExpectQuery(selectQuery).
			WithArgs(name, tenantID).
			WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "tenant_id", "name", "image", "container_id", "status", "ports", "vpc_id", "subnet_id", "private_ip", "ovs_port", "instance_type", "volume_binds", "env", "cmd", "cpu_limit", "memory_limit", "disk_limit", "metadata", "labels", "ssh_key_id", "version", "created_at", "updated_at"}).
				AddRow(id, userID, tenantID, name, testInstanceImg, "cid-1", string(domain.StatusRunning), "80:80", nil, nil, testutil.TestIPHost, "ovs-1", testInstanceType, []string{}, []string{}, []string{}, int64(0), int64(0), int64(0), map[string]string{}, map[string]string{}, nil, 1, now, now))

		inst, err := repo.GetByName(ctx, name)
		require.NoError(t, err)
		if assert.NotNil(t, inst) {
			assert.Equal(t, id, inst.ID)
		}
	})

	t.Run("not found", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewInstanceRepository(mock)
		userID := uuid.New()
		tenantID := uuid.New()
		ctx := appcontext.WithTenantID(appcontext.WithUserID(context.Background(), userID), tenantID)
		name := testInstanceName

		mock.ExpectQuery(selectQuery).
			WithArgs(name, tenantID).
			WillReturnError(pgx.ErrNoRows)

		inst, err := repo.GetByName(ctx, name)
		require.Error(t, err)
		assert.Nil(t, inst)
		var target *theclouderrors.Error
		ok := errors.As(err, &target)
		if ok {
			assert.Equal(t, theclouderrors.NotFound, target.Type)
		}
	})
}

func TestInstanceRepositoryList(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewInstanceRepository(mock)
		userID := uuid.New()
		tenantID := uuid.New()
		ctx := appcontext.WithTenantID(appcontext.WithUserID(context.Background(), userID), tenantID)
		now := time.Now()

		mock.ExpectQuery(selectQuery).
			WithArgs(tenantID).
			WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "tenant_id", "name", "image", "container_id", "status", "ports", "vpc_id", "subnet_id", "private_ip", "ovs_port", "instance_type", "volume_binds", "env", "cmd", "cpu_limit", "memory_limit", "disk_limit", "metadata", "labels", "ssh_key_id", "version", "created_at", "updated_at"}).
				AddRow(uuid.New(), userID, tenantID, testInstanceName, testInstanceImg, "cid-1", string(domain.StatusRunning), "80:80", nil, nil, testutil.TestIPHost, "ovs-1", testInstanceType, []string{}, []string{}, []string{}, int64(0), int64(0), int64(0), map[string]string{}, map[string]string{}, nil, 1, now, now))

		list, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.Len(t, list, 1)
	})

	t.Run("db error", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewInstanceRepository(mock)
		userID := uuid.New()
		tenantID := uuid.New()
		ctx := appcontext.WithTenantID(appcontext.WithUserID(context.Background(), userID), tenantID)

		mock.ExpectQuery(selectQuery).
			WithArgs(tenantID).
			WillReturnError(errors.New(testDBError))

		list, err := repo.List(ctx, nil)
		require.Error(t, err)
		assert.Nil(t, list)
	})
}

func TestInstanceRepositoryListBySubnet(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewInstanceRepository(mock)
		userID := uuid.New()
		tenantID := uuid.New()
		subnetID := uuid.New()
		ctx := appcontext.WithTenantID(appcontext.WithUserID(context.Background(), userID), tenantID)
		now := time.Now()

		mock.ExpectQuery(selectQuery).
			WithArgs(subnetID, tenantID).
			WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "tenant_id", "name", "image", "container_id", "status", "ports", "vpc_id", "subnet_id", "private_ip", "ovs_port", "instance_type", "volume_binds", "env", "cmd", "cpu_limit", "memory_limit", "disk_limit", "metadata", "labels", "ssh_key_id", "version", "created_at", "updated_at"}).
				AddRow(uuid.New(), userID, tenantID, testInstanceName, testInstanceImg, "cid-1", string(domain.StatusRunning), "80:80", nil, &subnetID, testutil.TestIPHost, "ovs-1", "basic-2", []string{}, []string{}, []string{}, int64(0), int64(0), int64(0), map[string]string{}, map[string]string{}, nil, 1, now, now))

		list, err := repo.ListBySubnet(ctx, subnetID)
		require.NoError(t, err)
		assert.Len(t, list, 1)
	})
}

func TestInstanceRepositoryUpdate(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewInstanceRepository(mock)
		inst := &domain.Instance{
			ID:           uuid.New(),
			UserID:       uuid.New(),
			TenantID:     uuid.New(),
			Name:         "inst-1-updated",
			Status:       domain.StatusStopped,
			ContainerID:  "cid-1",
			Ports:        "80:80",
			InstanceType: testInstanceType,
			Version:      1,
			UpdatedAt:    time.Now(),
		}

		mock.ExpectExec("(?s)UPDATE instances.*").
			WithArgs(inst.Name, string(inst.Status), pgxmock.AnyArg(), inst.ContainerID, inst.Ports, inst.VpcID, inst.SubnetID, inst.PrivateIP, inst.OvsPort, testInstanceType, inst.VolumeBinds, inst.Env, inst.Cmd, inst.CPULimit, inst.MemoryLimit, inst.DiskLimit, inst.Metadata, inst.Labels, inst.SSHKeyID, inst.ID, inst.Version, inst.TenantID).
			WillReturnResult(pgxmock.NewResult("UPDATE", 1))

		err = repo.Update(context.Background(), inst)
		require.NoError(t, err)
		assert.Equal(t, 2, inst.Version)
	})

	t.Run("concurrency conflict", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewInstanceRepository(mock)
		inst := &domain.Instance{
			ID:           uuid.New(),
			UserID:       uuid.New(),
			TenantID:     uuid.New(),
			Status:       domain.StatusStopped,
			InstanceType: testInstanceType,
			Version:      1,
		}

		mock.ExpectExec("(?s)UPDATE instances.*").
			WithArgs(inst.Name, string(inst.Status), pgxmock.AnyArg(), inst.ContainerID, inst.Ports, inst.VpcID, inst.SubnetID, inst.PrivateIP, inst.OvsPort, testInstanceType, inst.VolumeBinds, inst.Env, inst.Cmd, inst.CPULimit, inst.MemoryLimit, inst.DiskLimit, inst.Metadata, inst.Labels, inst.SSHKeyID, inst.ID, inst.Version, inst.TenantID).
			WillReturnResult(pgxmock.NewResult("UPDATE", 0))

		err = repo.Update(context.Background(), inst)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "conflict")
	})
}

func TestInstanceRepositoryDelete(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewInstanceRepository(mock)
		id := uuid.New()
		userID := uuid.New()
		tenantID := uuid.New()
		ctx := appcontext.WithTenantID(appcontext.WithUserID(context.Background(), userID), tenantID)

		mock.ExpectExec("DELETE FROM instances").
			WithArgs(id, tenantID).
			WillReturnResult(pgxmock.NewResult("DELETE", 1))

		err = repo.Delete(ctx, id)
		require.NoError(t, err)
	})

	t.Run(testNotFound, func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		require.NoError(t, err)
		defer mock.Close()

		repo := NewInstanceRepository(mock)
		id := uuid.New()
		userID := uuid.New()
		tenantID := uuid.New()
		ctx := appcontext.WithTenantID(appcontext.WithUserID(context.Background(), userID), tenantID)

		mock.ExpectExec("DELETE FROM instances").
			WithArgs(id, tenantID).
			WillReturnResult(pgxmock.NewResult("DELETE", 0))

		err = repo.Delete(ctx, id)
		require.Error(t, err)
		var target *theclouderrors.Error
		ok := errors.As(err, &target)
		if ok {
			assert.Equal(t, theclouderrors.NotFound, target.Type)
		}
	})
}

func TestInstanceRepositoryListAll(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewInstanceRepository(mock)
	now := time.Now()

	mock.ExpectQuery(selectQuery).
		WillReturnRows(pgxmock.NewRows([]string{"id", "user_id", "tenant_id", "name", "image", "container_id", "status", "ports", "vpc_id", "subnet_id", "private_ip", "ovs_port", "instance_type", "volume_binds", "env", "cmd", "cpu_limit", "memory_limit", "disk_limit", "metadata", "labels", "ssh_key_id", "version", "created_at", "updated_at"}).
			AddRow(uuid.New(), uuid.New(), uuid.New(), testInstanceName, testInstanceImg, "cid-1", string(domain.StatusRunning), "80:80", nil, nil, testutil.TestIPHost, "ovs-1", testInstanceType, []string{}, []string{}, []string{}, int64(0), int64(0), int64(0), map[string]string{}, map[string]string{}, nil, 1, now, now))

	list, err := repo.ListAll(context.Background())
	require.NoError(t, err)
	assert.Len(t, list, 1)
}

func TestInstanceRepositoryScanError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	repo := NewInstanceRepository(mock)
	id := uuid.New()
	tenantID := uuid.New()
	ctx := appcontext.WithTenantID(context.Background(), tenantID)

	mock.ExpectQuery(selectQuery).
		WithArgs(id, tenantID).
		WillReturnError(errors.New("scan failed"))

	inst, err := repo.GetByID(ctx, id)
	require.Error(t, err)
	assert.Nil(t, inst)
}
