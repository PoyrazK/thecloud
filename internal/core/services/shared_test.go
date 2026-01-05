package services_test

import (
	"context"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/core/ports"
	"github.com/stretchr/testify/mock"
)

// MockAutoScalingRepo
type MockAutoScalingRepo struct{ mock.Mock }

func (m *MockAutoScalingRepo) CreateGroup(ctx context.Context, group *domain.ScalingGroup) error {
	args := m.Called(ctx, group)
	return args.Error(0)
}
func (m *MockAutoScalingRepo) GetGroupByID(ctx context.Context, id uuid.UUID) (*domain.ScalingGroup, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ScalingGroup), args.Error(1)
}
func (m *MockAutoScalingRepo) GetGroupByIdempotencyKey(ctx context.Context, key string) (*domain.ScalingGroup, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ScalingGroup), args.Error(1)
}
func (m *MockAutoScalingRepo) ListGroups(ctx context.Context) ([]*domain.ScalingGroup, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.ScalingGroup), args.Error(1)
}
func (m *MockAutoScalingRepo) ListAllGroups(ctx context.Context) ([]*domain.ScalingGroup, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.ScalingGroup), args.Error(1)
}
func (m *MockAutoScalingRepo) CountGroupsByVPC(ctx context.Context, vpcID uuid.UUID) (int, error) {
	args := m.Called(ctx, vpcID)
	return args.Int(0), args.Error(1)
}
func (m *MockAutoScalingRepo) UpdateGroup(ctx context.Context, group *domain.ScalingGroup) error {
	args := m.Called(ctx, group)
	return args.Error(0)
}
func (m *MockAutoScalingRepo) DeleteGroup(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
func (m *MockAutoScalingRepo) CreatePolicy(ctx context.Context, policy *domain.ScalingPolicy) error {
	args := m.Called(ctx, policy)
	return args.Error(0)
}
func (m *MockAutoScalingRepo) GetPoliciesForGroup(ctx context.Context, groupID uuid.UUID) ([]*domain.ScalingPolicy, error) {
	args := m.Called(ctx, groupID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.ScalingPolicy), args.Error(1)
}
func (m *MockAutoScalingRepo) GetAllPolicies(ctx context.Context, groupIDs []uuid.UUID) (map[uuid.UUID][]*domain.ScalingPolicy, error) {
	args := m.Called(ctx, groupIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[uuid.UUID][]*domain.ScalingPolicy), args.Error(1)
}
func (m *MockAutoScalingRepo) UpdatePolicyLastScaled(ctx context.Context, policyID uuid.UUID, t time.Time) error {
	args := m.Called(ctx, policyID, t)
	return args.Error(0)
}
func (m *MockAutoScalingRepo) DeletePolicy(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
func (m *MockAutoScalingRepo) AddInstanceToGroup(ctx context.Context, groupID, instanceID uuid.UUID) error {
	args := m.Called(ctx, groupID, instanceID)
	return args.Error(0)
}
func (m *MockAutoScalingRepo) RemoveInstanceFromGroup(ctx context.Context, groupID, instanceID uuid.UUID) error {
	args := m.Called(ctx, groupID, instanceID)
	return args.Error(0)
}
func (m *MockAutoScalingRepo) GetInstancesInGroup(ctx context.Context, groupID uuid.UUID) ([]uuid.UUID, error) {
	args := m.Called(ctx, groupID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]uuid.UUID), args.Error(1)
}
func (m *MockAutoScalingRepo) GetAllScalingGroupInstances(ctx context.Context, groupIDs []uuid.UUID) (map[uuid.UUID][]uuid.UUID, error) {
	args := m.Called(ctx, groupIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[uuid.UUID][]uuid.UUID), args.Error(1)
}
func (m *MockAutoScalingRepo) GetAverageCPU(ctx context.Context, instanceIDs []uuid.UUID, since time.Time) (float64, error) {
	args := m.Called(ctx, instanceIDs, since)
	return args.Get(0).(float64), args.Error(1)
}

// MockInstanceService
type MockInstanceService struct{ mock.Mock }

func (m *MockInstanceService) LaunchInstance(ctx context.Context, name, image, ports string, vpcID *uuid.UUID, volumes []domain.VolumeAttachment) (*domain.Instance, error) {
	args := m.Called(ctx, name, image, ports, vpcID, volumes)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Instance), args.Error(1)
}
func (m *MockInstanceService) StopInstance(ctx context.Context, idOrName string) error {
	args := m.Called(ctx, idOrName)
	return args.Error(0)
}
func (m *MockInstanceService) ListInstances(ctx context.Context) ([]*domain.Instance, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Instance), args.Error(1)
}
func (m *MockInstanceService) GetInstance(ctx context.Context, idOrName string) (*domain.Instance, error) {
	args := m.Called(ctx, idOrName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Instance), args.Error(1)
}
func (m *MockInstanceService) GetInstanceLogs(ctx context.Context, idOrName string) (string, error) {
	args := m.Called(ctx, idOrName)
	return args.String(0), args.Error(1)
}
func (m *MockInstanceService) GetInstanceStats(ctx context.Context, idOrName string) (*domain.InstanceStats, error) {
	args := m.Called(ctx, idOrName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.InstanceStats), args.Error(1)
}
func (m *MockInstanceService) TerminateInstance(ctx context.Context, idOrName string) error {
	args := m.Called(ctx, idOrName)
	return args.Error(0)
}

// MockLBService
type MockLBService struct{ mock.Mock }

func (m *MockLBService) Create(ctx context.Context, name string, vpcID uuid.UUID, port int, algo string, idempotencyKey string) (*domain.LoadBalancer, error) {
	args := m.Called(ctx, name, vpcID, port, algo, idempotencyKey)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.LoadBalancer), args.Error(1)
}
func (m *MockLBService) Get(ctx context.Context, id uuid.UUID) (*domain.LoadBalancer, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.LoadBalancer), args.Error(1)
}
func (m *MockLBService) List(ctx context.Context) ([]*domain.LoadBalancer, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.LoadBalancer), args.Error(1)
}
func (m *MockLBService) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
func (m *MockLBService) AddTarget(ctx context.Context, lbID, instanceID uuid.UUID, port, weight int) error {
	args := m.Called(ctx, lbID, instanceID, port, weight)
	return args.Error(0)
}
func (m *MockLBService) RemoveTarget(ctx context.Context, lbID, instanceID uuid.UUID) error {
	args := m.Called(ctx, lbID, instanceID)
	return args.Error(0)
}
func (m *MockLBService) ListTargets(ctx context.Context, lbID uuid.UUID) ([]*domain.LBTarget, error) {
	args := m.Called(ctx, lbID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.LBTarget), args.Error(1)
}

// MockEventService
type MockEventService struct{ mock.Mock }

func (m *MockEventService) RecordEvent(ctx context.Context, eType, resourceID, resourceType string, meta map[string]interface{}) error {
	args := m.Called(ctx, eType, resourceID, resourceType, meta)
	return args.Error(0)
}
func (m *MockEventService) ListEvents(ctx context.Context, limit int) ([]*domain.Event, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Event), args.Error(1)
}

// MockClock
type MockClock struct{ mock.Mock }

func (m *MockClock) Now() time.Time {
	args := m.Called()
	return args.Get(0).(time.Time)
}

// MockVpcRepo
type MockVpcRepo struct{ mock.Mock }

func (m *MockVpcRepo) Create(ctx context.Context, vpc *domain.VPC) error {
	args := m.Called(ctx, vpc)
	return args.Error(0)
}
func (m *MockVpcRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.VPC, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.VPC), args.Error(1)
}
func (m *MockVpcRepo) GetByName(ctx context.Context, name string) (*domain.VPC, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.VPC), args.Error(1)
}
func (m *MockVpcRepo) List(ctx context.Context) ([]*domain.VPC, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.VPC), args.Error(1)
}

func (m *MockVpcRepo) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// MockStorageRepo
type MockStorageRepo struct {
	mock.Mock
}

func (m *MockStorageRepo) SaveMeta(ctx context.Context, obj *domain.Object) error {
	args := m.Called(ctx, obj)
	return args.Error(0)
}
func (m *MockStorageRepo) GetMeta(ctx context.Context, bucket, key string) (*domain.Object, error) {
	args := m.Called(ctx, bucket, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Object), args.Error(1)
}
func (m *MockStorageRepo) List(ctx context.Context, bucket string) ([]*domain.Object, error) {
	args := m.Called(ctx, bucket)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Object), args.Error(1)
}
func (m *MockStorageRepo) SoftDelete(ctx context.Context, bucket, key string) error {
	args := m.Called(ctx, bucket, key)
	return args.Error(0)
}

// MockFileStore
type MockFileStore struct {
	mock.Mock
}

func (m *MockFileStore) Write(ctx context.Context, bucket, key string, r io.Reader) (int64, error) {
	args := m.Called(ctx, bucket, key, r)
	return args.Get(0).(int64), args.Error(1)
}
func (m *MockFileStore) Read(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	args := m.Called(ctx, bucket, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(io.ReadCloser), args.Error(1)
}
func (m *MockFileStore) Delete(ctx context.Context, bucket, key string) error {
	args := m.Called(ctx, bucket, key)
	return args.Error(0)
}

// MockVolumeRepo
type MockVolumeRepo struct {
	mock.Mock
}

func (m *MockVolumeRepo) Create(ctx context.Context, v *domain.Volume) error {
	args := m.Called(ctx, v)
	return args.Error(0)
}

func (m *MockVolumeRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Volume, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Volume), args.Error(1)
}

func (m *MockVolumeRepo) GetByName(ctx context.Context, name string) (*domain.Volume, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Volume), args.Error(1)
}

func (m *MockVolumeRepo) List(ctx context.Context) ([]*domain.Volume, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Volume), args.Error(1)
}

func (m *MockVolumeRepo) ListByInstanceID(ctx context.Context, id uuid.UUID) ([]*domain.Volume, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Volume), args.Error(1)
}

func (m *MockVolumeRepo) Update(ctx context.Context, v *domain.Volume) error {
	args := m.Called(ctx, v)
	return args.Error(0)
}

func (m *MockVolumeRepo) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// MockQueueRepo
type MockQueueRepo struct{ mock.Mock }

func (m *MockQueueRepo) Create(ctx context.Context, q *domain.Queue) error {
	args := m.Called(ctx, q)
	return args.Error(0)
}
func (m *MockQueueRepo) GetByID(ctx context.Context, id, userID uuid.UUID) (*domain.Queue, error) {
	args := m.Called(ctx, id, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Queue), args.Error(1)
}
func (m *MockQueueRepo) GetByName(ctx context.Context, name string, userID uuid.UUID) (*domain.Queue, error) {
	args := m.Called(ctx, name, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Queue), args.Error(1)
}
func (m *MockQueueRepo) List(ctx context.Context, userID uuid.UUID) ([]*domain.Queue, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Queue), args.Error(1)
}
func (m *MockQueueRepo) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
func (m *MockQueueRepo) SendMessage(ctx context.Context, queueID uuid.UUID, body string) (*domain.Message, error) {
	args := m.Called(ctx, queueID, body)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Message), args.Error(1)
}
func (m *MockQueueRepo) ReceiveMessages(ctx context.Context, queueID uuid.UUID, maxMessages, visibilityTimeout int) ([]*domain.Message, error) {
	args := m.Called(ctx, queueID, maxMessages, visibilityTimeout)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Message), args.Error(1)
}
func (m *MockQueueRepo) DeleteMessage(ctx context.Context, queueID uuid.UUID, receiptHandle string) error {
	args := m.Called(ctx, queueID, receiptHandle)
	return args.Error(0)
}
func (m *MockQueueRepo) PurgeMessages(ctx context.Context, queueID uuid.UUID) (int64, error) {
	args := m.Called(ctx, queueID)
	return args.Get(0).(int64), args.Error(1)
}
func (m *MockQueueRepo) GetQueueStats(ctx context.Context, queueID uuid.UUID) (int, int, error) {
	args := m.Called(ctx, queueID)
	return args.Int(0), args.Int(1), args.Error(2)
}

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
	args := m.Called(ctx, id)
	return args.Error(0)
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
	args := m.Called(ctx, queueID, receiptHandle)
	return args.Error(0)
}
func (m *MockQueueService) PurgeQueue(ctx context.Context, queueID uuid.UUID) error {
	args := m.Called(ctx, queueID)
	return args.Error(0)
}

// MockNotifyRepo
type MockNotifyRepo struct{ mock.Mock }

func (m *MockNotifyRepo) CreateTopic(ctx context.Context, topic *domain.Topic) error {
	args := m.Called(ctx, topic)
	return args.Error(0)
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
	args := m.Called(ctx, id)
	return args.Error(0)
}
func (m *MockNotifyRepo) CreateSubscription(ctx context.Context, sub *domain.Subscription) error {
	args := m.Called(ctx, sub)
	return args.Error(0)
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
	args := m.Called(ctx, id)
	return args.Error(0)
}
func (m *MockNotifyRepo) SaveMessage(ctx context.Context, msg *domain.NotifyMessage) error {
	args := m.Called(ctx, msg)
	return args.Error(0)
}

// MockCronRepo
type MockCronRepo struct{ mock.Mock }

func (m *MockCronRepo) CreateJob(ctx context.Context, job *domain.CronJob) error {
	args := m.Called(ctx, job)
	return args.Error(0)
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
	args := m.Called(ctx, job)
	return args.Error(0)
}
func (m *MockCronRepo) DeleteJob(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
func (m *MockCronRepo) GetNextJobsToRun(ctx context.Context) ([]*domain.CronJob, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.CronJob), args.Error(1)
}
func (m *MockCronRepo) SaveJobRun(ctx context.Context, run *domain.CronJobRun) error {
	args := m.Called(ctx, run)
	return args.Error(0)
}

// MockGatewayRepo
type MockGatewayRepo struct{ mock.Mock }

func (m *MockGatewayRepo) CreateRoute(ctx context.Context, route *domain.GatewayRoute) error {
	args := m.Called(ctx, route)
	return args.Error(0)
}
func (m *MockGatewayRepo) GetRouteByID(ctx context.Context, id, userID uuid.UUID) (*domain.GatewayRoute, error) {
	args := m.Called(ctx, id, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.GatewayRoute), args.Error(1)
}
func (m *MockGatewayRepo) ListRoutes(ctx context.Context, userID uuid.UUID) ([]*domain.GatewayRoute, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.GatewayRoute), args.Error(1)
}
func (m *MockGatewayRepo) DeleteRoute(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
func (m *MockGatewayRepo) GetAllActiveRoutes(ctx context.Context) ([]*domain.GatewayRoute, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.GatewayRoute), args.Error(1)
}

// MockContainerRepo
type MockContainerRepo struct{ mock.Mock }

func (m *MockContainerRepo) CreateDeployment(ctx context.Context, d *domain.Deployment) error {
	args := m.Called(ctx, d)
	return args.Error(0)
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
	args := m.Called(ctx, d)
	return args.Error(0)
}
func (m *MockContainerRepo) DeleteDeployment(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}
func (m *MockContainerRepo) AddContainer(ctx context.Context, deploymentID, instanceID uuid.UUID) error {
	args := m.Called(ctx, deploymentID, instanceID)
	return args.Error(0)
}
func (m *MockContainerRepo) RemoveContainer(ctx context.Context, deploymentID, instanceID uuid.UUID) error {
	args := m.Called(ctx, deploymentID, instanceID)
	return args.Error(0)
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

// MockAuditRepo
type MockAuditRepo struct{ mock.Mock }

func (m *MockAuditRepo) Create(ctx context.Context, log *domain.AuditLog) error {
	args := m.Called(ctx, log)
	return args.Error(0)
}
func (m *MockAuditRepo) ListByUserID(ctx context.Context, userID uuid.UUID, limit int) ([]*domain.AuditLog, error) {
	args := m.Called(ctx, userID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AuditLog), args.Error(1)
}

// MockAuditService
type MockAuditService struct{ mock.Mock }

func (m *MockAuditService) Log(ctx context.Context, userID uuid.UUID, action, resourceType, resourceID string, details map[string]interface{}) error {
	args := m.Called(ctx, userID, action, resourceType, resourceID, details)
	return args.Error(0)
}
func (m *MockAuditService) ListLogs(ctx context.Context, userID uuid.UUID, limit int) ([]*domain.AuditLog, error) {
	args := m.Called(ctx, userID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AuditLog), args.Error(1)
}
