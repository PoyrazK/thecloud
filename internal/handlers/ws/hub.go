// Package ws provides WebSocket handler components.
package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/google/uuid"
	"github.com/poyrazk/thecloud/internal/core/domain"
	"github.com/poyrazk/thecloud/internal/platform"
)

// Hub maintains active WebSocket connections and broadcasts messages.
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	stopCh     chan struct{}
	stoppedCh  chan struct{}
	isOpen     atomic.Bool
	mu         sync.RWMutex
	logger     *slog.Logger
}

// NewHub creates a new WebSocket hub.
func NewHub(logger *slog.Logger) *Hub {
	h := &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		stopCh:     make(chan struct{}),
		stoppedCh:  make(chan struct{}),
		logger:     logger,
	}
	h.isOpen.Store(true)
	return h
}

// Run starts the hub's main loop.
func (h *Hub) Run() {
	defer close(h.stoppedCh)
	for {
		select {
		case <-h.stopCh:
			h.cleanupClients()
			return
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			platform.WSConnectionsActive.Inc()
			h.logger.Debug("client connected", slog.Int("total", len(h.clients)))

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			platform.WSConnectionsActive.Dec()
			h.logger.Debug("client disconnected", slog.Int("total", len(h.clients)))

		case message := <-h.broadcast:
			h.mu.Lock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					// Client send buffer is full — remove client directly while locked.
					// Do NOT send to unregister (which needs the lock) to avoid deadlock.
					// Decrement is handled by BroadcastEventToTenant for consistency.
					delete(h.clients, client)
					close(client.send)
				}
			}
			h.mu.Unlock()
		}
	}
}

// cleanupClients closes all client connections and decrements metrics.
func (h *Hub) cleanupClients() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for client := range h.clients {
		close(client.send)
		delete(h.clients, client)
		platform.WSConnectionsActive.Dec()
	}
}

// Stop signals the hub to shut down. Safe to call multiple times.
func (h *Hub) Stop() {
	if h.isOpen.Load() {
		h.isOpen.Store(false)
		close(h.stopCh)
	}
}

// Stopped returns a channel that is closed when Run() has exited.
func (h *Hub) Stopped() <-chan struct{} {
	return h.stoppedCh
}

// BroadcastEvent sends a WSEvent to all connected clients.
func (h *Hub) BroadcastEvent(event *domain.WSEvent) {
	if !h.isOpen.Load() {
		return
	}
	data, err := json.Marshal(event)
	if err != nil {
		h.logger.Error("failed to marshal event", slog.String("error", err.Error()))
		return
	}
	h.broadcast <- data
}

// BroadcastEventToTenant sends a WSEvent only to clients matching the given tenant.
// If userID is not nil, further filter to that specific user.
// Collects clients to remove under RLock, then removes them via unregister channel.
func (h *Hub) BroadcastEventToTenant(ctx context.Context, event *domain.WSEvent, tenantID uuid.UUID, userID *uuid.UUID) error {
	if !h.isOpen.Load() {
		return nil
	}
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	// Collect clients to notify under read lock
	h.mu.RLock()
	var toRemove []*Client
	for client := range h.clients {
		if client.tenantID != tenantID {
			continue
		}
		if userID != nil && client.userID != userID.String() {
			continue
		}
		select {
		case client.send <- data:
		default:
			// Client buffer full - mark for removal
			toRemove = append(toRemove, client)
		}
	}
	h.mu.RUnlock()

	// Remove dead clients directly - don't go through Unregister to avoid double-decrement.
	// ReadPump's deferred Unregister will find client already removed and skip Decrement.
	for _, client := range toRemove {
		h.mu.Lock()
		if _, ok := h.clients[client]; ok {
			delete(h.clients, client)
			close(client.send)
			platform.WSConnectionsActive.Dec()
		}
		h.mu.Unlock()
	}

	return nil
}

// PublishEvent implements ports.RealtimePublisher.
func (h *Hub) PublishEvent(ctx context.Context, event *domain.WSEvent, tenantID uuid.UUID, userID *uuid.UUID) error {
	return h.BroadcastEventToTenant(ctx, event, tenantID, userID)
}

// ClientCount returns the number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// Register adds a client to the hub.
func (h *Hub) Register(client *Client) {
	if !h.isOpen.Load() {
		return
	}
	h.register <- client
}

// Unregister removes a client from the hub.
func (h *Hub) Unregister(client *Client) {
	if !h.isOpen.Load() {
		return
	}
	h.unregister <- client
}
