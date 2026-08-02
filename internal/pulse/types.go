package pulse

import (
	"context"
	"time"
)

type SSESubscriber struct {
	Ch     chan string
	Ctx    context.Context
	Cancel context.CancelFunc
}

type Client struct {
	URL       string
	Ctx       context.Context
	Cancel    context.CancelFunc
	Connected bool
	LastEvent time.Time
	Reconnect chan bool
}
