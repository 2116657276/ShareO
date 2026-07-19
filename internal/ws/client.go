package ws

import (
	"log"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 90 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 7) / 10 // ~63s

	// Maximum message size allowed from peer.
	maxMessageSize = 4096

	// Send channel buffer size.
	sendBufSize = 256
)

// Client represents a single WebSocket connection.
type Client struct {
	hub    *Hub
	conn   *websocket.Conn
	userID int64
	send   chan []byte
}

// NewClient wraps an upgraded WebSocket connection as a Client.
func NewClient(hub *Hub, conn *websocket.Conn, userID int64) *Client {
	return &Client{
		hub:    hub,
		conn:   conn,
		userID: userID,
		send:   make(chan []byte, sendBufSize),
	}
}

// readPump reads messages from the WebSocket connection.
// For this design, the server does not expect INCOMING WS messages — all sends go via REST.
// readPump only handles pong messages and detects disconnection.
func (c *Client) readPump() {
	defer func() {
		c.hub.Unregister(c)
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("ws: unexpected close for user %d: %v", c.userID, err)
			}
			break
		}
	}
}

// writePump writes messages from the send channel to the WebSocket connection.
// It also sends periodic ping messages to keep the connection alive.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// Close closes the client connection gracefully.
func (c *Client) Close() {
	c.hub.Unregister(c)
	c.conn.Close()
}

// Start launches the read and write goroutines. Blocks until the client registers.
func (c *Client) Start() {
	c.hub.Register(c)
	go c.writePump()
	go c.readPump()
}
