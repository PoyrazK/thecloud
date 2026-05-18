package coordinator

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	pb "github.com/poyrazk/thecloud/internal/storage/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const (
	node1 = "node-1"
	node2 = "node-2"
	node3 = "node-3"
)

// MockStoreClient implements pb.StorageNode_StoreClient
type MockStoreClient struct {
	mock.Mock
	grpc.ClientStream
}

func (m *MockStoreClient) Send(req *pb.StoreRequest) error {
	args := m.Called(req)
	return args.Error(0)
}

func (m *MockStoreClient) CloseAndRecv() (*pb.StoreResponse, error) {
	args := m.Called()
	r0, _ := args.Get(0).(*pb.StoreResponse)
	return r0, args.Error(1)
}

func (m *MockStoreClient) Context() context.Context     { return context.Background() }
func (m *MockStoreClient) Header() (metadata.MD, error) { return nil, nil }
func (m *MockStoreClient) Trailer() metadata.MD         { return nil }
func (m *MockStoreClient) CloseSend() error             { return nil }

// MockRetrieveClient implements pb.StorageNode_RetrieveClient
type MockRetrieveClient struct {
	mock.Mock
	grpc.ClientStream
	resps []*pb.RetrieveResponse
	idx   int
}

func (m *MockRetrieveClient) Recv() (*pb.RetrieveResponse, error) {
	if m.idx >= len(m.resps) {
		return nil, io.EOF
	}
	r := m.resps[m.idx]
	m.idx++
	return r, nil
}

func (m *MockRetrieveClient) Context() context.Context     { return context.Background() }
func (m *MockRetrieveClient) Header() (metadata.MD, error) { return nil, nil }
func (m *MockRetrieveClient) Trailer() metadata.MD         { return nil }
func (m *MockRetrieveClient) CloseSend() error             { return nil }

// MockStorageNodeClient
type MockStorageNodeClient struct {
	mock.Mock
}

func (m *MockStorageNodeClient) Store(ctx context.Context, opts ...grpc.CallOption) (pb.StorageNode_StoreClient, error) {
	args := m.Called(ctx)
	r0, _ := args.Get(0).(pb.StorageNode_StoreClient)
	return r0, args.Error(1)
}

func (m *MockStorageNodeClient) Retrieve(ctx context.Context, in *pb.RetrieveRequest, opts ...grpc.CallOption) (pb.StorageNode_RetrieveClient, error) {
	args := m.Called(ctx, in)
	r0, _ := args.Get(0).(pb.StorageNode_RetrieveClient)
	return r0, args.Error(1)
}

func (m *MockStorageNodeClient) Delete(ctx context.Context, in *pb.DeleteRequest, opts ...grpc.CallOption) (*pb.DeleteResponse, error) {
	args := m.Called(ctx, in)
	r0, _ := args.Get(0).(*pb.DeleteResponse)
	return r0, args.Error(1)
}

func (m *MockStorageNodeClient) Gossip(ctx context.Context, in *pb.GossipMessage, opts ...grpc.CallOption) (*pb.GossipResponse, error) {
	args := m.Called(ctx, in)
	r0, _ := args.Get(0).(*pb.GossipResponse)
	return r0, args.Error(1)
}

func (m *MockStorageNodeClient) GetClusterStatus(ctx context.Context, in *pb.Empty, opts ...grpc.CallOption) (*pb.ClusterStatusResponse, error) {
	args := m.Called(ctx, in)
	r0, _ := args.Get(0).(*pb.ClusterStatusResponse)
	return r0, args.Error(1)
}

func (m *MockStorageNodeClient) Assemble(ctx context.Context, in *pb.AssembleRequest, opts ...grpc.CallOption) (*pb.AssembleResponse, error) {
	args := m.Called(ctx, in)
	r0, _ := args.Get(0).(*pb.AssembleResponse)
	return r0, args.Error(1)
}

func TestCoordinatorWriteQuorum_TCs(t *testing.T) {
	tests := []struct {
		name          string
		setupMocks    func(m1, m2, m3 *MockStorageNodeClient)
		expectedError string
		expectedSize  int64
	}{
		{
			name: "Success_AllNodes",
			setupMocks: func(m1, m2, m3 *MockStorageNodeClient) {
				setupSuccess := func(m *MockStorageNodeClient) {
					sm := new(MockStoreClient)
					sm.On("Send", mock.Anything).Return(nil)
					sm.On("CloseAndRecv").Return(&pb.StoreResponse{Success: true}, nil)
					m.On("Store", mock.Anything).Return(sm, nil)
				}
				setupSuccess(m1)
				setupSuccess(m2)
				setupSuccess(m3)
			},
			expectedSize: 5,
		},
		{
			name: "Failure_QuorumNotMet",
			setupMocks: func(m1, m2, m3 *MockStorageNodeClient) {
				// m1 fails init
				m1.On("Store", mock.Anything).Return(nil, errors.New("init failed"))
				// m2 fails Send
				sm2 := new(MockStoreClient)
				sm2.On("Send", mock.Anything).Return(errors.New("send failed"))
				m2.On("Store", mock.Anything).Return(sm2, nil)
				// m3 succeeds
				sm3 := new(MockStoreClient)
				sm3.On("Send", mock.Anything).Return(nil)
				sm3.On("CloseAndRecv").Return(&pb.StoreResponse{Success: true}, nil)
				m3.On("Store", mock.Anything).Return(sm3, nil)
			},
			expectedError: "write quorum failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ring := NewConsistentHashRing(10)
			ring.AddNode(node1)
			ring.AddNode(node2)
			ring.AddNode(node3)

			c1, c2, c3 := new(MockStorageNodeClient), new(MockStorageNodeClient), new(MockStorageNodeClient)
			tt.setupMocks(c1, c2, c3)

			clients := map[string]pb.StorageNodeClient{node1: c1, node2: c2, node3: c3}
			coord := NewCoordinator(context.Background(), ring, clients, 3)
			defer coord.Stop()

			n, err := coord.Write(context.Background(), "b", "k", bytes.NewReader([]byte("hello")))
			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedSize, n)
			}
		})
	}
}

func TestCoordinatorReadRepair(t *testing.T) {
	ring := NewConsistentHashRing(10)
	ring.AddNode(node1)
	ring.AddNode(node2)
	ring.AddNode(node3)

	c1, c2, c3 := new(MockStorageNodeClient), new(MockStorageNodeClient), new(MockStorageNodeClient)
	clients := map[string]pb.StorageNodeClient{node1: c1, node2: c2, node3: c3}
	coord := NewCoordinator(context.Background(), ring, clients, 3)
	defer coord.Stop()

	tsNew := time.Now().UnixNano()
	tsOld := tsNew - 1000

	// Node 1: Latest data
	rm1 := &MockRetrieveClient{resps: []*pb.RetrieveResponse{
		{Payload: &pb.RetrieveResponse_Metadata{Metadata: &pb.RetrieveMetadata{Found: true, Timestamp: tsNew}}},
		{Payload: &pb.RetrieveResponse_ChunkData{ChunkData: []byte("new")}},
	}}
	c1.On("Retrieve", mock.Anything, mock.Anything).Return(rm1, nil)

	// Node 2: Old data
	rm2 := &MockRetrieveClient{resps: []*pb.RetrieveResponse{
		{Payload: &pb.RetrieveResponse_Metadata{Metadata: &pb.RetrieveMetadata{Found: true, Timestamp: tsOld}}},
		{Payload: &pb.RetrieveResponse_ChunkData{ChunkData: []byte("old")}},
	}}
	c2.On("Retrieve", mock.Anything, mock.Anything).Return(rm2, nil)

	// Node 3: Not found
	rm3 := &MockRetrieveClient{resps: []*pb.RetrieveResponse{
		{Payload: &pb.RetrieveResponse_Metadata{Metadata: &pb.RetrieveMetadata{Found: false}}},
	}}
	c3.On("Retrieve", mock.Anything, mock.Anything).Return(rm3, nil)

	// Repairs expected for c2 and c3 with independent mocks
	smRepairC2 := new(MockStoreClient)
	smRepairC2.On("Send", mock.Anything).Return(nil)
	smRepairC2.On("CloseAndRecv").Return(&pb.StoreResponse{Success: true}, nil)
	c2.On("Store", mock.Anything).Return(smRepairC2, nil)

	smRepairC3 := new(MockStoreClient)
	smRepairC3.On("Send", mock.Anything).Return(nil)
	smRepairC3.On("CloseAndRecv").Return(&pb.StoreResponse{Success: true}, nil)
	c3.On("Store", mock.Anything).Return(smRepairC3, nil)

	r, err := coord.Read(context.Background(), "b", "k")
	require.NoError(t, err)

	data, err := io.ReadAll(r)
	require.NoError(t, err)
	assert.Equal(t, "new", string(data))
	_ = r.Close()

	// Wait for async repair
	time.Sleep(200 * time.Millisecond)
	c2.AssertCalled(t, "Store", mock.Anything)
	c3.AssertCalled(t, "Store", mock.Anything)
}

func TestCoordinatorDelete(t *testing.T) {
	ring := NewConsistentHashRing(10)
	ring.AddNode(node1)
	ring.AddNode(node2)
	ring.AddNode(node3)

	c1, c2, c3 := new(MockStorageNodeClient), new(MockStorageNodeClient), new(MockStorageNodeClient)
	clients := map[string]pb.StorageNodeClient{node1: c1, node2: c2, node3: c3}
	coord := NewCoordinator(context.Background(), ring, clients, 3)
	defer coord.Stop()

	c1.On("Delete", mock.Anything, mock.Anything).Return(&pb.DeleteResponse{Success: true}, nil)
	c2.On("Delete", mock.Anything, mock.Anything).Return(&pb.DeleteResponse{Success: true}, nil)
	c3.On("Delete", mock.Anything, mock.Anything).Return(&pb.DeleteResponse{Success: true}, nil)

	err := coord.Delete(context.Background(), "b", "k")
	require.NoError(t, err)
}

func TestCoordinatorAssemble(t *testing.T) {
	ring := NewConsistentHashRing(10)
	ring.AddNode(node1)
	ring.AddNode(node2)
	ring.AddNode(node3)

	c1, c2, c3 := new(MockStorageNodeClient), new(MockStorageNodeClient), new(MockStorageNodeClient)
	clients := map[string]pb.StorageNodeClient{node1: c1, node2: c2, node3: c3}
	coord := NewCoordinator(context.Background(), ring, clients, 3)
	defer coord.Stop()

	c1.On("Assemble", mock.Anything, mock.Anything).Return(&pb.AssembleResponse{Size: 100}, nil)
	c2.On("Assemble", mock.Anything, mock.Anything).Return(&pb.AssembleResponse{Size: 100}, nil)
	c3.On("Assemble", mock.Anything, mock.Anything).Return(&pb.AssembleResponse{Size: 100}, nil)

	size, err := coord.Assemble(context.Background(), "b", "k", []string{"p1", "p2"})
	require.NoError(t, err)
	assert.Equal(t, int64(100), size)
}

func TestCoordinatorGetClusterStatus(t *testing.T) {
	ring := NewConsistentHashRing(10)
	clients := make(map[string]pb.StorageNodeClient)
	coord := NewCoordinator(context.Background(), ring, clients, 3)
	defer coord.Stop()

	status, err := coord.GetClusterStatus(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, status)
}

func TestCoordinatorWriteStreamFailureMidChunk(t *testing.T) {
	ring := NewConsistentHashRing(10)
	ring.AddNode(node1)
	ring.AddNode(node2)
	ring.AddNode(node3)

	// Node A (node1): succeeds on all sends
	sm1 := new(MockStoreClient)
	sm1.On("Send", mock.Anything).Return(nil).Once() // metadata
	sm1.On("Send", mock.Anything).Return(nil).Once() // chunk
	sm1.On("CloseAndRecv").Return(&pb.StoreResponse{Success: true}, nil)

	// Node B (node2): fails on first chunk Send (after metadata Send succeeds)
	sm2 := new(MockStoreClient)
	sm2.On("Send", mock.Anything).Return(nil).Once() // metadata succeeds
	sm2.On("Send", mock.Anything).Return(errors.New("mid-stream failure")).Once()
	sm2.On("CloseAndRecv").Return(&pb.StoreResponse{Success: false, Error: "stream error"}, nil)

	// Node C (node3): succeeds on all sends
	sm3 := new(MockStoreClient)
	sm3.On("Send", mock.Anything).Return(nil).Maybe()
	sm3.On("CloseAndRecv").Return(&pb.StoreResponse{Success: true}, nil)

	c1, c2, c3 := new(MockStorageNodeClient), new(MockStorageNodeClient), new(MockStorageNodeClient)
	c1.On("Store", mock.Anything).Return(sm1, nil)
	c2.On("Store", mock.Anything).Return(sm2, nil)
	c3.On("Store", mock.Anything).Return(sm3, nil)

	clients := map[string]pb.StorageNodeClient{node1: c1, node2: c2, node3: c3}
	coord := NewCoordinator(context.Background(), ring, clients, 3)
	defer coord.Stop()

	// One chunk of data "hello" — 3 nodes, writeQuorum=2 (3/2+1), B fails mid-stream
	n, err := coord.Write(context.Background(), "b", "k", bytes.NewReader([]byte("hello")))
	require.NoError(t, err)
	assert.Equal(t, int64(5), n)

	// B's stream had 1 successful Send (metadata) then 1 failing Send (chunk) — verified by mock
	sm2.AssertNumberOfCalls(t, "Send", 2)

	// A and C should have received the chunk (at least 2 Sends each: metadata + chunk)
	sm1.AssertNumberOfCalls(t, "Send", 2)
	sm3.AssertNumberOfCalls(t, "Send", 2)

	// A and C must have called CloseAndRecv (stream stayed live)
	sm1.AssertCalled(t, "CloseAndRecv")
	sm3.AssertCalled(t, "CloseAndRecv")

	// B's stream should also have called CloseAndRecv (to clean up the failed stream)
	sm2.AssertCalled(t, "CloseAndRecv")
}

func TestCoordinatorWriteRepair(t *testing.T) {
	ring := NewConsistentHashRing(10)
	ring.AddNode(node1)
	ring.AddNode(node2)
	ring.AddNode(node3)

	ts := time.Now().UnixNano()

	// Node1 and Node3 succeed; Node2 fails mid-stream
	sm1 := new(MockStoreClient)
	sm1.On("Send", mock.Anything).Return(nil).Once() // metadata
	sm1.On("Send", mock.Anything).Return(nil).Once() // chunk
	sm1.On("CloseAndRecv").Return(&pb.StoreResponse{Success: true}, nil)

	sm3 := new(MockStoreClient)
	sm3.On("Send", mock.Anything).Return(nil).Maybe()
	sm3.On("CloseAndRecv").Return(&pb.StoreResponse{Success: true}, nil)

	// Node2 fails on chunk send (not at CloseAndRecv — that would count as quorum failure)
	sm2Write := new(MockStoreClient)
	sm2Write.On("Send", mock.Anything).Return(nil).Once() // metadata succeeds
	sm2Write.On("Send", mock.Anything).Return(errors.New("mid-stream failure")).Once()
	// CloseAndRecv is still called on the failed stream for cleanup
	sm2Write.On("CloseAndRecv").Return(&pb.StoreResponse{Success: false, Error: "stream error"}, nil)

	c1, c2, c3 := new(MockStorageNodeClient), new(MockStorageNodeClient), new(MockStorageNodeClient)
	c1.On("Store", mock.Anything).Return(sm1, nil)
	c3.On("Store", mock.Anything).Return(sm3, nil)

	// c2.Store: first call = initial write (sm2Write), second call = repair (sm2Repair)
	sm2Repair := new(MockStoreClient)
	sm2Repair.On("Send", mock.Anything).Return(nil).Maybe()
	sm2Repair.On("CloseAndRecv").Return(&pb.StoreResponse{Success: true}, nil)
	c2.On("Store", mock.Anything).Return(sm2Write, nil).Once()
	c2.On("Store", mock.Anything).Return(sm2Repair, nil).Once()

	// Write repair: Node1 serves Retrieve, Node2 receives repair Store
	rm1 := &MockRetrieveClient{resps: []*pb.RetrieveResponse{
		{Payload: &pb.RetrieveResponse_Metadata{Metadata: &pb.RetrieveMetadata{Found: true, Timestamp: ts}}},
		{Payload: &pb.RetrieveResponse_ChunkData{ChunkData: []byte("hello")}},
	}}
	c1.On("Retrieve", mock.Anything, mock.Anything).Return(rm1, nil).Once()

	clients := map[string]pb.StorageNodeClient{node1: c1, node2: c2, node3: c3}
	coord := NewCoordinator(context.Background(), ring, clients, 3)
	defer coord.Stop()

	// Write succeeds with quorum (node1 + node3 = 2, node2 pruned mid-stream)
	n, err := coord.Write(context.Background(), "b", "k", bytes.NewReader([]byte("hello")))
	require.NoError(t, err)
	assert.Equal(t, int64(5), n)

	// Wait for async write repair
	time.Sleep(200 * time.Millisecond)

	// Verify write repair: Node1 was used as source, Node2 was repaired
	c1.AssertCalled(t, "Retrieve", mock.Anything, mock.Anything)
	sm2Repair.AssertCalled(t, "CloseAndRecv")
}

func TestCoordinatorWriteRepair_SourceNodeDown(t *testing.T) {
	ring := NewConsistentHashRing(10)
	ring.AddNode(node1)
	ring.AddNode(node2)
	ring.AddNode(node3)

	// Node1 and Node3 succeed; Node2 fails mid-stream
	sm1 := new(MockStoreClient)
	sm1.On("Send", mock.Anything).Return(nil).Once()
	sm1.On("Send", mock.Anything).Return(nil).Once()
	sm1.On("CloseAndRecv").Return(&pb.StoreResponse{Success: true}, nil)

	sm3 := new(MockStoreClient)
	sm3.On("Send", mock.Anything).Return(nil).Maybe()
	sm3.On("CloseAndRecv").Return(&pb.StoreResponse{Success: true}, nil)

	sm2Write := new(MockStoreClient)
	sm2Write.On("Send", mock.Anything).Return(nil).Once()
	sm2Write.On("Send", mock.Anything).Return(errors.New("mid-stream failure")).Once()
	sm2Write.On("CloseAndRecv").Return(&pb.StoreResponse{Success: false, Error: "stream error"}, nil)

	c1, c2, c3 := new(MockStorageNodeClient), new(MockStorageNodeClient), new(MockStorageNodeClient)
	c1.On("Store", mock.Anything).Return(sm1, nil)
	c3.On("Store", mock.Anything).Return(sm3, nil)

	// c2.Store returns different mocks for write vs repair
	sm2Repair := new(MockStoreClient)
	sm2Repair.On("Send", mock.Anything).Return(nil).Maybe()
	sm2Repair.On("CloseAndRecv").Return(&pb.StoreResponse{Success: true}, nil)
	c2.On("Store", mock.Anything).Return(sm2Write, nil).Once()
	c2.On("Store", mock.Anything).Return(sm2Repair, nil).Once()

	// Source node (Node1) Retrieve fails during repair
	c1.On("Retrieve", mock.Anything, mock.Anything).Return(nil, errors.New("source node down")).Once()

	clients := map[string]pb.StorageNodeClient{node1: c1, node2: c2, node3: c3}
	coord := NewCoordinator(context.Background(), ring, clients, 3)
	defer coord.Stop()

	// Write succeeds with quorum
	n, err := coord.Write(context.Background(), "b", "k", bytes.NewReader([]byte("hello")))
	require.NoError(t, err)
	assert.Equal(t, int64(5), n)

	// Wait for async write repair
	time.Sleep(200 * time.Millisecond)

	// Source was called but failed, so repair should not complete
	c1.AssertCalled(t, "Retrieve", mock.Anything, mock.Anything)
	// sm2Repair should NOT be called since source failed
	sm2Repair.AssertNotCalled(t, "CloseAndRecv")
}

func TestCoordinatorWriteRepair_AllRepairNodesDown(t *testing.T) {
	ring := NewConsistentHashRing(10)
	ring.AddNode(node1)
	ring.AddNode(node2)
	ring.AddNode(node3)

	// Node1 and Node3 succeed; Node2 fails mid-stream
	sm1 := new(MockStoreClient)
	sm1.On("Send", mock.Anything).Return(nil).Once()
	sm1.On("Send", mock.Anything).Return(nil).Once()
	sm1.On("CloseAndRecv").Return(&pb.StoreResponse{Success: true}, nil)

	sm3 := new(MockStoreClient)
	sm3.On("Send", mock.Anything).Return(nil).Maybe()
	sm3.On("CloseAndRecv").Return(&pb.StoreResponse{Success: true}, nil)

	sm2Write := new(MockStoreClient)
	sm2Write.On("Send", mock.Anything).Return(nil).Once()
	sm2Write.On("Send", mock.Anything).Return(errors.New("mid-stream failure")).Once()
	sm2Write.On("CloseAndRecv").Return(&pb.StoreResponse{Success: false, Error: "stream error"}, nil)

	c1, c2, c3 := new(MockStorageNodeClient), new(MockStorageNodeClient), new(MockStorageNodeClient)
	c1.On("Store", mock.Anything).Return(sm1, nil)
	c3.On("Store", mock.Anything).Return(sm3, nil)

	// c2.Store first call succeeds for write, second call fails for repair
	c2.On("Store", mock.Anything).Return(sm2Write, nil).Once()
	c2.On("Store", mock.Anything).Return(nil, errors.New("repair node down")).Once()

	// Node1 serves as source for repair
	rm1 := &MockRetrieveClient{resps: []*pb.RetrieveResponse{
		{Payload: &pb.RetrieveResponse_Metadata{Metadata: &pb.RetrieveMetadata{Found: true, Timestamp: time.Now().UnixNano()}}},
		{Payload: &pb.RetrieveResponse_ChunkData{ChunkData: []byte("hello")}},
	}}
	c1.On("Retrieve", mock.Anything, mock.Anything).Return(rm1, nil).Once()

	clients := map[string]pb.StorageNodeClient{node1: c1, node2: c2, node3: c3}
	coord := NewCoordinator(context.Background(), ring, clients, 3)
	defer coord.Stop()

	// Write succeeds with quorum
	n, err := coord.Write(context.Background(), "b", "k", bytes.NewReader([]byte("hello")))
	require.NoError(t, err)
	assert.Equal(t, int64(5), n)

	// Wait for async write repair
	time.Sleep(200 * time.Millisecond)

	// Source was called but repair node was down - repair silently skipped
	c1.AssertCalled(t, "Retrieve", mock.Anything, mock.Anything)
	// No repair Store call succeeded
}

func TestCoordinatorWriteRepair_PartialRepairFailure(t *testing.T) {
	ring := NewConsistentHashRing(10)
	ring.AddNode(node1)
	ring.AddNode(node2)
	ring.AddNode(node3)

	ts := time.Now().UnixNano()

	// Node1 and Node3 succeed; Node2 fails mid-stream
	sm1 := new(MockStoreClient)
	sm1.On("Send", mock.Anything).Return(nil).Once()
	sm1.On("Send", mock.Anything).Return(nil).Once()
	sm1.On("CloseAndRecv").Return(&pb.StoreResponse{Success: true}, nil)

	sm3 := new(MockStoreClient)
	sm3.On("Send", mock.Anything).Return(nil).Maybe()
	sm3.On("CloseAndRecv").Return(&pb.StoreResponse{Success: true}, nil)

	sm2Write := new(MockStoreClient)
	sm2Write.On("Send", mock.Anything).Return(nil).Once()
	sm2Write.On("Send", mock.Anything).Return(errors.New("mid-stream failure")).Once()
	sm2Write.On("CloseAndRecv").Return(&pb.StoreResponse{Success: false, Error: "stream error"}, nil)

	c1, c2, c3 := new(MockStorageNodeClient), new(MockStorageNodeClient), new(MockStorageNodeClient)
	c1.On("Store", mock.Anything).Return(sm1, nil)
	c3.On("Store", mock.Anything).Return(sm3, nil)

	// c2.Store: write succeeds, repair sends but CloseAndRecv fails
	sm2Repair := new(MockStoreClient)
	sm2Repair.On("Send", mock.Anything).Return(nil).Maybe()
	sm2Repair.On("CloseAndRecv").Return(&pb.StoreResponse{Success: false, Error: "repair failed"}, nil)
	c2.On("Store", mock.Anything).Return(sm2Write, nil).Once()
	c2.On("Store", mock.Anything).Return(sm2Repair, nil).Once()

	// Node1 serves as source for repair
	rm1 := &MockRetrieveClient{resps: []*pb.RetrieveResponse{
		{Payload: &pb.RetrieveResponse_Metadata{Metadata: &pb.RetrieveMetadata{Found: true, Timestamp: ts}}},
		{Payload: &pb.RetrieveResponse_ChunkData{ChunkData: []byte("hello")}},
	}}
	c1.On("Retrieve", mock.Anything, mock.Anything).Return(rm1, nil).Once()

	clients := map[string]pb.StorageNodeClient{node1: c1, node2: c2, node3: c3}
	coord := NewCoordinator(context.Background(), ring, clients, 3)
	defer coord.Stop()

	// Write succeeds with quorum
	n, err := coord.Write(context.Background(), "b", "k", bytes.NewReader([]byte("hello")))
	require.NoError(t, err)
	assert.Equal(t, int64(5), n)

	// Wait for async write repair
	time.Sleep(200 * time.Millisecond)

	// Repair was attempted but CloseAndRecv returned failure
	c1.AssertCalled(t, "Retrieve", mock.Anything, mock.Anything)
	sm2Repair.AssertCalled(t, "CloseAndRecv")
}

func TestCoordinatorRepairStreamFailureContinues(t *testing.T) {
	ring := NewConsistentHashRing(10)
	ring.AddNode(node1)
	ring.AddNode(node2)
	ring.AddNode(node3)

	// Setup read: node1 is the winner (newest), node2 and node3 are stale
	tsNew := time.Now().UnixNano()
	tsOld := tsNew - 1000

	rm1 := &MockRetrieveClient{resps: []*pb.RetrieveResponse{
		{Payload: &pb.RetrieveResponse_Metadata{Metadata: &pb.RetrieveMetadata{Found: true, Timestamp: tsNew}}},
		{Payload: &pb.RetrieveResponse_ChunkData{ChunkData: []byte("newdata")}},
	}}
	c1 := new(MockStorageNodeClient)
	c1.On("Retrieve", mock.Anything, mock.Anything).Return(rm1, nil)

	// node2: stale, repair will fail on first chunk
	rm2 := &MockRetrieveClient{resps: []*pb.RetrieveResponse{
		{Payload: &pb.RetrieveResponse_Metadata{Metadata: &pb.RetrieveMetadata{Found: true, Timestamp: tsOld}}},
		{Payload: &pb.RetrieveResponse_ChunkData{ChunkData: []byte("olddata")}},
	}}
	c2 := new(MockStorageNodeClient)
	c2.On("Retrieve", mock.Anything, mock.Anything).Return(rm2, nil)

	// node3: stale, repair should succeed
	rm3 := &MockRetrieveClient{resps: []*pb.RetrieveResponse{
		{Payload: &pb.RetrieveResponse_Metadata{Metadata: &pb.RetrieveMetadata{Found: true, Timestamp: tsOld}}},
		{Payload: &pb.RetrieveResponse_ChunkData{ChunkData: []byte("olddata")}},
	}}
	c3 := new(MockStorageNodeClient)
	c3.On("Retrieve", mock.Anything, mock.Anything).Return(rm3, nil)

	// Repair streams: node2 fails on first chunk, node3 succeeds
	smRepair2 := new(MockStoreClient)
	smRepair2.On("Send", mock.Anything).Return(nil).Once() // metadata
	smRepair2.On("Send", mock.Anything).Return(errors.New("repair node2 failure")).Once()
	smRepair2.On("CloseAndRecv").Return(&pb.StoreResponse{Success: false, Error: "repair failed"}, nil)
	c2.On("Store", mock.Anything).Return(smRepair2, nil)

	smRepair3 := new(MockStoreClient)
	smRepair3.On("Send", mock.Anything).Return(nil).Maybe()
	smRepair3.On("CloseAndRecv").Return(&pb.StoreResponse{Success: true}, nil)
	c3.On("Store", mock.Anything).Return(smRepair3, nil)

	clients := map[string]pb.StorageNodeClient{node1: c1, node2: c2, node3: c3}
	coord := NewCoordinator(context.Background(), ring, clients, 3)
	defer coord.Stop()

	r, err := coord.Read(context.Background(), "b", "k")
	require.NoError(t, err)

	data, err := io.ReadAll(r)
	require.NoError(t, err)
	assert.Equal(t, "newdata", string(data))
	require.NoError(t, r.Close())

	// Wait for async repair goroutines
	time.Sleep(200 * time.Millisecond)

	// node2: metadata succeeded, chunk failed, CloseAndRecv called to clean up
	smRepair2.AssertNumberOfCalls(t, "Send", 2)
	smRepair2.AssertCalled(t, "CloseAndRecv")

	// node3: both metadata and chunk sent, CloseAndRecv called
	smRepair3.AssertNumberOfCalls(t, "Send", 2)
	smRepair3.AssertCalled(t, "CloseAndRecv")
}

func TestCoordinatorReadQuorum(t *testing.T) {
	ts := time.Now().UnixNano()

	tests := []struct {
		name          string
		replicaCount  int
		setupMocks    func(c1, c2, c3 *MockStorageNodeClient) []*MockStoreClient
		expectedError string
	}{
		{
			name:         "Failure_OnlyOneNodeResponds",
			replicaCount: 3,
			setupMocks: func(c1, c2, c3 *MockStorageNodeClient) []*MockStoreClient {
				// Node 1: has data (counted in foundCount)
				rm1 := &MockRetrieveClient{resps: []*pb.RetrieveResponse{
					{Payload: &pb.RetrieveResponse_Metadata{Metadata: &pb.RetrieveMetadata{Found: true, Timestamp: ts}}},
					{Payload: &pb.RetrieveResponse_ChunkData{ChunkData: []byte("data")}},
				}}
				c1.On("Retrieve", mock.Anything, mock.Anything).Return(rm1, nil)
				// Node 2: Retrieve error
				c2.On("Retrieve", mock.Anything, mock.Anything).Return(nil, errors.New("node down"))
				// Node 3: Retrieve error
				c3.On("Retrieve", mock.Anything, mock.Anything).Return(nil, errors.New("node down"))
				return nil
			},
			expectedError: "read quorum failed (1/2)",
		},
		{
			name:         "Success_TwoOfThreeRespond",
			replicaCount: 3,
			setupMocks: func(c1, c2, c3 *MockStorageNodeClient) []*MockStoreClient {
				rm1 := &MockRetrieveClient{resps: []*pb.RetrieveResponse{
					{Payload: &pb.RetrieveResponse_Metadata{Metadata: &pb.RetrieveMetadata{Found: true, Timestamp: ts}}},
					{Payload: &pb.RetrieveResponse_ChunkData{ChunkData: []byte("data")}},
				}}
				c1.On("Retrieve", mock.Anything, mock.Anything).Return(rm1, nil)
				rm2 := &MockRetrieveClient{resps: []*pb.RetrieveResponse{
					{Payload: &pb.RetrieveResponse_Metadata{Metadata: &pb.RetrieveMetadata{Found: true, Timestamp: ts}}},
					{Payload: &pb.RetrieveResponse_ChunkData{ChunkData: []byte("data")}},
				}}
				c2.On("Retrieve", mock.Anything, mock.Anything).Return(rm2, nil)
				c3.On("Retrieve", mock.Anything, mock.Anything).Return(nil, errors.New("node down"))
				return nil
			},
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ring := NewConsistentHashRing(10)
			ring.AddNode(node1)
			ring.AddNode(node2)
			ring.AddNode(node3)

			c1, c2, c3 := new(MockStorageNodeClient), new(MockStorageNodeClient), new(MockStorageNodeClient)
			tt.setupMocks(c1, c2, c3)

			clients := map[string]pb.StorageNodeClient{node1: c1, node2: c2, node3: c3}
			coord := NewCoordinator(context.Background(), ring, clients, tt.replicaCount)
			defer coord.Stop()

			r, err := coord.Read(context.Background(), "b", "k")
			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)
				data, readErr := io.ReadAll(r)
				require.NoError(t, readErr)
				assert.Equal(t, "data", string(data))
				require.NoError(t, r.Close())
			}
		})
	}
}
