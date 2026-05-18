// Package node implements storage node services.
package node

import (
	"context"
	"crypto/rand"
	"log/slog"
	"math"
	"math/big"
	"sync"
	"time"

	pb "github.com/poyrazk/thecloud/internal/storage/protocol"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Gossip protocol tuning constants. These are deliberately exported so that
// tests, embedders, and ops tooling can override them in lock-step without
// editing source. The defaults are conservative for low-latency LAN clusters
// and may need to be raised for WAN/high-latency deployments to avoid false
// positives.
const (
	// deadPurgeAfter is how long a member stays in the members map after being
	// flagged dead, giving the Coordinator time to observe the status before
	// the entry is reaped.
	deadPurgeAfter = 60 * time.Second
	// defaultFailCheckInterval is how often detectFailures runs.
	defaultFailCheckInterval = 2 * time.Second
	// defaultGossipRPCTimeout bounds a single Gossip RPC. Networks with
	// >2s p99 latency should override this.
	defaultGossipRPCTimeout = 2 * time.Second
	// defaultFailureDetectionTimeout is the LastSeen age at which a peer is
	// promoted from "alive" → "suspect". Dead promotion is 3× this.
	defaultFailureDetectionTimeout = 5 * time.Second
)

// MemberState tracks a peer's status in the gossip ring.
type MemberState struct {
	Address   string
	Status    string
	LastSeen  time.Time
	Heartbeat uint64
	// DeadAt is the time the member transitioned to "dead". Zero otherwise.
	DeadAt time.Time
}

// peerClient bundles a gRPC connection with its generated client so the
// connection can be closed when the peer is no longer needed.
type peerClient struct {
	conn   *grpc.ClientConn
	client pb.StorageNodeClient
}

// GossipProtocol manages membership and health gossip between nodes.
type GossipProtocol struct {
	nodeID   string
	address  string
	members  map[string]*MemberState
	mu       sync.RWMutex
	stopCh   chan struct{}
	stopOnce sync.Once
	logger   *slog.Logger
	dialOpts []grpc.DialOption
	peers    map[string]*peerClient

	// started tracks whether Start() has been called. Once true, tunable
	// setters no-op to prevent confusing race conditions where a goroutine
	// reads the new value while the old value is already live in tickers.
	started                    bool
	failureDetectionTimeout    time.Duration
	gossipRPCTimeout          time.Duration
	failCheckInterval          time.Duration
}

// SetFailureDetectionTimeout overrides the alive→suspect promotion threshold.
// Must be called before Start; changes after Start are ignored.
func (g *GossipProtocol) SetFailureDetectionTimeout(d time.Duration) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.started {
		return
	}
	g.failureDetectionTimeout = d
}

// SetGossipRPCTimeout overrides the per-RPC Gossip timeout.
// Must be called before Start; changes after Start are ignored.
func (g *GossipProtocol) SetGossipRPCTimeout(d time.Duration) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.started {
		return
	}
	g.gossipRPCTimeout = d
}

// SetFailCheckInterval overrides how often detectFailures runs.
// Must be called before Start; changes after Start are ignored.
func (g *GossipProtocol) SetFailCheckInterval(d time.Duration) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.started {
		return
	}
	g.failCheckInterval = d
}

// NewGossipProtocol constructs a GossipProtocol for a node.
func NewGossipProtocol(nodeID, address string, logger *slog.Logger) *GossipProtocol {
	g := &GossipProtocol{
		nodeID:                  nodeID,
		address:                 address,
		members:                 make(map[string]*MemberState),
		stopCh:                  make(chan struct{}),
		logger:                  logger,
		peers:                   make(map[string]*peerClient),
		dialOpts:                []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())},
		failureDetectionTimeout: defaultFailureDetectionTimeout,
		gossipRPCTimeout:        defaultGossipRPCTimeout,
		failCheckInterval:       defaultFailCheckInterval,
	}
	// Add self
	g.members[nodeID] = &MemberState{
		Address:   address,
		Status:    "alive",
		LastSeen:  time.Now(),
		Heartbeat: 0,
	}
	return g
}

func (g *GossipProtocol) AddPeer(id, addr string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, ok := g.members[id]; !ok {
		g.members[id] = &MemberState{
			Address:   addr,
			Status:    "alive",
			LastSeen:  time.Now(),
			Heartbeat: 0,
		}
	}
}

func (g *GossipProtocol) Start(interval time.Duration) {
	g.mu.Lock()
	g.started = true
	g.mu.Unlock()

	ticker := time.NewTicker(interval)
	g.mu.RLock()
	failInterval := g.failCheckInterval
	g.mu.RUnlock()
	failTicker := time.NewTicker(failInterval)
	go func() {
		for {
			select {
			case <-ticker.C:
				g.gossip()
			case <-failTicker.C:
				g.detectFailures()
			case <-g.stopCh:
				ticker.Stop()
				failTicker.Stop()
				return
			}
		}
	}()
}

func (g *GossipProtocol) detectFailures() {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := time.Now()
	timeout := g.failureDetectionTimeout

	for id, m := range g.members {
		if id == g.nodeID {
			continue
		}

		switch {
		case m.Status == "alive" && now.Sub(m.LastSeen) > timeout:
			m.Status = "suspect"
			g.logger.Warn("node flagged as suspect", "id", id, "last_seen", m.LastSeen)
		case m.Status == "suspect" && now.Sub(m.LastSeen) > 3*timeout:
			m.Status = "dead"
			m.DeadAt = now
			g.logger.Error("node flagged as dead", "id", id, "last_seen", m.LastSeen)
			// Close and drop the gRPC client connection so it doesn't leak.
			// The member entry is kept until the purge below so the
			// Coordinator can observe the "dead" status via GetClusterStatus.
			g.closePeerLocked(id)
		case m.Status == "dead" && !m.DeadAt.IsZero() && now.Sub(m.DeadAt) > deadPurgeAfter:
			delete(g.members, id)
			g.closePeerLocked(id)
			g.logger.Info("purged dead member", "id", id)
		}
	}

	// Clean up orphaned peers: entries in g.peers whose corresponding
	// members no longer exist. This can happen when:
	// 1. sendGossip connects to a node and adds it to g.peers
	// 2. detectFailures purges that member from g.members before sendGossip runs
	// Without this sweep, gRPC connections would leak indefinitely.
	for id := range g.peers {
		if _, inMembers := g.members[id]; !inMembers {
			g.closePeerLocked(id)
		}
	}
}

// closePeerLocked closes and removes the peer client for id. Caller must hold
// g.mu (write lock).
func (g *GossipProtocol) closePeerLocked(id string) {
	p, ok := g.peers[id]
	if !ok {
		return
	}
	delete(g.peers, id)
	if err := p.conn.Close(); err != nil {
		g.logger.Warn("failed to close peer connection", "peer", id, "error", err)
	}
}

func (g *GossipProtocol) Stop() {
	g.stopOnce.Do(func() {
		close(g.stopCh)

		g.mu.Lock()
		defer g.mu.Unlock()
		for id, p := range g.peers {
			if err := p.conn.Close(); err != nil {
				g.logger.Warn("failed to close peer connection", "peer", id, "error", err)
			}
			delete(g.peers, id)
		}
		clear(g.members)
	})
}

func (g *GossipProtocol) gossip() {
	g.mu.Lock()
	// Increment own heartbeat
	me := g.members[g.nodeID]
	if me.Heartbeat == math.MaxUint64 {
		me.Heartbeat = 0
		g.logger.Warn("heartbeat counter overflow, reset to 0", "node_id", g.nodeID)
	} else {
		me.Heartbeat++
	}
	me.LastSeen = time.Now()

	// Prepare message
	msg := &pb.GossipMessage{
		SenderId:   g.nodeID,
		SenderAddr: g.address,
		Timestamp:  time.Now().Unix(),
		Members:    make(map[string]*pb.MemberState),
	}

	// Convert members to proto and select random peer
	var peers []string
	for id, m := range g.members {
		msg.Members[id] = &pb.MemberState{
			Addr:      m.Address,
			Status:    m.Status,
			LastSeen:  m.LastSeen.Unix(),
			Heartbeat: m.Heartbeat,
		}
		if id != g.nodeID && m.Status == "alive" {
			peers = append(peers, id)
		}
	}
	g.mu.Unlock()

	if len(peers) == 0 {
		return
	}

	// Pick random peer
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(peers))))
	var targetID string
	if err != nil {
		targetID = peers[0]
	} else {
		targetID = peers[n.Int64()]
	}
	g.sendGossip(targetID, msg)
}

func (g *GossipProtocol) sendGossip(targetID string, msg *pb.GossipMessage) {
	g.mu.RLock()
	member, memberOK := g.members[targetID]
	p, peerOK := g.peers[targetID]
	g.mu.RUnlock()

	if !memberOK {
		return
	}
	targetAddr := member.Address

	if !peerOK {
		conn, err := grpc.NewClient(targetAddr, g.dialOpts...)
		if err != nil {
			g.logger.Error("failed to connect to peer", "peer", targetID, "error", err)
			return
		}
		newPeer := &peerClient{conn: conn, client: pb.NewStorageNodeClient(conn)}

		g.mu.Lock()
		// Re-check under the write lock: another goroutine may have created
		// the peer concurrently, or the member may have been purged.
		if existing, ok := g.peers[targetID]; ok {
			g.mu.Unlock()
			_ = conn.Close()
			p = existing
		} else if _, stillMember := g.members[targetID]; !stillMember {
			g.mu.Unlock()
			_ = conn.Close()
			return
		} else {
			g.peers[targetID] = newPeer
			p = newPeer
			g.mu.Unlock()
		}
	}

	g.mu.RLock()
	rpcTimeout := g.gossipRPCTimeout
	g.mu.RUnlock()
	ctx, cancel := context.WithTimeout(context.Background(), rpcTimeout)
	defer cancel()

	if _, err := p.client.Gossip(ctx, msg); err != nil {
		g.logger.Warn("gossip failed", "target", targetID, "error", err)
		// Mark as suspect if needed (Phase 2 enhancement)
	}
}

func (g *GossipProtocol) OnGossip(msg *pb.GossipMessage) {
	g.mu.Lock()
	defer g.mu.Unlock()

	for id, remoteState := range msg.Members {
		localState, exists := g.members[id]
		if !exists {
			// Don't resurrect a node a peer is reporting as already dead —
			// otherwise a tombstone we just purged could be re-added.
			if remoteState.Status == "dead" {
				continue
			}
			g.members[id] = &MemberState{
				Address:   remoteState.Addr,
				Status:    remoteState.Status,
				LastSeen:  time.Now(),
				Heartbeat: remoteState.Heartbeat,
			}
			g.logger.Info("discovered new member", "id", id, "addr", remoteState.Addr)
			continue
		}

		// Once we've locally flagged a member as dead, ignore further updates
		// from gossip — peers will eventually converge.
		if localState.Status == "dead" {
			continue
		}

		if remoteState.Heartbeat > localState.Heartbeat {
			localState.Heartbeat = remoteState.Heartbeat
			localState.LastSeen = time.Now()
			localState.Status = remoteState.Status
		} else if remoteState.Heartbeat < localState.Heartbeat && remoteState.Heartbeat < 100 {
			// Heartbeat appears to have wrapped (very low value while local is high).
			// Use the gossip message timestamp as a freshness tiebreaker — a newer
			// timestamp means the remote heartbeat was incremented after the wrap.
			remoteTime := time.Unix(msg.Timestamp, 0)
			if remoteTime.After(localState.LastSeen) {
				localState.Heartbeat = remoteState.Heartbeat
				localState.LastSeen = remoteTime
				localState.Status = remoteState.Status
			}
		}
	}
}
