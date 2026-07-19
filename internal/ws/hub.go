package ws

import (
	"encoding/json"
	"log"
	"sync"
)

// Hub maintains the set of active WebSocket connections, indexed by userID.
// A single user may have multiple connections (phone + desktop).
type Hub struct {
	mu       sync.RWMutex
	conns    map[int64]map[*Client]bool // userID → set of connections
	register chan *Client

	// stop signals the Hub to shut down.
	stop chan struct{}
}

// NewHub creates and starts a Hub. The Hub runs until Stop is called.
func NewHub() *Hub {
	h := &Hub{
		conns:    make(map[int64]map[*Client]bool),
		register: make(chan *Client, 64),
		stop:     make(chan struct{}),
	}
	go h.run()
	return h
}

// run is the Hub's main loop. It processes register/unregister requests.
func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if h.conns[client.userID] == nil {
				h.conns[client.userID] = make(map[*Client]bool)
			}
			h.conns[client.userID][client] = true
			h.mu.Unlock()

		case <-h.stop:
			// Close all connections
			h.mu.Lock()
			for _, clients := range h.conns {
				for c := range clients {
					c.conn.Close()
				}
			}
			h.mu.Unlock()
			return
		}
	}
}

// Register adds a client to the Hub once its WS handshake is complete.
func (h *Hub) Register(c *Client) {
	h.register <- c
}

// Unregister removes a client from the Hub (called when connection closes).
func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if clients, ok := h.conns[c.userID]; ok {
		delete(clients, c)
		if len(clients) == 0 {
			delete(h.conns, c.userID)
		}
	}
}

// SendToUsers delivers a message to all online connections of the given users.
// msg will be JSON-encoded before sending. Silently skips offline users.
func (h *Hub) SendToUsers(userIDs []int64, msg any) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("ws.Hub: failed to marshal message: %v", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, uid := range userIDs {
		if clients, ok := h.conns[uid]; ok {
			for c := range clients {
				select {
				case c.send <- data:
				default:
					// Buffer full — close this connection
					go c.Close()
				}
			}
		}
	}
}

// IsOnline returns true if the user has at least one active WebSocket connection.
func (h *Hub) IsOnline(userID int64) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	clients, ok := h.conns[userID]
	return ok && len(clients) > 0
}

// Stop shuts down the Hub and closes all connections.
func (h *Hub) Stop() {
	close(h.stop)
}
