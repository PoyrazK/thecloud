package services_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	appcontext "github.com/poyrazk/thecloud/internal/core/context"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestVolumeServiceUnit(t *testing.T) {
	mockRepo := new(MockVolumeRepo)
	mockStorage := new(MockStorageBackend)
	mockEventSvc := new(MockEventService)
	mockAuditSvc := new(MockAuditService)
	rbacSvc := new(MockRBACService)
	rbacSvc.On("Authorize", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	defer mockRepo.AssertExpectations(t)
	defer mockStorage.AssertExpectations(t)
	defer mockEventSvc.AssertExpectations(t)
	defer mockAuditSvc.AssertExpectations(t)

	svc := services.NewVolumeService(services.VolumeServiceParams{
		Repo:     mockRepo,
		RBACSvc:  rbacSvc,
		Storage:  mockStorage,
		EventSvc: mockEventSvc,
		AuditSvc: mockAuditSvc,
		Logger:   slog.Default(),
	})

	ctx := context.Background()
	userID := uuid.New()
	ctx = appcontext.WithUserID(ctx, userID)

	t.Run("CreateVolume", func(t *testing.T) {
		tests := []struct {
			name      string
			volName   string
			sizeGB    int
			setup     func()
			wantError bool
		}{
			{
				name:    "Success",
				volName: "test-vol",
				sizeGB:  10,
				setup: func() {
					mockStorage.On("CreateVolume", mock.Anything, mock.Anything, 10).Return("/dev/vdb", nil).Once()
					mockRepo.On("Create", mock.Anything, mock.Anything).Return(nil).Once()
					mockEventSvc.On("RecordEvent", mock.Anything, "VOLUME_CREATE", mock.Anything, "VOLUME", mock.Anything).Return(nil).Once()
					mockAuditSvc.On("Log", mock.Anything, userID, "volume.create", "volume", mock.Anything, mock.Anything).Return(nil).Once()
				},
				wantError: false,
			},
			{
				name:    "Storage Error",
				volName: "test-vol",
				sizeGB:  10,
				setup: func() {
					mockStorage.On("CreateVolume", mock.Anything, mock.Anything, 10).Return("", errors.New("storage fail")).Once()
				},
				wantError: true,
			},
			{
				name:    "Repo Error Rollback",
				volName: "test-vol",
				sizeGB:  10,
				setup: func() {
					mockStorage.On("CreateVolume", mock.Anything, mock.Anything, 10).Return("/dev/vdb", nil).Once()
					mockRepo.On("Create", mock.Anything, mock.Anything).Return(errors.New("db fail")).Once()
					mockStorage.On("DeleteVolume", mock.Anything, mock.Anything).Return(nil).Once()
				},
				wantError: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				tt.setup()
				vol, err := svc.CreateVolume(ctx, tt.volName, tt.sizeGB)
				if tt.wantError {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
					assert.NotNil(t, vol)
					assert.Equal(t, "/dev/vdb", vol.BackendPath)
				}
			})
		}
	})

	t.Run("GetAndList", func(t *testing.T) {
		t.Run("ListVolumes", func(t *testing.T) {
			mockRepo.On("List", mock.Anything).Return([]*domain.Volume{{ID: uuid.New()}}, nil).Once()
			vols, err := svc.ListVolumes(ctx)
			require.NoError(t, err)
			assert.Len(t, vols, 1)
		})

		t.Run("GetVolume", func(t *testing.T) {
			id := uuid.New()
			tests := []struct {
				name     string
				idOrName string
				setup    func()
				wantID   uuid.UUID
				wantName string
			}{
				{
					name:     "By ID",
					idOrName: id.String(),
					setup: func() {
						mockRepo.On("GetByID", mock.Anything, id).Return(&domain.Volume{ID: id}, nil).Once()
					},
					wantID: id,
				},
				{
					name:     "By Name",
					idOrName: "myvol",
					setup: func() {
						mockRepo.On("GetByName", mock.Anything, "myvol").Return(&domain.Volume{Name: "myvol"}, nil).Once()
					},
					wantName: "myvol",
				},
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					tt.setup()
					vol, err := svc.GetVolume(ctx, tt.idOrName)
					require.NoError(t, err)
					if tt.wantID != uuid.Nil {
						assert.Equal(t, tt.wantID, vol.ID)
					}
					if tt.wantName != "" {
						assert.Equal(t, tt.wantName, vol.Name)
					}
				})
			}
		})
	})

	t.Run("DeleteVolume", func(t *testing.T) {
		volID := uuid.New()
		tests := []struct {
			name      string
			idOrName  string
			setup     func()
			wantError bool
		}{
			{
				name:     "Success",
				idOrName: volID.String(),
				setup: func() {
					vol := &domain.Volume{ID: volID, Status: domain.VolumeStatusAvailable, UserID: userID, SizeGB: 10}
					mockRepo.On("GetByID", mock.Anything, volID).Return(vol, nil).Once()
					mockRepo.On("Update", mock.Anything, mock.MatchedBy(func(v *domain.Volume) bool {
						return v.Status == domain.VolumeStatusDeleting
					})).Return(nil).Once()
					mockStorage.On("DeleteVolume", mock.Anything, mock.Anything).Return(nil).Once()
					mockRepo.On("Delete", mock.Anything, volID).Return(nil).Once()
					mockEventSvc.On("RecordEvent", mock.Anything, "VOLUME_DELETE", volID.String(), "VOLUME", mock.Anything).Return(nil).Once()
					mockAuditSvc.On("Log", mock.Anything, userID, "volume.delete", "volume", volID.String(), mock.Anything).Return(nil).Once()
				},
				wantError: false,
			},
			{
				name:     "Storage Delete Failure",
				idOrName: volID.String(),
				setup: func() {
					vol := &domain.Volume{ID: volID, Status: domain.VolumeStatusAvailable, UserID: userID, SizeGB: 10}
					mockRepo.On("GetByID", mock.Anything, volID).Return(vol, nil).Once()
					mockRepo.On("Update", mock.Anything, mock.MatchedBy(func(v *domain.Volume) bool {
						return v.Status == domain.VolumeStatusDeleting
					})).Return(nil).Once()
					mockStorage.On("DeleteVolume", mock.Anything, mock.Anything).Return(errors.New("storage fail")).Once()
				},
				wantError: true,
			},
			{
				name:     "Update DB Failure",
				idOrName: volID.String(),
				setup: func() {
					vol := &domain.Volume{ID: volID, Status: domain.VolumeStatusAvailable, UserID: userID, SizeGB: 10}
					mockRepo.On("GetByID", mock.Anything, volID).Return(vol, nil).Once()
					mockRepo.On("Update", mock.Anything, mock.Anything).Return(errors.New("update fail")).Once()
				},
				wantError: true,
			},
			{
				name:     "Already Deleting Success",
				idOrName: volID.String(),
				setup: func() {
					vol := &domain.Volume{ID: volID, Status: domain.VolumeStatusDeleting, UserID: userID, SizeGB: 10}
					mockRepo.On("GetByID", mock.Anything, volID).Return(vol, nil).Once()
					mockStorage.On("DeleteVolume", mock.Anything, mock.Anything).Return(nil).Once()
					mockRepo.On("Delete", mock.Anything, volID).Return(nil).Once()
					mockEventSvc.On("RecordEvent", mock.Anything, "VOLUME_DELETE", volID.String(), "VOLUME", mock.Anything).Return(nil).Once()
					mockAuditSvc.On("Log", mock.Anything, userID, "volume.delete", "volume", volID.String(), mock.Anything).Return(nil).Once()
				},
				wantError: false,
			},
			{
				name:     "In Use Error",
				idOrName: volID.String(),
				setup: func() {
					vol := &domain.Volume{ID: volID, Status: domain.VolumeStatusInUse}
					mockRepo.On("GetByID", mock.Anything, volID).Return(vol, nil).Once()
				},
				wantError: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				tt.setup()
				err := svc.DeleteVolume(ctx, tt.idOrName)
				if tt.wantError {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}
			})
		}
	})

	t.Run("AttachDetach", func(t *testing.T) {
		volID := uuid.New()
		instID := uuid.New()

		t.Run("Attach", func(t *testing.T) {
			tests := []struct {
				name      string
				setup     func()
				wantError bool
			}{
				{
					name: "Success",
					setup: func() {
						vol := &domain.Volume{ID: volID, Status: domain.VolumeStatusAvailable, UserID: userID}
						mockRepo.On("GetByID", mock.Anything, volID).Return(vol, nil).Once()
						mockStorage.On("AttachVolume", mock.Anything, mock.Anything, instID.String()).Return("/dev/vdb", nil).Once()
						mockRepo.On("Update", mock.Anything, mock.MatchedBy(func(v *domain.Volume) bool {
							return v.Status == domain.VolumeStatusInUse && v.InstanceID != nil && *v.InstanceID == instID
						})).Return(nil).Once()
						mockAuditSvc.On("Log", mock.Anything, userID, "volume.attach", "volume", volID.String(), mock.Anything).Return(nil).Once()
					},
					wantError: false,
				},
				{
					name: "Repo Update Fail with Rollback",
					setup: func() {
						vol := &domain.Volume{ID: volID, Status: domain.VolumeStatusAvailable, UserID: userID}
						mockRepo.On("GetByID", mock.Anything, volID).Return(vol, nil).Once()
						mockStorage.On("AttachVolume", mock.Anything, mock.Anything, instID.String()).Return("/dev/vdb", nil).Once()
						mockRepo.On("Update", mock.Anything, mock.Anything).Return(errors.New("db update fail")).Once()
						mockStorage.On("DetachVolume", mock.Anything, mock.Anything, instID.String()).Return(nil).Once()
					},
					wantError: true,
				},
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					tt.setup()
					path, err := svc.AttachVolume(ctx, volID.String(), instID.String(), "/mnt/data")
					if tt.wantError {
						require.Error(t, err)
					} else {
						require.NoError(t, err)
						assert.Equal(t, "/dev/vdb", path)
					}
				})
			}
		})

		t.Run("Detach Success", func(t *testing.T) {
			vol := &domain.Volume{ID: volID, Status: domain.VolumeStatusInUse, InstanceID: &instID, UserID: userID}
			mockRepo.On("GetByID", mock.Anything, volID).Return(vol, nil).Once()
			mockStorage.On("DetachVolume", mock.Anything, mock.Anything, instID.String()).Return(nil).Once()
			mockRepo.On("Update", mock.Anything, mock.MatchedBy(func(v *domain.Volume) bool {
				return v.Status == domain.VolumeStatusAvailable && v.InstanceID == nil
			})).Return(nil).Once()
			mockAuditSvc.On("Log", mock.Anything, userID, "volume.detach", "volume", volID.String(), mock.Anything).Return(nil).Once()

			err := svc.DetachVolume(ctx, volID.String())
			require.NoError(t, err)
		})
	})

	t.Run("ReleaseVolumesForInstance", func(t *testing.T) {
		instID := uuid.New()
		vol := &domain.Volume{ID: uuid.New(), Status: domain.VolumeStatusInUse, InstanceID: &instID}
		mockRepo.On("ListByInstanceID", mock.Anything, instID).Return([]*domain.Volume{vol}, nil).Once()
		mockStorage.On("DetachVolume", mock.Anything, mock.Anything, instID.String()).Return(nil).Once()
		mockRepo.On("Update", mock.Anything, mock.MatchedBy(func(v *domain.Volume) bool {
			return v.Status == domain.VolumeStatusAvailable && v.InstanceID == nil
		})).Return(nil).Once()

		err := svc.ReleaseVolumesForInstance(ctx, instID)
		require.NoError(t, err)
	})

	t.Run("ResizeVolume", func(t *testing.T) {
		volID := uuid.New()
		t.Run("Success", func(t *testing.T) {
			vol := &domain.Volume{ID: volID, SizeGB: 10, UserID: userID}
			mockRepo.On("GetByID", mock.Anything, volID).Return(vol, nil).Once()
			mockStorage.On("ResizeVolume", mock.Anything, mock.Anything, 20).Return(nil).Once()
			mockRepo.On("Update", mock.Anything, mock.MatchedBy(func(v *domain.Volume) bool {
				return v.SizeGB == 20
			})).Return(nil).Once()
			mockEventSvc.On("RecordEvent", mock.Anything, "VOLUME_RESIZE", volID.String(), "VOLUME", mock.Anything).Return(nil).Once()
			mockAuditSvc.On("Log", mock.Anything, userID, "volume.resize", "volume", volID.String(), mock.Anything).Return(nil).Once()

			err := svc.ResizeVolume(ctx, volID.String(), 20)
			require.NoError(t, err)
		})

		t.Run("InvalidSize", func(t *testing.T) {
			vol := &domain.Volume{ID: volID, SizeGB: 10}
			mockRepo.On("GetByID", mock.Anything, volID).Return(vol, nil).Once()

			err := svc.ResizeVolume(ctx, volID.String(), 5)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "must be larger")
		})

		t.Run("VolumeDeleting", func(t *testing.T) {
			vol := &domain.Volume{ID: volID, Status: domain.VolumeStatusDeleting}
			mockRepo.On("GetByID", mock.Anything, volID).Return(vol, nil).Once()

			err := svc.ResizeVolume(ctx, volID.String(), 20)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "being deleted")
		})

		t.Run("DBUpdateFails_RollbackBackend", func(t *testing.T) {
			vol := &domain.Volume{ID: volID, SizeGB: 10, UserID: userID}
			mockRepo.On("GetByID", mock.Anything, volID).Return(vol, nil).Once()
			mockStorage.On("ResizeVolume", mock.Anything, mock.Anything, 20).Return(nil).Once()
			// DB update fails
			mockRepo.On("Update", mock.Anything, mock.Anything).Return(errors.New("db error")).Once()
			// Rollback called with old size
			mockStorage.On("ResizeVolume", mock.Anything, mock.Anything, 10).Return(nil).Once()

			err := svc.ResizeVolume(ctx, volID.String(), 20)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "db error")
			mockStorage.AssertCalled(t, "ResizeVolume", mock.Anything, mock.Anything, 10)
		})
	})

	t.Run("AttachErrors", func(t *testing.T) {
		volID := uuid.New()
		t.Run("AlreadyAttached", func(t *testing.T) {
			vol := &domain.Volume{ID: volID, Status: domain.VolumeStatusInUse}
			mockRepo.On("GetByID", mock.Anything, volID).Return(vol, nil).Once()

			_, err := svc.AttachVolume(ctx, volID.String(), uuid.New().String(), "/mnt")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "already attached")
		})

		t.Run("VolumeDeleting", func(t *testing.T) {
			vol := &domain.Volume{ID: volID, Status: domain.VolumeStatusDeleting}
			mockRepo.On("GetByID", mock.Anything, volID).Return(vol, nil).Once()

			_, err := svc.AttachVolume(ctx, volID.String(), uuid.New().String(), "/mnt")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "being deleted")
		})

		t.Run("InvalidInstanceID", func(t *testing.T) {
			vol := &domain.Volume{ID: volID, Status: domain.VolumeStatusAvailable}
			mockRepo.On("GetByID", mock.Anything, volID).Return(vol, nil).Once()

			_, err := svc.AttachVolume(ctx, volID.String(), "invalid-uuid", "/mnt")
			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid instance ID")
		})
	})

	t.Run("DetachErrors", func(t *testing.T) {
		volID := uuid.New()
		t.Run("NotAttached", func(t *testing.T) {
			vol := &domain.Volume{ID: volID, Status: domain.VolumeStatusAvailable}
			mockRepo.On("GetByID", mock.Anything, volID).Return(vol, nil).Once()

			err := svc.DetachVolume(ctx, volID.String())
			require.Error(t, err)
			assert.Contains(t, err.Error(), "not attached")
		})
	})
}

// TestVolumeServiceWithComputeBackend tests the full flow including compute backend integration
func TestVolumeServiceWithComputeBackend(t *testing.T) {
	mockRepo := new(MockVolumeRepo)
	mockStorage := new(MockStorageBackend)
	mockCompute := new(MockComputeBackend)
	mockInstanceRepo := new(MockInstanceRepo)
	mockEventSvc := new(MockEventService)
	mockAuditSvc := new(MockAuditService)
	rbacSvc := new(MockRBACService)
	rbacSvc.On("Authorize", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	defer mockRepo.AssertExpectations(t)
	defer mockStorage.AssertExpectations(t)
	defer mockCompute.AssertExpectations(t)
	defer mockInstanceRepo.AssertExpectations(t)
	defer mockEventSvc.AssertExpectations(t)
	defer mockAuditSvc.AssertExpectations(t)

	svc := services.NewVolumeService(services.VolumeServiceParams{
		Repo:         mockRepo,
		RBACSvc:      rbacSvc,
		Storage:      mockStorage,
		Compute:      mockCompute,
		InstanceRepo: mockInstanceRepo,
		EventSvc:     mockEventSvc,
		AuditSvc:     mockAuditSvc,
		Logger:       slog.Default(),
	})

	ctx := context.Background()
	userID := uuid.New()
	ctx = appcontext.WithUserID(ctx, userID)

	t.Run("Attach Success with Compute Backend", func(t *testing.T) {
		volID := uuid.New()
		instID := uuid.New()

		vol := &domain.Volume{ID: volID, Status: domain.VolumeStatusAvailable, UserID: userID}
		inst := &domain.Instance{ID: instID, ContainerID: "old-container-id"}

		mockRepo.On("GetByID", mock.Anything, volID).Return(vol, nil).Once()
		mockStorage.On("AttachVolume", mock.Anything, mock.Anything, instID.String()).Return("/dev/vdb", nil).Once()
		mockInstanceRepo.On("GetByID", mock.Anything, instID).Return(inst, nil).Once()
		mockCompute.On("AttachVolume", mock.Anything, "old-container-id", "/dev/vdb:/mnt/data:rw").
			Return("/mnt/data", "new-container-id", nil).Once()
		mockInstanceRepo.On("Update", mock.Anything, mock.MatchedBy(func(i *domain.Instance) bool {
			return i.ContainerID == "new-container-id"
		})).Return(nil).Once()
		mockRepo.On("Update", mock.Anything, mock.MatchedBy(func(v *domain.Volume) bool {
			return v.Status == domain.VolumeStatusInUse && v.InstanceID.String() == instID.String()
		})).Return(nil).Once()
		mockAuditSvc.On("Log", mock.Anything, userID, "volume.attach", "volume", mock.Anything, mock.Anything).Return(nil).Once()

		path, err := svc.AttachVolume(ctx, volID.String(), instID.String(), "/mnt/data")
		require.NoError(t, err)
		assert.Equal(t, "/dev/vdb", path)
	})

	t.Run("Detach Success with Compute Backend", func(t *testing.T) {
		volID := uuid.New()
		instID := uuid.New()

		vol := &domain.Volume{ID: volID, Status: domain.VolumeStatusInUse, InstanceID: &instID, MountPath: "/mnt/data", UserID: userID}
		inst := &domain.Instance{ID: instID, ContainerID: "old-container-id"}

		mockRepo.On("GetByID", mock.Anything, volID).Return(vol, nil).Once()
		mockInstanceRepo.On("GetByID", mock.Anything, instID).Return(inst, nil).Once()
		mockCompute.On("DetachVolume", mock.Anything, "old-container-id", "/mnt/data").
			Return("new-container-id", nil).Once()
		mockInstanceRepo.On("Update", mock.Anything, mock.MatchedBy(func(i *domain.Instance) bool {
			return i.ContainerID == "new-container-id"
		})).Return(nil).Once()
		mockStorage.On("DetachVolume", mock.Anything, mock.Anything, instID.String()).Return(nil).Once()
		mockRepo.On("Update", mock.Anything, mock.MatchedBy(func(v *domain.Volume) bool {
			return v.Status == domain.VolumeStatusAvailable && v.InstanceID == nil
		})).Return(nil).Once()
		mockAuditSvc.On("Log", mock.Anything, userID, "volume.detach", "volume", mock.Anything, mock.Anything).Return(nil).Once()

		err := svc.DetachVolume(ctx, volID.String())
		require.NoError(t, err)
	})

	t.Run("Attach Compute Failure Rollback", func(t *testing.T) {
		volID := uuid.New()
		instID := uuid.New()

		vol := &domain.Volume{ID: volID, Status: domain.VolumeStatusAvailable, UserID: userID}
		inst := &domain.Instance{ID: instID, ContainerID: "old-container-id"}

		mockRepo.On("GetByID", mock.Anything, volID).Return(vol, nil).Once()
		mockStorage.On("AttachVolume", mock.Anything, mock.Anything, instID.String()).Return("/dev/vdb", nil).Once()
		mockInstanceRepo.On("GetByID", mock.Anything, instID).Return(inst, nil).Once()
		mockCompute.On("AttachVolume", mock.Anything, "old-container-id", "/dev/vdb:/mnt/data:rw").
			Return("", "", errors.New("docker error")).Once()
		mockStorage.On("DetachVolume", mock.Anything, mock.Anything, instID.String()).Return(nil).Once()

		_, err := svc.AttachVolume(ctx, volID.String(), instID.String(), "/mnt/data")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to attach volume to container")
	})

	t.Run("Attach Instance Not Found Rollback", func(t *testing.T) {
		volID := uuid.New()
		instID := uuid.New()

		vol := &domain.Volume{ID: volID, Status: domain.VolumeStatusAvailable, UserID: userID}

		mockRepo.On("GetByID", mock.Anything, volID).Return(vol, nil).Once()
		mockStorage.On("AttachVolume", mock.Anything, mock.Anything, instID.String()).Return("/dev/vdb", nil).Once()
		mockInstanceRepo.On("GetByID", mock.Anything, instID).Return(nil, errors.New("instance not found")).Once()
		mockStorage.On("DetachVolume", mock.Anything, mock.Anything, instID.String()).Return(nil).Once()

		_, err := svc.AttachVolume(ctx, volID.String(), instID.String(), "/mnt/data")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get instance")
	})
}
