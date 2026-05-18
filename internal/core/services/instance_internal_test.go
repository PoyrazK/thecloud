package services

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const testVolumeName = "test-vol"

// mockVolumeRepo is already defined in dashboard_test.go (package services)

func TestInstanceServiceInternalGetVolumeByIDOrName(t *testing.T) {
	t.Parallel()
	repo := new(mockVolumeRepo)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	rbacSvc := new(mockRBACService)
	rbacSvc.On("Authorize", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	svc := &InstanceService{volumeRepo: repo, rbacSvc: rbacSvc, logger: logger}
	ctx := context.Background()
	volID := uuid.New()

	t.Run("ByID", func(t *testing.T) {
		repo.On("GetByID", ctx, volID).Return(&domain.Volume{ID: volID}, nil).Once()
		res, err := svc.getVolumeByIDOrName(ctx, volID.String())
		require.NoError(t, err)
		assert.Equal(t, volID, res.ID)
	})

	t.Run("ByName", func(t *testing.T) {
		repo.On("GetByName", ctx, testVolumeName).Return(&domain.Volume{Name: testVolumeName}, nil).Once()
		res, err := svc.getVolumeByIDOrName(ctx, testVolumeName)
		require.NoError(t, err)
		assert.Equal(t, testVolumeName, res.Name)
	})
}

func TestInstanceServiceInternalResolveVolumes(t *testing.T) {
	t.Parallel()
	repo := new(mockVolumeRepo)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	rbacSvc := new(mockRBACService)
	rbacSvc.On("Authorize", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	svc := &InstanceService{volumeRepo: repo, rbacSvc: rbacSvc, logger: logger}
	ctx := context.Background()
	volID := uuid.New()

	repo.On("GetByID", ctx, volID).Return(&domain.Volume{ID: volID, Name: "vol1", Status: domain.VolumeStatusAvailable}, nil).Once()

	binds, vols, err := svc.resolveVolumes(ctx, []domain.VolumeAttachment{{VolumeIDOrName: volID.String(), MountPath: "/data"}})
	require.NoError(t, err)
	assert.Len(t, binds, 1)
	assert.Len(t, vols, 1)
}

func TestInstanceServiceInternalResolveVolumesUnavailable(t *testing.T) {
	t.Parallel()
	repo := new(mockVolumeRepo)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	rbacSvc := new(mockRBACService)
	rbacSvc.On("Authorize", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	svc := &InstanceService{volumeRepo: repo, rbacSvc: rbacSvc, logger: logger}
	ctx := context.Background()
	volID := uuid.New()

	repo.On("GetByID", ctx, volID).Return(&domain.Volume{ID: volID, Name: "vol1", Status: domain.VolumeStatusInUse}, nil).Once()

	_, _, err := svc.resolveVolumes(ctx, []domain.VolumeAttachment{{VolumeIDOrName: volID.String(), MountPath: "/data"}})
	require.Error(t, err)
}

func TestInstanceServiceInternalUpdateVolumesAfterLaunch(t *testing.T) {
	t.Parallel()
	repo := new(mockVolumeRepo)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	rbacSvc := new(mockRBACService)
	rbacSvc.On("Authorize", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	svc := &InstanceService{volumeRepo: repo, rbacSvc: rbacSvc, logger: logger}
	ctx := context.Background()
	instID := uuid.New()
	vol := &domain.Volume{ID: uuid.New(), Status: domain.VolumeStatusAvailable}

	repo.On("Update", ctx, mock.MatchedBy(func(v *domain.Volume) bool {
		return v.Status == domain.VolumeStatusInUse && v.InstanceID != nil && *v.InstanceID == instID
	})).Return(nil).Once()

	svc.updateVolumesAfterLaunch(ctx, []*domain.Volume{vol}, instID)
	repo.AssertExpectations(t)
}

func ptrUint64(v uint64) *uint64 { return &v }

func TestInstanceService_CalculateInstanceStats(t *testing.T) {
	svc := &InstanceService{}

	cases := []struct {
		name       string
		input      *domain.RawDockerStats
		cpuPercent float64
		memPercent float64
		netRx      *uint64
		netTx      *uint64
		diskRead   *uint64
		diskWrite  *uint64
		cpuTime    *uint64
	}{
		{
			name: "Basic CPU and Memory",
			input: func() *domain.RawDockerStats {
				s := &domain.RawDockerStats{}
				s.CPUStats.CPUUsage.TotalUsage = 1000
				s.CPUStats.SystemCPUUsage = 10000
				s.PreCPUStats.CPUUsage.TotalUsage = 500
				s.PreCPUStats.SystemCPUUsage = 5000
				s.MemoryStats.Usage = 1024
				s.MemoryStats.Limit = 2048
				return s
			}(),
			cpuPercent: 10.0,
			memPercent: 50.0,
			netRx:      nil,
			netTx:      nil,
			diskRead:   nil,
			diskWrite:  nil,
			cpuTime:    nil,
		},
		{
			name: "Network I/O multiple interfaces",
			input: func() *domain.RawDockerStats {
				s := &domain.RawDockerStats{}
				s.NetworkStats = map[string]struct {
					RxBytes uint64 `json:"rx_bytes"`
					TxBytes uint64 `json:"tx_bytes"`
				}{
					"eth0": {RxBytes: 1000, TxBytes: 500},
					"eth1": {RxBytes: 2000, TxBytes: 1500},
				}
				return s
			}(),
			netRx: ptrUint64(3000),
			netTx: ptrUint64(2000),
		},
		{
			name: "Block I/O read and write",
			input: func() *domain.RawDockerStats {
				s := &domain.RawDockerStats{}
				s.BlkioStats.IoServiceBytes = []domain.BlkioStatEntry{
					{Op: "read", Value: 5000},
					{Op: "write", Value: 3000},
					{Op: "Read", Value: 1000},
					{Op: "Write", Value: 2000},
				}
				return s
			}(),
			diskRead:  ptrUint64(6000),
			diskWrite: ptrUint64(5000),
		},
		{
			name: "CPU time nanoseconds",
			input: func() *domain.RawDockerStats {
				s := &domain.RawDockerStats{}
				s.CPUStats.CPUTime = 5000000000
				return s
			}(),
			cpuTime: ptrUint64(5000000000),
		},
		{
			name: "Combined all fields",
			input: func() *domain.RawDockerStats {
				s := &domain.RawDockerStats{}
				s.CPUStats.CPUUsage.TotalUsage = 800
				s.CPUStats.SystemCPUUsage = 8000
				s.PreCPUStats.CPUUsage.TotalUsage = 400
				s.PreCPUStats.SystemCPUUsage = 4000
				s.MemoryStats.Usage = 512
				s.MemoryStats.Limit = 1024
				s.CPUStats.CPUTime = 3000000000
				s.NetworkStats = map[string]struct {
					RxBytes uint64 `json:"rx_bytes"`
					TxBytes uint64 `json:"tx_bytes"`
				}{
					"eth0": {RxBytes: 500, TxBytes: 250},
				}
				s.BlkioStats.IoServiceBytes = []domain.BlkioStatEntry{
					{Op: "read", Value: 2048},
				}
				return s
			}(),
			cpuPercent: 10.0,
			memPercent: 50.0,
			netRx:      ptrUint64(500),
			netTx:      ptrUint64(250),
			diskRead:   ptrUint64(2048),
			diskWrite:  ptrUint64(0),
			cpuTime:    ptrUint64(3000000000),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := svc.calculateInstanceStats(tc.input)
			assert.InDelta(t, tc.cpuPercent, res.CPUPercentage, 0.01)
			assert.InDelta(t, tc.memPercent, res.MemoryPercentage, 0.01)
			if tc.netRx == nil {
				assert.Nil(t, res.NetworkRxBytes)
			} else {
				assert.Equal(t, *tc.netRx, *res.NetworkRxBytes)
			}
			if tc.netTx == nil {
				assert.Nil(t, res.NetworkTxBytes)
			} else {
				assert.Equal(t, *tc.netTx, *res.NetworkTxBytes)
			}
			if tc.diskRead == nil {
				assert.Nil(t, res.DiskReadBytes)
			} else {
				assert.Equal(t, *tc.diskRead, *res.DiskReadBytes)
			}
			if tc.diskWrite == nil {
				assert.Nil(t, res.DiskWriteBytes)
			} else {
				assert.Equal(t, *tc.diskWrite, *res.DiskWriteBytes)
			}
			if tc.cpuTime == nil {
				assert.Nil(t, res.CPUTimeNanoseconds)
			} else {
				assert.Equal(t, *tc.cpuTime, *res.CPUTimeNanoseconds)
			}
		})
	}
}

func TestInstanceService_FormatContainerName(t *testing.T) {
	svc := &InstanceService{}
	id := uuid.New()
	name := svc.formatContainerName(id)
	assert.Equal(t, "thecloud-"+id.String()[:8], name)
}

func TestInstanceService_IsValidHostIP(t *testing.T) {
	svc := &InstanceService{}
	_, ipNet, _ := net.ParseCIDR("10.0.0.0/24")

	t.Run("Valid", func(t *testing.T) {
		assert.True(t, svc.isValidHostIP(net.ParseIP("10.0.0.5"), ipNet))
	})

	t.Run("NetworkAddress", func(t *testing.T) {
		assert.False(t, svc.isValidHostIP(net.ParseIP("10.0.0.0"), ipNet))
	})

	t.Run("BroadcastAddress", func(t *testing.T) {
		assert.False(t, svc.isValidHostIP(net.ParseIP("10.0.0.255"), ipNet))
	})

	t.Run("OutsideSubnet", func(t *testing.T) {
		assert.False(t, svc.isValidHostIP(net.ParseIP("10.0.1.5"), ipNet))
	})
}

func TestParsePort(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		p, err := parsePort("80")
		require.NoError(t, err)
		assert.Equal(t, 80, p)
	})

	t.Run("Empty", func(t *testing.T) {
		_, err := parsePort("")
		require.Error(t, err)
	})

	t.Run("Invalid", func(t *testing.T) {
		_, err := parsePort("abc")
		require.Error(t, err)
	})
}

func TestInstanceService_UpdateInstanceMetadata(t *testing.T) {
	repo := new(mockInstanceRepo)
	rbacSvc := new(mockRBACService)
	rbacSvc.On("Authorize", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	svc := &InstanceService{repo: repo, rbacSvc: rbacSvc}
	ctx := context.Background()
	id := uuid.New()
	inst := &domain.Instance{
		ID:       id,
		Metadata: map[string]string{"old": "val"},
		Labels:   map[string]string{"l1": "v1"},
	}

	repo.On("GetByID", ctx, id).Return(inst, nil).Once()
	repo.On("Update", ctx, inst).Return(nil).Once()

	metadata := map[string]string{"new": "val", "old": ""} // "old" should be deleted
	labels := map[string]string{"l2": "v2"}

	err := svc.UpdateInstanceMetadata(ctx, id, metadata, labels)
	require.NoError(t, err)
	assert.Equal(t, "val", inst.Metadata["new"])
	assert.Equal(t, "v2", inst.Labels["l2"])
	assert.Equal(t, "v1", inst.Labels["l1"])
	_, ok := inst.Metadata["old"]
	assert.False(t, ok)
}
