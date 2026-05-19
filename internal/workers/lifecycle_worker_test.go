package workers

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/stretchr/testify/assert"
)

type fakeLifecycleRepo struct {
	rules []*domain.LifecycleRule
	err   error
}

func (f *fakeLifecycleRepo) Create(ctx context.Context, rule *domain.LifecycleRule) error { return nil }
func (f *fakeLifecycleRepo) Get(ctx context.Context, id uuid.UUID) (*domain.LifecycleRule, error) {
	return nil, nil
}
func (f *fakeLifecycleRepo) List(ctx context.Context, bucketName string) ([]*domain.LifecycleRule, error) {
	return nil, nil
}
func (f *fakeLifecycleRepo) Delete(ctx context.Context, id uuid.UUID) error { return nil }
func (f *fakeLifecycleRepo) GetEnabledRules(ctx context.Context) ([]*domain.LifecycleRule, error) {
	return f.rules, f.err
}

type fakeLifecycleStorageService struct {
	objects      []*domain.Object
	listErr      error
	deleteErr    error
	deletedKeys  []string
	deletedMutex sync.Mutex
}

func (f *fakeLifecycleStorageService) ListObjects(ctx context.Context, bucket string) ([]*domain.Object, error) {
	return f.objects, f.listErr
}
func (f *fakeLifecycleStorageService) DeleteObject(ctx context.Context, bucket, key string) error {
	f.deletedMutex.Lock()
	defer f.deletedMutex.Unlock()
	f.deletedKeys = append(f.deletedKeys, key)
	return f.deleteErr
}
func (f *fakeLifecycleStorageService) DeletedKeys() []string {
	f.deletedMutex.Lock()
	defer f.deletedMutex.Unlock()
	return append([]string(nil), f.deletedKeys...)
}

func (f *fakeLifecycleStorageService) Upload(ctx context.Context, bucket, key string, r io.Reader, providedChecksum string) (*domain.Object, error) {
	return nil, nil
}
func (f *fakeLifecycleStorageService) Download(ctx context.Context, bucket, key string) (io.ReadCloser, *domain.Object, error) {
	return nil, nil, nil
}
func (f *fakeLifecycleStorageService) DownloadVersion(ctx context.Context, bucket, key, versionID string) (io.ReadCloser, *domain.Object, error) {
	return nil, nil, nil
}
func (f *fakeLifecycleStorageService) ListVersions(ctx context.Context, bucket, key string) ([]*domain.Object, error) {
	return nil, nil
}
func (f *fakeLifecycleStorageService) DeleteVersion(ctx context.Context, bucket, key, versionID string) error {
	return nil
}
func (f *fakeLifecycleStorageService) CreateBucket(ctx context.Context, name string, isPublic bool) (*domain.Bucket, error) {
	return nil, nil
}
func (f *fakeLifecycleStorageService) GetBucket(ctx context.Context, name string) (*domain.Bucket, error) {
	return nil, nil
}
func (f *fakeLifecycleStorageService) DeleteBucket(ctx context.Context, name string, force bool) error {
	return nil
}
func (f *fakeLifecycleStorageService) ListBuckets(ctx context.Context) ([]*domain.Bucket, error) {
	return nil, nil
}
func (f *fakeLifecycleStorageService) SetBucketVersioning(ctx context.Context, name string, enabled bool) error {
	return nil
}
func (f *fakeLifecycleStorageService) GetClusterStatus(ctx context.Context) (*domain.StorageCluster, error) {
	return nil, nil
}
func (f *fakeLifecycleStorageService) CreateMultipartUpload(ctx context.Context, bucket, key string) (*domain.MultipartUpload, error) {
	return nil, nil
}
func (f *fakeLifecycleStorageService) UploadPart(ctx context.Context, uploadID uuid.UUID, partNumber int, r io.Reader, providedChecksum string) (*domain.Part, error) {
	return nil, nil
}
func (f *fakeLifecycleStorageService) CompleteMultipartUpload(ctx context.Context, uploadID uuid.UUID) (*domain.Object, error) {
	return nil, nil
}
func (f *fakeLifecycleStorageService) AbortMultipartUpload(ctx context.Context, uploadID uuid.UUID) error {
	return nil
}
func (f *fakeLifecycleStorageService) CleanupDeleted(ctx context.Context, limit int) (int, error) {
	return 0, nil
}
func (f *fakeLifecycleStorageService) CleanupPendingUploads(ctx context.Context, olderThan time.Duration, limit int) (int, error) {
	return 0, nil
}
func (f *fakeLifecycleStorageService) GeneratePresignedURL(ctx context.Context, bucket, key, method string, expiry time.Duration) (*domain.PresignedURL, error) {
	return nil, nil
}

func TestLifecycleWorkerProcessRulesDeletesExpired(t *testing.T) {
	repo := &fakeLifecycleRepo{
		rules: []*domain.LifecycleRule{
			{
				ID:             uuid.New(),
				BucketName:     "logs",
				Prefix:         "app/",
				ExpirationDays: 1,
				UserID:         uuid.New(),
			},
		},
	}

	old := time.Now().UTC().Add(-48 * time.Hour)
	newer := time.Now().UTC().Add(-12 * time.Hour)

	storageSvc := &fakeLifecycleStorageService{
		objects: []*domain.Object{
			{Key: "app/old.log", CreatedAt: old},
			{Key: "app/new.log", CreatedAt: newer},
			{Key: "other/old.log", CreatedAt: old},
		},
	}

	worker := &LifecycleWorker{
		lifecycleRepo: repo,
		storageSvc:    storageSvc,
		storageRepo:   nil,
		logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	worker.processRules(context.Background())

	deleted := storageSvc.DeletedKeys()
	assert.Equal(t, []string{"app/old.log"}, deleted)
}

func TestLifecycleWorkerProcessRulesListError(t *testing.T) {
	repo := &fakeLifecycleRepo{rules: []*domain.LifecycleRule{{ID: uuid.New(), BucketName: "logs", UserID: uuid.New()}}}
	storageSvc := &fakeLifecycleStorageService{listErr: io.EOF}

	worker := &LifecycleWorker{
		lifecycleRepo: repo,
		storageSvc:    storageSvc,
		storageRepo:   nil,
		logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	worker.processRules(context.Background())

	assert.Empty(t, storageSvc.DeletedKeys())
}

func TestLifecycleWorkerProcessRulesRepoError(t *testing.T) {
	repo := &fakeLifecycleRepo{err: io.EOF}
	storageSvc := &fakeLifecycleStorageService{}

	worker := &LifecycleWorker{
		lifecycleRepo: repo,
		storageSvc:    storageSvc,
		storageRepo:   nil,
		logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	worker.processRules(context.Background())

	assert.Empty(t, storageSvc.DeletedKeys())
}

func TestLifecycleWorkerProcessRulesDeleteError(t *testing.T) {
	repo := &fakeLifecycleRepo{
		rules: []*domain.LifecycleRule{
			{
				ID:             uuid.New(),
				BucketName:     "logs",
				ExpirationDays: 1,
				UserID:         uuid.New(),
			},
		},
	}

	old := time.Now().UTC().Add(-48 * time.Hour)
	storageSvc := &fakeLifecycleStorageService{
		objects:   []*domain.Object{{Key: "old.log", CreatedAt: old}},
		deleteErr: io.EOF,
	}

	worker := &LifecycleWorker{
		lifecycleRepo: repo,
		storageSvc:    storageSvc,
		storageRepo:   nil,
		logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	worker.processRules(context.Background())

	deleted := storageSvc.DeletedKeys()
	assert.Len(t, deleted, 1)
}

func TestLifecycleWorkerRun(t *testing.T) {
	repo := &fakeLifecycleRepo{}
	storageSvc := &fakeLifecycleStorageService{}
	worker := NewLifecycleWorker(repo, storageSvc, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	worker.interval = 10 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)

	go worker.Run(ctx, &wg)

	// Let it run for a bit
	time.Sleep(50 * time.Millisecond)
	cancel()

	wg.Wait()
}

func TestLifecycleWorkerProcessRulesCapEnforcement(t *testing.T) {
	repo := &fakeLifecycleRepo{
		rules: []*domain.LifecycleRule{
			{
				ID:             uuid.New(),
				BucketName:     "large-bucket",
				ExpirationDays: 1,
				UserID:         uuid.New(),
			},
		},
	}

	old := time.Now().UTC().Add(-48 * time.Hour)
	// Create more objects than the cap to verify only capped number is processed
	objects := make([]*domain.Object, maxObjectsPerRulePerTick+1000)
	for i := range objects {
		objects[i] = &domain.Object{Key: "key-" + string(rune(i)), CreatedAt: old}
	}

	storageSvc := &fakeLifecycleStorageService{objects: objects}
	worker := &LifecycleWorker{
		lifecycleRepo: repo,
		storageSvc:    storageSvc,
		storageRepo:   nil,
		logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	worker.processRules(context.Background())

	// Only maxObjectsPerRulePerTick objects should have been evaluated
	deleted := storageSvc.DeletedKeys()
	assert.Len(t, deleted, maxObjectsPerRulePerTick, "only capped number of objects should be processed")
}

func TestLifecycleWorkerProcessRulesNonUTCObjectTimestamp(t *testing.T) {
	repo := &fakeLifecycleRepo{
		rules: []*domain.LifecycleRule{
			{
				ID:             uuid.New(),
				BucketName:     "tz-test",
				ExpirationDays: 1,
				UserID:         uuid.New(),
			},
		},
	}

	// Create an object with non-UTC timezone (UTC+5)
	loc := time.FixedZone("UTC+5", 5*60*60)
	old := time.Now().In(loc).Add(-48 * time.Hour)

	storageSvc := &fakeLifecycleStorageService{
		objects: []*domain.Object{
			{Key: "tz.log", CreatedAt: old},
		},
	}
	worker := &LifecycleWorker{
		lifecycleRepo: repo,
		storageSvc:    storageSvc,
		storageRepo:   nil,
		logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	worker.processRules(context.Background())

	// Should still correctly identify as expired since .UTC() is called internally
	deleted := storageSvc.DeletedKeys()
	assert.Equal(t, []string{"tz.log"}, deleted)
}
