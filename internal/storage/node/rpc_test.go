package node

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	pb "github.com/poyrazk/thecloud/internal/storage/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

type mockStoreServer struct {
	grpc.ServerStream
	ctx     context.Context
	reqs    []*pb.StoreRequest
	resp    *pb.StoreResponse
	recvIdx int
}

func (m *mockStoreServer) Context() context.Context { return m.ctx }
func (m *mockStoreServer) SendAndClose(r *pb.StoreResponse) error {
	m.resp = r
	return nil
}
func (m *mockStoreServer) Recv() (*pb.StoreRequest, error) {
	if m.recvIdx >= len(m.reqs) {
		return nil, io.EOF
	}
	r := m.reqs[m.recvIdx]
	m.recvIdx++
	return r, nil
}

type mockRetrieveServer struct {
	grpc.ServerStream
	ctx   context.Context
	resps []*pb.RetrieveResponse
}

func (m *mockRetrieveServer) Context() context.Context { return m.ctx }
func (m *mockRetrieveServer) Send(r *pb.RetrieveResponse) error {
	m.resps = append(m.resps, r)
	return nil
}

func TestRPCServer(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewLocalStore(tmpDir)
	server := NewRPCServer(store, nil)

	ctx := context.Background()

	// 1. Store (Streaming)
	ts := time.Now().UnixNano()
	storeMock := &mockStoreServer{
		ctx: ctx,
		reqs: []*pb.StoreRequest{
			{Payload: &pb.StoreRequest_Metadata{Metadata: &pb.StoreMetadata{Bucket: "bucket1", Key: "key1", Timestamp: ts}}},
			{Payload: &pb.StoreRequest_ChunkData{ChunkData: []byte("value1")}},
		},
	}
	err := server.Store(storeMock)
	require.NoError(t, err)
	assert.True(t, storeMock.resp.Success)

	// 2. Retrieve (Streaming)
	retrieveMock := &mockRetrieveServer{ctx: ctx}
	err = server.Retrieve(&pb.RetrieveRequest{Bucket: "bucket1", Key: "key1"}, retrieveMock)
	require.NoError(t, err)

	found := false
	var data []byte
	for _, r := range retrieveMock.resps {
		switch p := r.Payload.(type) {
		case *pb.RetrieveResponse_Metadata:
			found = p.Metadata.Found
		case *pb.RetrieveResponse_ChunkData:
			data = append(data, p.ChunkData...)
		}
	}
	assert.True(t, found)
	assert.Equal(t, []byte("value1"), data)

	// 3. Delete
	_, err = server.Delete(ctx, &pb.DeleteRequest{
		Bucket: "bucket1",
		Key:    "key1",
	})
	require.NoError(t, err)

	// Verify Retrieve fails
	retrieveMock2 := &mockRetrieveServer{ctx: ctx}
	err = server.Retrieve(&pb.RetrieveRequest{Bucket: "bucket1", Key: "key1"}, retrieveMock2)
	require.NoError(t, err)
	assert.False(t, retrieveMock2.resps[0].GetMetadata().Found)

	// 4. Assemble
	require.NoError(t, store.Write("bucket1", "parts/1", []byte("A"), 0))
	require.NoError(t, store.Write("bucket1", "parts/2", []byte("B"), 0))

	asmResp, err := server.Assemble(ctx, &pb.AssembleRequest{
		Bucket: "bucket1",
		Key:    "final",
		Parts:  []string{"parts/1", "parts/2"},
	})
	require.NoError(t, err)
	assert.Equal(t, int64(2), asmResp.Size)
}

func TestRPCServerGetClusterStatus(t *testing.T) {
	server := NewRPCServer(nil, nil)
	resp, err := server.GetClusterStatus(context.Background(), &pb.Empty{})
	require.NoError(t, err)
	assert.Empty(t, resp.Members)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	g := NewGossipProtocol("node1", "localhost:5001", nil, logger)
	g.AddPeer("node2", "localhost:5002")

	server = NewRPCServer(nil, g)
	resp, err = server.GetClusterStatus(context.Background(), &pb.Empty{})
	require.NoError(t, err)
	assert.Len(t, resp.Members, 2)
}

func TestRPCServerStoreError(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewLocalStore(tmpDir)
	server := NewRPCServer(store, nil)

	storeMock := &mockStoreServer{
		ctx: context.Background(),
		reqs: []*pb.StoreRequest{
			{Payload: &pb.StoreRequest_Metadata{Metadata: &pb.StoreMetadata{Bucket: "bucket", Key: "../bad"}}},
			{Payload: &pb.StoreRequest_ChunkData{ChunkData: []byte("data")}},
		},
	}
	err := server.Store(storeMock)
	// In my implementation, traversal returns error to gRPC layer
	require.NoError(t, err) // SendAndClose returns nil usually, error is in response
	assert.False(t, storeMock.resp.Success)
}
