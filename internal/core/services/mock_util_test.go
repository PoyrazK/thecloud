package services_test

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/stretchr/testify/mock"
)

// MockAccountingRepo
type MockAccountingRepo struct{ mock.Mock }

func (m *MockAccountingRepo) CreateRecord(ctx context.Context, r domain.UsageRecord) error {
	return m.Called(ctx, r).Error(0)
}
func (m *MockAccountingRepo) GetUsageSummary(ctx context.Context, userID uuid.UUID, start, end time.Time) (map[domain.ResourceType]float64, error) {
	args := m.Called(ctx, userID, start, end)
	return args.Get(0).(map[domain.ResourceType]float64), args.Error(1)
}
func (m *MockAccountingRepo) ListRecords(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]domain.UsageRecord, error) {
	args := m.Called(ctx, userID, start, end)
	return args.Get(0).([]domain.UsageRecord), args.Error(1)
}

type MockAccountingRepository = MockAccountingRepo

// MockAuditService
type MockAuditService struct{ mock.Mock }

type MockAuditRepository struct{ mock.Mock }

func (m *MockAuditRepository) Create(ctx context.Context, log *domain.AuditLog) error {
	return m.Called(ctx, log).Error(0)
}
func (m *MockAuditRepository) ListByUserID(ctx context.Context, userID uuid.UUID, limit int) ([]*domain.AuditLog, error) {
	args := m.Called(ctx, userID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AuditLog), args.Error(1)
}

func (m *MockAuditService) Log(ctx context.Context, userID uuid.UUID, action, resourceType, resourceID string, details map[string]interface{}) error {
	return m.Called(ctx, userID, action, resourceType, resourceID, details).Error(0)
}
func (m *MockAuditService) ListLogs(ctx context.Context, userID uuid.UUID, limit int) ([]*domain.AuditLog, error) {
	args := m.Called(ctx, userID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AuditLog), args.Error(1)
}

// MockContainerRepo
type MockContainerRepo struct{ mock.Mock }

func (m *MockContainerRepo) CreateDeployment(ctx context.Context, d *domain.Deployment) error {
	return m.Called(ctx, d).Error(0)
}
func (m *MockContainerRepo) GetDeploymentByID(ctx context.Context, id, userID uuid.UUID) (*domain.Deployment, error) {
	args := m.Called(ctx, id, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Deployment), args.Error(1)
}
func (m *MockContainerRepo) ListDeployments(ctx context.Context, userID uuid.UUID) ([]*domain.Deployment, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Deployment), args.Error(1)
}
func (m *MockContainerRepo) UpdateDeployment(ctx context.Context, d *domain.Deployment) error {
	return m.Called(ctx, d).Error(0)
}
func (m *MockContainerRepo) DeleteDeployment(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *MockContainerRepo) AddContainer(ctx context.Context, deploymentID, instanceID uuid.UUID) error {
	return m.Called(ctx, deploymentID, instanceID).Error(0)
}
func (m *MockContainerRepo) RemoveContainer(ctx context.Context, deploymentID, instanceID uuid.UUID) error {
	return m.Called(ctx, deploymentID, instanceID).Error(0)
}
func (m *MockContainerRepo) GetContainers(ctx context.Context, deploymentID uuid.UUID) ([]uuid.UUID, error) {
	args := m.Called(ctx, deploymentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]uuid.UUID), args.Error(1)
}
func (m *MockContainerRepo) ListAllDeployments(ctx context.Context) ([]*domain.Deployment, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Deployment), args.Error(1)
}

type MockContainerRepository = MockContainerRepo

// MockCronRepo
type MockCronRepo struct{ mock.Mock }

func (m *MockCronRepo) CreateJob(ctx context.Context, job *domain.CronJob) error {
	return m.Called(ctx, job).Error(0)
}
func (m *MockCronRepo) GetJobByID(ctx context.Context, id, userID uuid.UUID) (*domain.CronJob, error) {
	args := m.Called(ctx, id, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.CronJob), args.Error(1)
}
func (m *MockCronRepo) ListJobs(ctx context.Context, userID uuid.UUID) ([]*domain.CronJob, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.CronJob), args.Error(1)
}
func (m *MockCronRepo) UpdateJob(ctx context.Context, job *domain.CronJob) error {
	return m.Called(ctx, job).Error(0)
}
func (m *MockCronRepo) DeleteJob(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *MockCronRepo) ClaimNextJobsToRun(ctx context.Context, lockTimeout time.Duration) ([]*domain.CronJob, error) {
	args := m.Called(ctx, lockTimeout)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.CronJob), args.Error(1)
}
func (m *MockCronRepo) CompleteJobRun(ctx context.Context, run *domain.CronJobRun, job *domain.CronJob, nextRunAt time.Time) error {
	return m.Called(ctx, run, job, nextRunAt).Error(0)
}
func (m *MockCronRepo) ReapStaleClaims(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Get(0).(int), args.Error(1)
}
func (m *MockCronRepo) GetJobRuns(ctx context.Context, jobID uuid.UUID, limit int) ([]*domain.CronJobRun, error) {
	args := m.Called(ctx, jobID, limit)
	return args.Get(0).([]*domain.CronJobRun), args.Error(1)
}

type MockCronRepository = MockCronRepo

// MockDatabaseRepo
type MockDatabaseRepo struct{ mock.Mock }

func (m *MockDatabaseRepo) Create(ctx context.Context, db *domain.Database) error {
	return m.Called(ctx, db).Error(0)
}
func (m *MockDatabaseRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Database, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Database), args.Error(1)
}
func (m *MockDatabaseRepo) List(ctx context.Context) ([]*domain.Database, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*domain.Database), args.Error(1)
}
func (m *MockDatabaseRepo) ListReplicas(ctx context.Context, primaryID uuid.UUID) ([]*domain.Database, error) {
	args := m.Called(ctx, primaryID)
	return args.Get(0).([]*domain.Database), args.Error(1)
}
func (m *MockDatabaseRepo) Update(ctx context.Context, db *domain.Database) error {
	return m.Called(ctx, db).Error(0)
}
func (m *MockDatabaseRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

// MockElasticIPRepo
type MockElasticIPRepo struct{ mock.Mock }

func (m *MockElasticIPRepo) Create(ctx context.Context, eip *domain.ElasticIP) error {
	return m.Called(ctx, eip).Error(0)
}
func (m *MockElasticIPRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.ElasticIP, error) {
	args := m.Called(ctx, id)
	r0, _ := args.Get(0).(*domain.ElasticIP)
	return r0, args.Error(1)
}
func (m *MockElasticIPRepo) GetByPublicIP(ctx context.Context, ip string) (*domain.ElasticIP, error) {
	args := m.Called(ctx, ip)
	r0, _ := args.Get(0).(*domain.ElasticIP)
	return r0, args.Error(1)
}
func (m *MockElasticIPRepo) GetByInstanceID(ctx context.Context, id uuid.UUID) (*domain.ElasticIP, error) {
	args := m.Called(ctx, id)
	r0, _ := args.Get(0).(*domain.ElasticIP)
	return r0, args.Error(1)
}
func (m *MockElasticIPRepo) List(ctx context.Context) ([]*domain.ElasticIP, error) {
	args := m.Called(ctx)
	r0, _ := args.Get(0).([]*domain.ElasticIP)
	return r0, args.Error(1)
}
func (m *MockElasticIPRepo) Update(ctx context.Context, eip *domain.ElasticIP) error {
	return m.Called(ctx, eip).Error(0)
}
func (m *MockElasticIPRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

// MockEventRepo
type MockEventRepo struct{ mock.Mock }

func (m *MockEventRepo) Create(ctx context.Context, e *domain.Event) error {
	return m.Called(ctx, e).Error(0)
}
func (m *MockEventRepo) List(ctx context.Context, limit int) ([]*domain.Event, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Event), args.Error(1)
}
func (m *MockEventRepo) ListByUserID(ctx context.Context, id uuid.UUID, limit int) ([]*domain.Event, error) {
	args := m.Called(ctx, id, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Event), args.Error(1)
}
func (m *MockEventRepo) ListByTenantID(ctx context.Context, id uuid.UUID, limit int) ([]*domain.Event, error) {
	args := m.Called(ctx, id, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Event), args.Error(1)
}

type MockEventRepository = MockEventRepo

// MockEventService
type MockEventService struct{ mock.Mock }

func (m *MockEventService) RecordEvent(ctx context.Context, eType, resourceID, resourceType string, meta map[string]interface{}) error {
	args := m.Called(ctx, eType, resourceID, resourceType, meta)
	return args.Error(0)
}
func (m *MockEventService) ListEvents(ctx context.Context, limit int) ([]*domain.Event, error) {
	args := m.Called(ctx, limit)
	r0, _ := args.Get(0).([]*domain.Event)
	return r0, args.Error(1)
}

// MockLogRepository
type MockLogRepository struct{ mock.Mock }

func (m *MockLogRepository) Create(ctx context.Context, entries []*domain.LogEntry) error {
	return m.Called(ctx, entries).Error(0)
}
func (m *MockLogRepository) List(ctx context.Context, query domain.LogQuery) ([]*domain.LogEntry, int, error) {
	args := m.Called(ctx, query)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.LogEntry), args.Int(1), args.Error(2)
}
func (m *MockLogRepository) DeleteByAge(ctx context.Context, days int) error {
	return m.Called(ctx, days).Error(0)
}

// MockLogService
type MockLogService struct{ mock.Mock }

func (m *MockLogService) IngestLogs(ctx context.Context, entries []*domain.LogEntry) error {
	return m.Called(ctx, entries).Error(0)
}
func (m *MockLogService) SearchLogs(ctx context.Context, query domain.LogQuery) ([]*domain.LogEntry, int, error) {
	args := m.Called(ctx, query)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.LogEntry), args.Int(1), args.Error(2)
}
func (m *MockLogService) RunRetentionPolicy(ctx context.Context, days int) error {
	return m.Called(ctx, days).Error(0)
}

// MockNotifyRepo
type MockNotifyRepo struct{ mock.Mock }

func (m *MockNotifyRepo) CreateTopic(ctx context.Context, topic *domain.Topic) error {
	return m.Called(ctx, topic).Error(0)
}
func (m *MockNotifyRepo) GetTopicByID(ctx context.Context, id, userID uuid.UUID) (*domain.Topic, error) {
	args := m.Called(ctx, id, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Topic), args.Error(1)
}
func (m *MockNotifyRepo) GetTopicByName(ctx context.Context, name string, userID uuid.UUID) (*domain.Topic, error) {
	args := m.Called(ctx, name, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Topic), args.Error(1)
}
func (m *MockNotifyRepo) ListTopics(ctx context.Context, userID uuid.UUID) ([]*domain.Topic, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Topic), args.Error(1)
}
func (m *MockNotifyRepo) DeleteTopic(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *MockNotifyRepo) CreateSubscription(ctx context.Context, sub *domain.Subscription) error {
	return m.Called(ctx, sub).Error(0)
}
func (m *MockNotifyRepo) GetSubscriptionByID(ctx context.Context, id, userID uuid.UUID) (*domain.Subscription, error) {
	args := m.Called(ctx, id, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Subscription), args.Error(1)
}
func (m *MockNotifyRepo) ListSubscriptions(ctx context.Context, topicID uuid.UUID) ([]*domain.Subscription, error) {
	args := m.Called(ctx, topicID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Subscription), args.Error(1)
}
func (m *MockNotifyRepo) DeleteSubscription(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *MockNotifyRepo) SaveMessage(ctx context.Context, msg *domain.NotifyMessage) error {
	return m.Called(ctx, msg).Error(0)
}

type MockNotifyRepository = MockNotifyRepo

// MockSecretService
type MockSecretService struct{ mock.Mock }

func (m *MockSecretService) CreateSecret(ctx context.Context, name, value, description string) (*domain.Secret, error) {
	args := m.Called(ctx, name, value, description)
	r0, _ := args.Get(0).(*domain.Secret)
	return r0, args.Error(1)
}
func (m *MockSecretService) GetSecret(ctx context.Context, id uuid.UUID) (*domain.Secret, error) {
	args := m.Called(ctx, id)
	r0, _ := args.Get(0).(*domain.Secret)
	return r0, args.Error(1)
}
func (m *MockSecretService) GetSecretByName(ctx context.Context, name string) (*domain.Secret, error) {
	args := m.Called(ctx, name)
	r0, _ := args.Get(0).(*domain.Secret)
	return r0, args.Error(1)
}
func (m *MockSecretService) ListSecrets(ctx context.Context) ([]*domain.Secret, error) {
	args := m.Called(ctx)
	r0, _ := args.Get(0).([]*domain.Secret)
	return r0, args.Error(1)
}
func (m *MockSecretService) DeleteSecret(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *MockSecretService) Encrypt(ctx context.Context, userID uuid.UUID, plain string) (string, error) {
	args := m.Called(ctx, userID, plain)
	return args.String(0), args.Error(1)
}
func (m *MockSecretService) Decrypt(ctx context.Context, userID uuid.UUID, cipher string) (string, error) {
	args := m.Called(ctx, userID, cipher)
	return args.String(0), args.Error(1)
}

// MockStackRepo
type MockStackRepo struct{ mock.Mock }

func (m *MockStackRepo) Create(ctx context.Context, s *domain.Stack) error {
	return m.Called(ctx, s).Error(0)
}
func (m *MockStackRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Stack, error) {
	args := m.Called(ctx, id)
	r0, _ := args.Get(0).(*domain.Stack)
	return r0, args.Error(1)
}
func (m *MockStackRepo) GetByName(ctx context.Context, userID uuid.UUID, name string) (*domain.Stack, error) {
	args := m.Called(ctx, userID, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	r0, _ := args.Get(0).(*domain.Stack)
	return r0, args.Error(1)
}
func (m *MockStackRepo) ListByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.Stack, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]*domain.Stack), args.Error(1)
}
func (m *MockStackRepo) Update(ctx context.Context, s *domain.Stack) error {
	return m.Called(ctx, s).Error(0)
}
func (m *MockStackRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *MockStackRepo) AddResource(ctx context.Context, r *domain.StackResource) error {
	return m.Called(ctx, r).Error(0)
}
func (m *MockStackRepo) ListResources(ctx context.Context, stackID uuid.UUID) ([]domain.StackResource, error) {
	args := m.Called(ctx, stackID)
	r0, _ := args.Get(0).([]domain.StackResource)
	return r0, args.Error(1)
}
func (m *MockStackRepo) DeleteResources(ctx context.Context, stackID uuid.UUID) error {
	return m.Called(ctx, stackID).Error(0)
}

// MockSSHKeyRepo
type MockSSHKeyRepo struct{ mock.Mock }

func (m *MockSSHKeyRepo) Create(ctx context.Context, key *domain.SSHKey) error {
	return m.Called(ctx, key).Error(0)
}
func (m *MockSSHKeyRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.SSHKey, error) {
	args := m.Called(ctx, id)
	r0, _ := args.Get(0).(*domain.SSHKey)
	return r0, args.Error(1)
}
func (m *MockSSHKeyRepo) GetByName(ctx context.Context, tenantID uuid.UUID, name string) (*domain.SSHKey, error) {
	args := m.Called(ctx, tenantID, name)
	r0, _ := args.Get(0).(*domain.SSHKey)
	return r0, args.Error(1)
}
func (m *MockSSHKeyRepo) List(ctx context.Context, tenantID uuid.UUID) ([]*domain.SSHKey, error) {
	args := m.Called(ctx, tenantID)
	r0, _ := args.Get(0).([]*domain.SSHKey)
	return r0, args.Error(1)
}
func (m *MockSSHKeyRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

// MockTaskQueue
type MockTaskQueue struct{ mock.Mock }

func (m *MockTaskQueue) Enqueue(ctx context.Context, queue string, payload interface{}) error {
	return m.Called(ctx, queue, payload).Error(0)
}
func (m *MockTaskQueue) Dequeue(ctx context.Context, queue string) (string, error) {
	args := m.Called(ctx, queue)
	return args.String(0), args.Error(1)
}

// MockQueueRepo
type MockQueueRepo struct{ mock.Mock }

func (m *MockQueueRepo) Create(ctx context.Context, q *domain.Queue) error {
	args := m.Called(ctx, q)
	return args.Error(0)
}
func (m *MockQueueRepo) GetByID(ctx context.Context, id, tenantID uuid.UUID) (*domain.Queue, error) {
	args := m.Called(ctx, id, tenantID)
	r0, _ := args.Get(0).(*domain.Queue)
	return r0, args.Error(1)
}
func (m *MockQueueRepo) GetByName(ctx context.Context, name string, tenantID uuid.UUID) (*domain.Queue, error) {
	args := m.Called(ctx, name, tenantID)
	r0, _ := args.Get(0).(*domain.Queue)
	return r0, args.Error(1)
}
func (m *MockQueueRepo) List(ctx context.Context, tenantID uuid.UUID) ([]*domain.Queue, error) {
	args := m.Called(ctx, tenantID)
	r0, _ := args.Get(0).([]*domain.Queue)
	return r0, args.Error(1)
}
func (m *MockQueueRepo) Delete(ctx context.Context, id, tenantID uuid.UUID) error {
	return m.Called(ctx, id, tenantID).Error(0)
}
func (m *MockQueueRepo) SendMessage(ctx context.Context, queueID uuid.UUID, body string) (*domain.Message, error) {
	args := m.Called(ctx, queueID, body)
	r0, _ := args.Get(0).(*domain.Message)
	return r0, args.Error(1)
}
func (m *MockQueueRepo) ReceiveMessages(ctx context.Context, queueID uuid.UUID, maxMessages, visibilityTimeout int) ([]*domain.Message, error) {
	args := m.Called(ctx, queueID, maxMessages, visibilityTimeout)
	r0, _ := args.Get(0).([]*domain.Message)
	return r0, args.Error(1)
}
func (m *MockQueueRepo) DeleteMessage(ctx context.Context, queueID uuid.UUID, receiptHandle string) error {
	return m.Called(ctx, queueID, receiptHandle).Error(0)
}
func (m *MockQueueRepo) PurgeMessages(ctx context.Context, queueID uuid.UUID) (int64, error) {
	args := m.Called(ctx, queueID)
	return int64(args.Int(0)), args.Error(1)
}

type MockQueueRepository = MockQueueRepo

// MockQueueService
type MockQueueService struct{ mock.Mock }

func (m *MockQueueService) CreateQueue(ctx context.Context, name string, opts *ports.CreateQueueOptions) (*domain.Queue, error) {
	args := m.Called(ctx, name, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Queue), args.Error(1)
}
func (m *MockQueueService) GetQueue(ctx context.Context, id uuid.UUID) (*domain.Queue, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Queue), args.Error(1)
}
func (m *MockQueueService) ListQueues(ctx context.Context) ([]*domain.Queue, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Queue), args.Error(1)
}
func (m *MockQueueService) DeleteQueue(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *MockQueueService) SendMessage(ctx context.Context, queueID uuid.UUID, body string) (*domain.Message, error) {
	args := m.Called(ctx, queueID, body)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Message), args.Error(1)
}
func (m *MockQueueService) ReceiveMessages(ctx context.Context, queueID uuid.UUID, maxMessages int) ([]*domain.Message, error) {
	args := m.Called(ctx, queueID, maxMessages)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Message), args.Error(1)
}
func (m *MockQueueService) DeleteMessage(ctx context.Context, queueID uuid.UUID, receiptHandle string) error {
	return m.Called(ctx, queueID, receiptHandle).Error(0)
}
func (m *MockQueueService) PurgeQueue(ctx context.Context, queueID uuid.UUID) error {
	return m.Called(ctx, queueID).Error(0)
}

// MockSecretRepo
type MockSecretRepo struct{ mock.Mock }

func (m *MockSecretRepo) Create(ctx context.Context, secret *domain.Secret) error {
	return m.Called(ctx, secret).Error(0)
}
func (m *MockSecretRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Secret, error) {
	args := m.Called(ctx, id)
	r0, _ := args.Get(0).(*domain.Secret)
	return r0, args.Error(1)
}
func (m *MockSecretRepo) GetByName(ctx context.Context, name string) (*domain.Secret, error) {
	args := m.Called(ctx, name)
	r0, _ := args.Get(0).(*domain.Secret)
	return r0, args.Error(1)
}
func (m *MockSecretRepo) List(ctx context.Context) ([]*domain.Secret, error) {
	args := m.Called(ctx)
	r0, _ := args.Get(0).([]*domain.Secret)
	return r0, args.Error(1)
}
func (m *MockSecretRepo) Update(ctx context.Context, secret *domain.Secret) error {
	return m.Called(ctx, secret).Error(0)
}
func (m *MockSecretRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

// MockRealtimePublisher
type MockRealtimePublisher struct{ mock.Mock }

func (m *MockRealtimePublisher) PublishEvent(ctx context.Context, event *domain.WSEvent, tenantID uuid.UUID, userID *uuid.UUID) error {
	return m.Called(ctx, event, tenantID, userID).Error(0)
}

// MockFunctionScheduleRepo
type MockFunctionScheduleRepo struct{ mock.Mock }

func (m *MockFunctionScheduleRepo) Create(ctx context.Context, s *domain.FunctionSchedule) error {
	return m.Called(ctx, s).Error(0)
}
func (m *MockFunctionScheduleRepo) GetByID(ctx context.Context, id, userID, tenantID uuid.UUID) (*domain.FunctionSchedule, error) {
	args := m.Called(ctx, id, userID, tenantID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.FunctionSchedule), args.Error(1)
}
func (m *MockFunctionScheduleRepo) List(ctx context.Context, userID, tenantID uuid.UUID) ([]*domain.FunctionSchedule, error) {
	args := m.Called(ctx, userID, tenantID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.FunctionSchedule), args.Error(1)
}
func (m *MockFunctionScheduleRepo) Update(ctx context.Context, s *domain.FunctionSchedule) error {
	return m.Called(ctx, s).Error(0)
}
func (m *MockFunctionScheduleRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *MockFunctionScheduleRepo) ClaimNextSchedulesToRun(ctx context.Context, lockTimeout time.Duration) ([]*domain.FunctionSchedule, error) {
	args := m.Called(ctx, lockTimeout)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.FunctionSchedule), args.Error(1)
}
func (m *MockFunctionScheduleRepo) CompleteScheduleRun(ctx context.Context, run *domain.FunctionScheduleRun, schedule *domain.FunctionSchedule, nextRunAt time.Time) error {
	return m.Called(ctx, run, schedule, nextRunAt).Error(0)
}
func (m *MockFunctionScheduleRepo) ReapStaleClaims(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Int(0), args.Error(1)
}
func (m *MockFunctionScheduleRepo) GetScheduleRuns(ctx context.Context, scheduleID uuid.UUID, limit int) ([]*domain.FunctionScheduleRun, error) {
	args := m.Called(ctx, scheduleID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.FunctionScheduleRun), args.Error(1)
}

type MockFunctionScheduleRepository = MockFunctionScheduleRepo

// MockFunctionService (for worker tests)
type MockFunctionService struct{ mock.Mock }

func (m *MockFunctionService) CreateFunction(ctx context.Context, name, runtime, handler string, code []byte) (*domain.Function, error) {
	args := m.Called(ctx, name, runtime, handler, code)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Function), args.Error(1)
}
func (m *MockFunctionService) GetFunction(ctx context.Context, id uuid.UUID) (*domain.Function, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Function), args.Error(1)
}
func (m *MockFunctionService) ListFunctions(ctx context.Context) ([]*domain.Function, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Function), args.Error(1)
}
func (m *MockFunctionService) DeleteFunction(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *MockFunctionService) InvokeFunction(ctx context.Context, id uuid.UUID, payload []byte, async bool) (*domain.Invocation, error) {
	args := m.Called(ctx, id, payload, async)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Invocation), args.Error(1)
}
func (m *MockFunctionService) GetFunctionLogs(ctx context.Context, id uuid.UUID, limit int) ([]*domain.Invocation, error) {
	args := m.Called(ctx, id, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Invocation), args.Error(1)
}
func (m *MockFunctionService) UpdateFunction(ctx context.Context, id uuid.UUID, req *domain.FunctionUpdate) (*domain.Function, error) {
	args := m.Called(ctx, id, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Function), args.Error(1)
}
func (m *MockFunctionService) GetDLQInvocations(ctx context.Context, id uuid.UUID) ([]*domain.Invocation, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Invocation), args.Error(1)
}
func (m *MockFunctionService) RetryDLQInvocation(ctx context.Context, functionID, invocationID uuid.UUID) (*domain.Invocation, error) {
	args := m.Called(ctx, functionID, invocationID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Invocation), args.Error(1)
}

type MockFunctionScheduleService = MockFunctionScheduleServiceImpl

type MockFunctionScheduleServiceImpl struct{ mock.Mock }

func (m *MockFunctionScheduleServiceImpl) CreateSchedule(ctx context.Context, functionID uuid.UUID, name, schedule string, payload []byte) (*domain.FunctionSchedule, error) {
	args := m.Called(ctx, functionID, name, schedule, payload)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.FunctionSchedule), args.Error(1)
}
func (m *MockFunctionScheduleServiceImpl) ListSchedules(ctx context.Context) ([]*domain.FunctionSchedule, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.FunctionSchedule), args.Error(1)
}
func (m *MockFunctionScheduleServiceImpl) GetSchedule(ctx context.Context, id uuid.UUID) (*domain.FunctionSchedule, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.FunctionSchedule), args.Error(1)
}
func (m *MockFunctionScheduleServiceImpl) DeleteSchedule(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *MockFunctionScheduleServiceImpl) PauseSchedule(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *MockFunctionScheduleServiceImpl) ResumeSchedule(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}
func (m *MockFunctionScheduleServiceImpl) GetScheduleRuns(ctx context.Context, id uuid.UUID, limit int) ([]*domain.FunctionScheduleRun, error) {
	args := m.Called(ctx, id, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.FunctionScheduleRun), args.Error(1)
}
