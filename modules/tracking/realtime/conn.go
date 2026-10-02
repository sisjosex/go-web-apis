// Package realtime is the WebSocket gateway of TRACK-025: a hub that holds one Valkey subscription
// per channel for every connection of the process, the per-connection outbound queue, and the
// endpoint that authorises each subscribe.
package realtime

import (
	"encoding/json"
	"sync"

	"github.com/coder/websocket"
)

// Frame types that only the latest one matters of: a newer one replaces the queued one (per channel)
// instead of queueing behind it, so a slow reader costs memory for one frame, not for all of them.
var latestWins = map[string]bool{"position": true, "eta": true}

// Close codes (RFC 6455 registry): 1012 the server is restarting or lost Valkey — reconnect;
// 1013 this connection fell too far behind — reconnect and resync. 4401 (private range) the
// session's token expired — reconnect at once with a fresh one, every channel is authorised again.
const (
	StatusServiceRestart = websocket.StatusCode(1012)
	StatusTryAgainLater  = websocket.StatusCode(1013)
	StatusTokenExpired   = websocket.StatusCode(4401)
)

// Frame is what the hub receives from Valkey: the publisher's {type, data}.
type Frame struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// wireFrame is what a client receives: the frame, its channel and its seq on this connection.
type wireFrame struct {
	Channel string          `json:"channel"`
	Type    string          `json:"type"`
	Seq     int64           `json:"seq"`
	Data    json.RawMessage `json:"data,omitempty"`
	Code    string          `json:"code,omitempty"`
}

type queued struct {
	channel string
	frame   Frame
	code    string
}

// Conn is one client's outbound side: a bounded queue of frames that must all arrive (stop,
// trip_status, task, error) and a latest-wins slot per channel for position and eta. The writer
// drains it; Deliver never blocks the hub.
type Conn struct {
	mu       sync.Mutex
	queueMax int
	queue    []queued
	latest   map[string]queued // key: channel + "\x00" + type
	order    []string
	seq      map[string]int64
	channels map[string]bool
	closed   bool
	code     websocket.StatusCode
	reason   string

	wake chan struct{}
	done chan struct{}

	// Who this connection is, for the subscribe check and the ETA's tenant database.
	Session Session
}

// NewConn builds a connection's queue; queueMax is WS_QUEUE_MAX.
func NewConn(session Session, queueMax int) *Conn {
	return &Conn{
		queueMax: queueMax,
		latest:   map[string]queued{},
		seq:      map[string]int64{},
		channels: map[string]bool{},
		wake:     make(chan struct{}, 1),
		done:     make(chan struct{}),
		Session:  session,
	}
}

// Deliver queues a frame of channel. A latest-wins frame replaces the one waiting; any other frame
// queues, and one past queueMax closes the connection 1013.
func (c *Conn) Deliver(channel string, frame Frame) {
	c.enqueue(queued{channel: channel, frame: frame})
}

// Refuse queues an error frame for channel, e.g. a subscribe that was not allowed.
func (c *Conn) Refuse(channel, code string) {
	c.enqueue(queued{channel: channel, frame: Frame{Type: "error"}, code: code})
}

func (c *Conn) enqueue(q queued) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	if latestWins[q.frame.Type] {
		key := q.channel + "\x00" + q.frame.Type
		if _, waiting := c.latest[key]; !waiting {
			c.order = append(c.order, key)
		}
		c.latest[key] = q
	} else {
		if len(c.queue) >= c.queueMax {
			c.mu.Unlock()
			c.Close(StatusTryAgainLater, "too slow")
			return
		}
		c.queue = append(c.queue, q)
	}
	c.mu.Unlock()
	select {
	case c.wake <- struct{}{}:
	default:
	}
}

// Drain takes everything waiting, in order — the queue first, then the latest-wins slots — each
// numbered with its channel's next seq on this connection.
func (c *Conn) Drain() [][]byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	pending := c.queue
	for _, key := range c.order {
		pending = append(pending, c.latest[key])
	}
	c.queue, c.order = nil, nil
	c.latest = map[string]queued{}

	out := make([][]byte, 0, len(pending))
	for _, q := range pending {
		c.seq[q.channel]++
		raw, err := json.Marshal(wireFrame{Channel: q.channel, Type: q.frame.Type, Seq: c.seq[q.channel], Data: q.frame.Data, Code: q.code})
		if err == nil {
			out = append(out, raw)
		}
	}
	return out
}

// Close marks the connection closed with code; the writer sends the close frame. Idempotent: the
// first code wins.
func (c *Conn) Close(code websocket.StatusCode, reason string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	c.closed, c.code, c.reason = true, code, reason
	close(c.done)
}

// Closed reports the close code and reason once the connection is closed.
func (c *Conn) Closed() (websocket.StatusCode, string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.code, c.reason, c.closed
}

// Wake fires when something was queued; Done when the connection was closed.
func (c *Conn) Wake() <-chan struct{} { return c.wake }
func (c *Conn) Done() <-chan struct{} { return c.done }
