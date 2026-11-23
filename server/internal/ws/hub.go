package ws

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/gofiber/websocket/v2"
)

type Envelope struct {
	Type      string          `json:"type"`
	RequestID string          `json:"requestId,omitempty"`
	TS        int64           `json:"ts"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

type Client struct {
	ID             string
	Conn           *websocket.Conn
	UserID         int64
	SessionVersion int64
	CreatedAt      time.Time
	send           chan any
	mu             sync.RWMutex
	closed         bool
}

type Hub struct {
	mu      sync.RWMutex
	clients map[*websocket.Conn]*Client
}

var (
	ErrClientNotFound  = errors.New("ws client not found")
	ErrClientClosed    = errors.New("ws client closed")
	ErrClientQueueFull = errors.New("ws client send queue full")
)

const defaultSendQueueSize = 64

func NewHub() *Hub {
	return &Hub{clients: make(map[*websocket.Conn]*Client)}
}

func (h *Hub) Register(conn *websocket.Conn) *Client {
	h.mu.Lock()
	defer h.mu.Unlock()
	c := &Client{
		ID:        newClientID(),
		Conn:      conn,
		CreatedAt: time.Now(),
		send:      make(chan any, defaultSendQueueSize),
	}
	h.clients[conn] = c
	return c
}

func (h *Hub) Unregister(conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	client, ok := h.clients[conn]
	if !ok {
		return
	}
	delete(h.clients, conn)
	client.closeSend()
}

func (h *Hub) Broadcast(t string, payload any) {
	env := map[string]any{
		"type":    t,
		"ts":      time.Now().UnixMilli(),
		"payload": payload,
	}
	for _, client := range h.Clients() {
		_ = client.enqueue(env)
	}
}

func (h *Hub) Send(conn *websocket.Conn, payload any) error {
	h.mu.RLock()
	client, ok := h.clients[conn]
	h.mu.RUnlock()
	if !ok {
		return ErrClientNotFound
	}
	return client.enqueue(payload)
}

func (h *Hub) WritePump(client *Client) error {
	for payload := range client.send {
		switch typed := payload.(type) {
		case []byte:
			if err := client.Conn.WriteMessage(websocket.BinaryMessage, typed); err != nil {
				return err
			}
		default:
			if err := client.Conn.WriteJSON(payload); err != nil {
				return err
			}
		}
	}
	return nil
}

func (h *Hub) Clients() []*Client {
	if h == nil {
		return nil
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]*Client, 0, len(h.clients))
	for _, client := range h.clients {
		out = append(out, client)
	}
	return out
}

func (c *Client) enqueue(payload any) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed {
		return ErrClientClosed
	}
	select {
	case c.send <- payload:
		return nil
	default:
		return ErrClientQueueFull
	}
}

func (c *Client) closeSend() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	c.closed = true
	close(c.send)
}

func newClientID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return hex.EncodeToString([]byte(time.Now().Format("150405.000")))
	}
	return hex.EncodeToString(buf)
}
