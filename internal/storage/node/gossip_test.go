package node

import (
	"bytes"
	"log/slog"
	"math"
	"testing"
	"time"

	pb "github.com/poyrazk/thecloud/internal/storage/protocol"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	testNode1Addr = "localhost:8080"
	testNode2Addr = "localhost:8081"
	testNode3Addr = "localhost:8082"
)

func TestGossipProtocolAddPeer(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	g := NewGossipProtocol("node1", testNode1Addr, nil, logger)

	g.AddPeer("node2", testNode2Addr)

	g.mu.RLock()
	defer g.mu.RUnlock()
	assert.Contains(t, g.members, "node2")
	assert.Equal(t, testNode2Addr, g.members["node2"].Address)
}

func TestGossipProtocolOnGossip(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	g := NewGossipProtocol("node1", testNode1Addr, nil, logger)

	// Update coming from node2 about node3
	msg := &pb.GossipMessage{
		SenderId: "node2",
		Members: map[string]*pb.MemberState{
			"node3": {
				Addr:      testNode3Addr,
				Status:    "alive",
				Heartbeat: 10,
			},
		},
	}

	g.OnGossip(msg)

	g.mu.RLock()
	node3, exists := g.members["node3"]
	g.mu.RUnlock()

	assert.True(t, exists)
	assert.Equal(t, testNode3Addr, node3.Address)
	assert.Equal(t, uint64(10), node3.Heartbeat)

	// Newer heartbeat
	msg2 := &pb.GossipMessage{
		Members: map[string]*pb.MemberState{
			"node3": {
				Addr:      testNode3Addr,
				Status:    "alive",
				Heartbeat: 15,
			},
		},
	}
	g.OnGossip(msg2)

	g.mu.RLock()
	assert.Equal(t, uint64(15), g.members["node3"].Heartbeat)
	g.mu.RUnlock()
}

func TestGossipProtocolOnGossipIgnoresOlderHeartbeat(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	g := NewGossipProtocol("node1", testNode1Addr, nil, logger)

	g.members["node2"] = &MemberState{
		Address:   testNode2Addr,
		Status:    "alive",
		LastSeen:  time.Now(),
		Heartbeat: 10,
	}

	msg := &pb.GossipMessage{
		Members: map[string]*pb.MemberState{
			"node2": {
				Addr:      testNode2Addr,
				Status:    "alive",
				Heartbeat: 5,
			},
		},
	}

	g.OnGossip(msg)

	g.mu.RLock()
	assert.Equal(t, uint64(10), g.members["node2"].Heartbeat)
	g.mu.RUnlock()
}

func TestGossipProtocolDetectFailures(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	g := NewGossipProtocol("node1", testNode1Addr, nil, logger)

	// Add a node that was seen long ago
	g.members["node2"] = &MemberState{
		Address:  testNode2Addr,
		Status:   "alive",
		LastSeen: time.Now().Add(-10 * time.Second), // Timeout is 5s
	}

	g.detectFailures()

	g.mu.RLock()
	assert.Equal(t, "suspect", g.members["node2"].Status)
	g.mu.RUnlock()

	// Advance time more to make it dead ( > 3*timeout = 15s)
	g.members["node2"].LastSeen = time.Now().Add(-20 * time.Second)
	g.detectFailures()

	g.mu.RLock()
	assert.Equal(t, "dead", g.members["node2"].Status)
	assert.False(t, g.members["node2"].DeadAt.IsZero(), "DeadAt should be set on transition")
	g.mu.RUnlock()
}

// fakeConn lets us seed g.peers without a real gRPC server, just to verify the
// connection is closed when the peer transitions to dead.
func newFakeGRPCConn(t *testing.T) *grpc.ClientConn {
	t.Helper()
	conn, err := grpc.NewClient("passthrough:///fake", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	return conn
}

func TestGossipProtocolDetectFailuresClosesPeerConnOnDead(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	g := NewGossipProtocol("node1", testNode1Addr, nil, logger)

	conn := newFakeGRPCConn(t)
	g.members["node2"] = &MemberState{
		Address:  testNode2Addr,
		Status:   "suspect",
		LastSeen: time.Now().Add(-20 * time.Second),
	}
	g.peers["node2"] = &peerClient{conn: conn, client: pb.NewStorageNodeClient(conn)}

	g.detectFailures()

	g.mu.RLock()
	_, peerStillThere := g.peers["node2"]
	g.mu.RUnlock()
	assert.False(t, peerStillThere, "peer connection should be removed when node is flagged dead")
	assert.Equal(t, "SHUTDOWN", conn.GetState().String(), "underlying gRPC conn should be closed")
}

func TestGossipProtocolDetectFailuresPurgesDeadMembers(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	g := NewGossipProtocol("node1", testNode1Addr, nil, logger)

	g.members["node2"] = &MemberState{
		Address:  testNode2Addr,
		Status:   "dead",
		LastSeen: time.Now().Add(-2 * time.Hour),
		DeadAt:   time.Now().Add(-2 * deadPurgeAfter),
	}

	g.detectFailures()

	g.mu.RLock()
	_, exists := g.members["node2"]
	g.mu.RUnlock()
	assert.False(t, exists, "dead member should be purged after grace period")
}

func TestGossipProtocolStopClosesAllPeers(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	g := NewGossipProtocol("node1", testNode1Addr, nil, logger)

	conn1 := newFakeGRPCConn(t)
	conn2 := newFakeGRPCConn(t)
	g.peers["node2"] = &peerClient{conn: conn1, client: pb.NewStorageNodeClient(conn1)}
	g.peers["node3"] = &peerClient{conn: conn2, client: pb.NewStorageNodeClient(conn2)}

	g.Stop()

	assert.Empty(t, g.peers, "peers map should be cleared on Stop")
	assert.Empty(t, g.members, "members map should be cleared on Stop")
	assert.Equal(t, "SHUTDOWN", conn1.GetState().String())
	assert.Equal(t, "SHUTDOWN", conn2.GetState().String())
}

func TestGossipProtocolStopIsIdempotent(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	g := NewGossipProtocol("node1", testNode1Addr, nil, logger)

	g.Stop()
	// Second call must not panic on close-of-closed-channel.
	g.Stop()
}

func TestGossipProtocolOnGossipIgnoresDeadResurrection(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	g := NewGossipProtocol("node1", testNode1Addr, nil, logger)

	// Locally we already consider node2 dead.
	g.members["node2"] = &MemberState{
		Address:   testNode2Addr,
		Status:    "dead",
		LastSeen:  time.Now().Add(-1 * time.Hour),
		Heartbeat: 5,
		DeadAt:    time.Now(),
	}

	// A peer reports node2 as alive with a higher heartbeat.
	msg := &pb.GossipMessage{
		Members: map[string]*pb.MemberState{
			"node2": {Addr: testNode2Addr, Status: "alive", Heartbeat: 99},
		},
	}
	g.OnGossip(msg)

	g.mu.RLock()
	defer g.mu.RUnlock()
	assert.Equal(t, "dead", g.members["node2"].Status, "dead status should be sticky locally")
	assert.Equal(t, uint64(5), g.members["node2"].Heartbeat)
}

func TestGossipProtocolOnGossipDoesNotDiscoverDeadNode(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	g := NewGossipProtocol("node1", testNode1Addr, nil, logger)

	msg := &pb.GossipMessage{
		Members: map[string]*pb.MemberState{
			"node2": {Addr: testNode2Addr, Status: "dead", Heartbeat: 1},
		},
	}
	g.OnGossip(msg)

	g.mu.RLock()
	_, exists := g.members["node2"]
	g.mu.RUnlock()
	assert.False(t, exists, "should not add a member that is reported dead")
}

func TestGossipProtocolHeartbeatOverflowResets(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	g := NewGossipProtocol("node1", testNode1Addr, nil, logger)

	g.mu.Lock()
	g.members["node1"].Heartbeat = math.MaxUint64
	g.mu.Unlock()

	// gossip() should detect overflow and reset to 0
	g.gossip()

	g.mu.RLock()
	hb := g.members["node1"].Heartbeat
	g.mu.RUnlock()
	assert.Equal(t, uint64(0), hb, "heartbeat should reset to 0 on overflow")
}

func TestGossipProtocolOnGossipWraparoundTiebreaker(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	g := NewGossipProtocol("node1", testNode1Addr, nil, logger)

	now := time.Now()
	g.mu.Lock()
	g.members["node2"] = &MemberState{
		Address:   testNode2Addr,
		Status:    "alive",
		LastSeen:  now,
		Heartbeat: 100, // high heartbeat before wrap
	}
	g.mu.Unlock()

	// Remote node rebooted — its heartbeat wrapped to 1, message timestamp is now
	olderMsg := &pb.GossipMessage{
		SenderId:  "node2",
		Timestamp: now.Add(-1 * time.Second).Unix(), // older than our LastSeen
		Members: map[string]*pb.MemberState{
			"node2": {Addr: testNode2Addr, Status: "alive", Heartbeat: 1},
		},
	}
	g.OnGossip(olderMsg)

	g.mu.RLock()
	hb := g.members["node2"].Heartbeat
	g.mu.RUnlock()
	assert.Equal(t, uint64(100), hb, "older message with low heartbeat should not override")

	// Newer message with wrapped heartbeat should win
	newerMsg := &pb.GossipMessage{
		SenderId:  "node2",
		Timestamp: now.Add(1 * time.Second).Unix(), // newer than our LastSeen
		Members: map[string]*pb.MemberState{
			"node2": {Addr: testNode2Addr, Status: "alive", Heartbeat: 1},
		},
	}
	g.OnGossip(newerMsg)

	g.mu.RLock()
	hb = g.members["node2"].Heartbeat
	ls := g.members["node2"].LastSeen
	g.mu.RUnlock()
	assert.Equal(t, uint64(1), hb, "newer message with wrapped heartbeat should override")
	assert.True(t, ls.After(now), "LastSeen should update to newer timestamp")
}

func TestGossipProtocolDetectFailuresCleansOrphanedPeer(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	g := NewGossipProtocol("node1", testNode1Addr, nil, logger)

	// Seed an orphaned peer — in peers but not in members
	// (simulates a peer that was added via AddPeer but whose member entry
	// was already purged before the peer was ever connected)
	conn := newFakeGRPCConn(t)
	g.peers["orphan-node"] = &peerClient{conn: conn, client: pb.NewStorageNodeClient(conn)}

	g.detectFailures()

	g.mu.RLock()
	_, peerStillThere := g.peers["orphan-node"]
	g.mu.RUnlock()
	assert.False(t, peerStillThere, "orphaned peer should be removed by detectFailures")
	assert.Equal(t, "SHUTDOWN", conn.GetState().String(), "connection should be closed")
}
