// Package coordinator manages distributed storage coordination.
package coordinator

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"math/big"
	"sync"

	"time"

	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/platform"
	pb "github.com/poyrazk/thecloud/internal/storage/protocol"
)

const (
	errNoNodesAvailable = "no storage nodes available"
	chunkSize           = 1024 * 1024 // 1MB chunks
	repairTimeout       = 30 * time.Second
	// maxObjectSize prevents memory exhaustion when writing large objects.
	maxObjectSize = 5 * 1024 * 1024 * 1024 // 5 GB
)

// Coordinator implements ports.FileStore to manage distributed storage.
type Coordinator struct {
	ring         *ConsistentHashRing
	clients      map[string]pb.StorageNodeClient
	replicaCount int
	writeQuorum  int
	readQuorum   int
	stopCh       chan struct{}
	lastStatus   *domain.StorageCluster
	mu           sync.RWMutex
}

// NewCoordinator creates a new distributed storage coordinator.
func NewCoordinator(ctx context.Context, ring *ConsistentHashRing, clients map[string]pb.StorageNodeClient, replicaCount int) *Coordinator {
	if replicaCount < 1 {
		replicaCount = 1
	}
	c := &Coordinator{
		ring:         ring,
		clients:      clients,
		replicaCount: replicaCount,
		writeQuorum:  (replicaCount / 2) + 1,
		readQuorum:   (replicaCount / 2) + 1,
		stopCh:       make(chan struct{}),
	}
	go c.startSyncLoop(ctx)
	return c
}

func (c *Coordinator) startSyncLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.SyncClusterState(ctx)
		case <-c.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (c *Coordinator) SyncClusterState(ctx context.Context) {
	// Pick random node to query
	var client pb.StorageNodeClient
	if len(c.clients) == 0 {
		return
	}

	// Convert map values to slice for random selection
	clients := make([]pb.StorageNodeClient, 0, len(c.clients))
	for _, cl := range c.clients {
		clients = append(clients, cl)
	}

	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(clients))))
	if err != nil {
		client = clients[0]
	} else {
		client = clients[n.Int64()]
	}

	if client == nil {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	resp, err := client.GetClusterStatus(ctx, &pb.Empty{})
	if err != nil {
		// Try another one next time
		return
	}

	// Update Ring based on status
	nodes := make([]domain.StorageNode, 0, len(resp.Members))
	for id, m := range resp.Members {
		if m.Status == "dead" {
			c.ring.RemoveNode(id)
		}
		nodes = append(nodes, domain.StorageNode{
			ID:       id,
			Address:  m.Addr,
			Status:   m.Status,
			LastSeen: time.Unix(m.LastSeen, 0),
		})
	}

	c.mu.Lock()
	c.lastStatus = &domain.StorageCluster{Nodes: nodes}
	c.mu.Unlock()
}

func (c *Coordinator) GetClusterStatus(ctx context.Context) (*domain.StorageCluster, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.lastStatus == nil {
		return &domain.StorageCluster{Nodes: []domain.StorageNode{}}, nil
	}
	return c.lastStatus, nil
}

func (c *Coordinator) Assemble(ctx context.Context, bucket, key string, parts []string) (int64, error) {
	// 1. Get target nodes
	nodes := c.ring.GetNodes(bucket+"/"+key, c.replicaCount)
	if len(nodes) == 0 {
		return 0, fmt.Errorf("%s", errNoNodesAvailable)
	}

	// 2. Parallel Assemble on all replicas
	var wg sync.WaitGroup
	var mu sync.Mutex
	successCount := 0
	var lastErr error
	var size int64

	for _, nodeID := range nodes {
		client, ok := c.clients[nodeID]
		if !ok {
			continue
		}

		wg.Add(1)
		go func(_ string, cl pb.StorageNodeClient) {
			defer wg.Done()
			resp, err := cl.Assemble(ctx, &pb.AssembleRequest{
				Bucket: bucket,
				Key:    key,
				Parts:  parts,
			})
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err != nil:
				lastErr = err
			case resp.Error != "":
				lastErr = fmt.Errorf("%s", resp.Error)
			default:
				successCount++
				size = resp.Size
			}
		}(nodeID, client)
	}
	wg.Wait()

	// 3. Quorum check — all goroutines have completed, variables are visible per Go Memory Model
	if successCount < c.writeQuorum {
		return 0, fmt.Errorf("assemble quorum failed (%d/%d): %w", successCount, c.writeQuorum, lastErr)
	}
	return size, nil
}

func (c *Coordinator) Stop() {
	close(c.stopCh)
}

// Write saves data to the cluster with replication using gRPC streaming.
func (c *Coordinator) Write(ctx context.Context, bucket, key string, r io.Reader) (int64, error) {
	nodes := c.ring.GetNodes(bucket+"/"+key, c.replicaCount)
	if len(nodes) == 0 {
		return 0, fmt.Errorf("%s", errNoNodesAvailable)
	}

	ts := time.Now().UnixNano()

	// 1. Initialize streams to all target nodes
	type nodeStream struct {
		id     string
		stream pb.StorageNode_StoreClient
	}
	streams := make([]nodeStream, 0, len(nodes))
	for _, nodeID := range nodes {
		client, ok := c.clients[nodeID]
		if !ok {
			continue
		}
		st, err := client.Store(ctx)
		if err != nil {
			continue
		}
		// Send metadata first
		err = st.Send(&pb.StoreRequest{
			Payload: &pb.StoreRequest_Metadata{
				Metadata: &pb.StoreMetadata{
					Bucket:    bucket,
					Key:       key,
					Timestamp: ts,
				},
			},
		})
		if err != nil {
			continue
		}
		streams = append(streams, nodeStream{id: nodeID, stream: st})
	}

	if len(streams) == 0 {
		return 0, fmt.Errorf("failed to initialize any streams")
	}

	// 2. Pipe chunks to all streams
	buf := make([]byte, chunkSize)
	var totalSize int64
	failedNodes := make(map[string]bool)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			totalSize += int64(n)
			if totalSize > maxObjectSize {
				return totalSize, fmt.Errorf("object exceeds max size: %d bytes (max %d)", totalSize, maxObjectSize)
			}
			// Broadcast chunk — build live-streams slice excluding failed ones to avoid index skip bug
			live := streams[:0]
			for i := 0; i < len(streams); i++ {
				errSend := streams[i].stream.Send(&pb.StoreRequest{
					Payload: &pb.StoreRequest_ChunkData{
						ChunkData: buf[:n],
					},
				})
				if errSend != nil {
					_, _ = streams[i].stream.CloseAndRecv()
					failedNodes[streams[i].id] = true
					continue
				}
				live = append(live, streams[i])
			}
			streams = live
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return totalSize, err
		}
	}

	// 3. Close streams and check responses
	successCount := 0
	var lastErr error
	for _, ns := range streams {
		resp, err := ns.stream.CloseAndRecv()
		if err != nil {
			lastErr = err
			failedNodes[ns.id] = true
			continue
		}
		if resp.Success {
			successCount++
		} else {
			lastErr = fmt.Errorf("%s: %s", ns.id, resp.Error)
			failedNodes[ns.id] = true
		}
	}

	// 4. Quorum check
	if successCount < c.writeQuorum {
		platform.StorageOperations.WithLabelValues("cluster_write", bucket, "quorum_failure").Inc()
		return totalSize, fmt.Errorf("write quorum failed (%d/%d): %w", successCount, c.writeQuorum, lastErr)
	}

	// 5. Write repair: async repair of failed nodes using a good replica as source
	if len(failedNodes) > 0 {
		goodNodes := make([]string, 0, len(streams))
		for _, ns := range streams {
			goodNodes = append(goodNodes, ns.id)
		}
		repairNodes := make([]string, 0, len(failedNodes))
		for id := range failedNodes {
			repairNodes = append(repairNodes, id)
		}
		go func() {
			defer func() {
				if r := recover(); r != nil {
					platform.StorageOperations.WithLabelValues("write_repair", bucket, "panic").Inc()
				}
			}()
			c.writeRepair(context.Background(), bucket, key, ts, repairNodes, goodNodes)
		}()
	}

	platform.StorageOperations.WithLabelValues("cluster_write", bucket, "success").Inc()
	return totalSize, nil
}

// Read retrieves data from the cluster using gRPC streaming and Read Repair.
func (c *Coordinator) Read(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	nodes := c.ring.GetNodes(bucket+"/"+key, c.replicaCount)
	if len(nodes) == 0 {
		return nil, fmt.Errorf("%s", errNoNodesAvailable)
	}

	results := c.collectReadResults(ctx, bucket, key, nodes)
	winner, repairNodes, foundCount := c.processReadResults(results)

	if foundCount < c.readQuorum {
		platform.StorageOperations.WithLabelValues("cluster_read", bucket, "quorum_failure").Inc()
		return nil, fmt.Errorf("read quorum failed (%d/%d)", foundCount, c.readQuorum)
	}

	// Wrapper to handle streaming read and async repair
	winningReader := &grpcStreamReader{stream: winner.stream}

	if len(repairNodes) > 0 {
		pr, pw := io.Pipe()
		tee := io.TeeReader(winningReader, pw)

		repairCtx, cancel := context.WithTimeout(ctx, repairTimeout)
		go func() {
			defer cancel()
			// Ensure the pipe reader is closed on every exit path (panic,
			// early return inside repairNodes, etc.) so the writer side
			// upstream can unblock.
			defer func() { _ = pr.Close() }()
			c.repairNodes(repairCtx, bucket, key, pr, winner.timestamp, repairNodes)
		}()

		return &repairingReadCloser{
			Reader: tee,
			pw:     pw,
			winner: winner.stream,
		}, nil
	}

	platform.StorageOperations.WithLabelValues("cluster_read", bucket, "success").Inc()
	return &repairingReadCloser{Reader: winningReader, winner: winner.stream}, nil
}

type repairingReadCloser struct {
	io.Reader
	pw     *io.PipeWriter
	winner pb.StorageNode_RetrieveClient
}

func (r *repairingReadCloser) Close() error {
	if r.pw != nil {
		_ = r.pw.Close()
	}
	// gRPC streams are closed when their context is canceled or Recv returns EOF.
	return nil
}

type readResult struct {
	nodeID    string
	stream    pb.StorageNode_RetrieveClient
	timestamp int64
	found     bool
	err       error
}

func (c *Coordinator) collectReadResults(ctx context.Context, bucket, key string, nodes []string) chan readResult {
	results := make(chan readResult, len(nodes))
	var wg sync.WaitGroup

	for _, nodeID := range nodes {
		client, ok := c.clients[nodeID]
		if !ok {
			continue
		}
		wg.Add(1)
		go func(id string, cl pb.StorageNodeClient) {
			defer wg.Done()
			st, err := cl.Retrieve(ctx, &pb.RetrieveRequest{Bucket: bucket, Key: key})
			if err != nil {
				results <- readResult{nodeID: id, err: err}
				return
			}

			// Read only metadata
			resp, err := st.Recv()
			if err != nil {
				results <- readResult{nodeID: id, err: err}
				return
			}

			switch p := resp.Payload.(type) {
			case *pb.RetrieveResponse_Metadata:
				results <- readResult{
					nodeID:    id,
					stream:    st,
					found:     p.Metadata.Found,
					timestamp: p.Metadata.Timestamp,
				}
			default:
				results <- readResult{nodeID: id, err: fmt.Errorf("unexpected message type: %T", p)}
			}
		}(nodeID, client)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	return results
}

func (c *Coordinator) processReadResults(results chan readResult) (readResult, []string, int) {
	var latest readResult
	foundCount := 0
	var repairNodes []string
	winners := make([]readResult, 0, cap(results))

	for res := range results {
		if res.err != nil || !res.found {
			if res.err == nil && !res.found {
				repairNodes = append(repairNodes, res.nodeID)
			}
			continue
		}

		foundCount++
		if res.timestamp > latest.timestamp {
			latest = res
		}
		winners = append(winners, res)
	}

	// Add stale nodes to repair list
	for _, res := range winners {
		if res.timestamp < latest.timestamp {
			repairNodes = append(repairNodes, res.nodeID)
		}
	}

	return latest, repairNodes, foundCount
}

func (c *Coordinator) repairNodes(ctx context.Context, bucket, key string, r io.Reader, timestamp int64, nodes []string) {
	type nodeStream struct {
		id     string
		stream pb.StorageNode_StoreClient
		cancel context.CancelFunc
	}
	streams := make([]nodeStream, 0, len(nodes))
	for _, nodeID := range nodes {
		if client, ok := c.clients[nodeID]; ok {
			streamCtx, cancel := context.WithCancel(ctx)
			st, err := client.Store(streamCtx)
			if err != nil {
				cancel()
				continue
			}
			err = st.Send(&pb.StoreRequest{
				Payload: &pb.StoreRequest_Metadata{
					Metadata: &pb.StoreMetadata{
						Bucket:    bucket,
						Key:       key,
						Timestamp: timestamp,
					},
				},
			})
			if err != nil {
				// The stream is half-open: it was created on the server side but the
				// metadata frame failed. Drain it with CloseAndRecv before cancelling
				// so the server-side goroutine exits cleanly instead of waiting on
				// context cancellation.
				_, _ = st.CloseAndRecv()
				cancel()
				continue
			}
			streams = append(streams, nodeStream{id: nodeID, stream: st, cancel: cancel})
		}
	}

	if len(streams) == 0 {
		return
	}

	buf := make([]byte, chunkSize)
	var totalSize int64
	for {
		nr, err := r.Read(buf)
		if nr > 0 {
			totalSize += int64(nr)
			if totalSize > maxObjectSize {
				// Cancel all stream contexts so server-side handlers abort cleanly.
				// Drain r to unblock the caller's io.TeeReader/io.Pipe so it can exit.
				for _, ns := range streams {
					ns.cancel()
					_, _ = ns.stream.CloseAndRecv()
				}
				go func() {
					_, _ = io.Copy(io.Discard, r)
				}()
				return
			}
			// Broadcast chunk to live streams — build live-streams slice excluding failed ones to avoid index skip bug
			live := streams[:0]
			for i := 0; i < len(streams); i++ {
				errSend := streams[i].stream.Send(&pb.StoreRequest{
					Payload: &pb.StoreRequest_ChunkData{
						ChunkData: buf[:nr],
					},
				})
				if errSend != nil {
					streams[i].cancel()
					_, _ = streams[i].stream.CloseAndRecv()
					continue
				}
				live = append(live, streams[i])
			}
			streams = live
		}
		if err != nil {
			break
		}
	}

	for _, ns := range streams {
		resp, err := ns.stream.CloseAndRecv()
		if err == nil && resp.Success {
			platform.StorageOperations.WithLabelValues("repair", bucket, "success").Inc()
		} else {
			platform.StorageOperations.WithLabelValues("repair", bucket, "failure").Inc()
		}
	}
}

// writeRepair reads the object from a good replica and streams it to failed nodes.
func (c *Coordinator) writeRepair(ctx context.Context, bucket, key string, timestamp int64, repairNodes []string, goodNodes []string) {
	if len(repairNodes) == 0 || len(goodNodes) == 0 {
		return
	}

	srcNode := goodNodes[0]
	srcClient, ok := c.clients[srcNode]
	if !ok {
		return
	}

	st, err := srcClient.Retrieve(ctx, &pb.RetrieveRequest{Bucket: bucket, Key: key})
	if err != nil {
		platform.StorageOperations.WithLabelValues("write_repair", bucket, "source_error").Inc()
		return
	}

	meta, err := st.Recv()
	if err != nil || meta.GetMetadata() == nil {
		platform.StorageOperations.WithLabelValues("write_repair", bucket, "source_error").Inc()
		return
	}

	type nodeStream struct {
		id     string
		stream pb.StorageNode_StoreClient
	}
	repairStreams := make([]nodeStream, 0, len(repairNodes))
	for _, nodeID := range repairNodes {
		if client, ok := c.clients[nodeID]; ok {
			storeSt, err := client.Store(ctx)
			if err != nil {
				continue
			}
			err = storeSt.Send(&pb.StoreRequest{
				Payload: &pb.StoreRequest_Metadata{
					Metadata: &pb.StoreMetadata{
						Bucket:    bucket,
						Key:       key,
						Timestamp: timestamp,
					},
				},
			})
			if err != nil {
				continue
			}
			repairStreams = append(repairStreams, nodeStream{id: nodeID, stream: storeSt})
		}
	}

	if len(repairStreams) == 0 {
		return
	}

	for {
		chunk, err := st.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			break
		}
		cd := chunk.GetChunkData()
		if cd == nil {
			continue
		}

		live := repairStreams[:0]
		for i := 0; i < len(repairStreams); i++ {
			errSend := repairStreams[i].stream.Send(&pb.StoreRequest{
				Payload: &pb.StoreRequest_ChunkData{ChunkData: cd},
			})
			if errSend != nil {
				_, _ = repairStreams[i].stream.CloseAndRecv()
				continue
			}
			live = append(live, repairStreams[i])
		}
		repairStreams = live
	}

	for _, ns := range repairStreams {
		resp, err := ns.stream.CloseAndRecv()
		if err == nil && resp.Success {
			platform.StorageOperations.WithLabelValues("write_repair", bucket, "success").Inc()
		} else {
			platform.StorageOperations.WithLabelValues("write_repair", bucket, "failure").Inc()
		}
	}
}

type grpcStreamReader struct {
	stream pb.StorageNode_RetrieveClient
	buf    []byte
}

func (r *grpcStreamReader) Read(p []byte) (n int, err error) {
	if len(r.buf) > 0 {
		n = copy(p, r.buf)
		r.buf = r.buf[n:]
		return n, nil
	}

	resp, err := r.stream.Recv()
	if err != nil {
		return 0, err
	}

	switch pld := resp.Payload.(type) {
	case *pb.RetrieveResponse_ChunkData:
		n = copy(p, pld.ChunkData)
		if n < len(pld.ChunkData) {
			r.buf = pld.ChunkData[n:]
		}
		return n, nil
	default:
		return 0, fmt.Errorf("unexpected payload during stream: %T", pld)
	}
}

// Delete removes data from the cluster.
func (c *Coordinator) Delete(ctx context.Context, bucket, key string) error {
	nodes := c.ring.GetNodes(bucket+"/"+key, c.replicaCount)

	// Best effort delete from all replicas
	// We don't necessarily fail if one is down, but we should report if all fail.

	successCount := 0
	for _, nodeID := range nodes {
		client, ok := c.clients[nodeID]
		if !ok {
			continue
		}

		_, err := client.Delete(ctx, &pb.DeleteRequest{Bucket: bucket, Key: key})
		if err == nil {
			successCount++
		}
	}

	if successCount == 0 && len(nodes) > 0 {
		platform.StorageOperations.WithLabelValues("cluster_delete", bucket, "failure").Inc()
		return fmt.Errorf("failed to delete from any node")
	}

	platform.StorageOperations.WithLabelValues("cluster_delete", bucket, "success").Inc()
	return nil
}
