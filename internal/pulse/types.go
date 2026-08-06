package pulse

import (
	"context"
	"time"
)

// Subscriber is a connected SSE consumer on the server side.
type Subscriber struct {
	Ch     chan string
	Ctx    context.Context
	Cancel context.CancelFunc
}

// Client tracks an outbound SSE connection to a pulse server.
type Client struct {
	URL       string
	Ctx       context.Context
	Cancel    context.CancelFunc
	Connected bool
	LastEvent time.Time
	Reconnect chan bool
}

// EventHandler is called for each complete SSE event.
type EventHandler func(eventType, data string)

// DefaultServers are well-known relay pulse endpoints for local nodes.
var DefaultServers = []string{
	"https://prismatec.onrender.com/api/pulse",
}
