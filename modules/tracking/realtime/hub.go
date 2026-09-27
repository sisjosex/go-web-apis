package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// Session is who a connection is: the tenant it opened under, the user, whether that user is a
// guardian (the portal level), and the tenant's database for anything read on its behalf.
type Session struct {
	TenantID    uuid.UUID
	UserID      uuid.UUID
	Guardian    bool
	DatabaseURL string
}

// EtaSource answers a watched trip's ETA frame data and when it was computed; ok false when there is
// nothing to send (no position, no pending stop, or a failure the caller logs). riderID set is a rider
// channel: the ETA to that rider's stops only (MOBILE-010). sentAt is when the estimate last sent on
// the channel was computed, so a source can answer ok false before any read when nothing is newer.
type EtaSource func(ctx context.Context, session Session, tripID uuid.UUID, riderID *uuid.UUID, sentAt time.Time) (data json.RawMessage, computedAt time.Time, ok bool)

// healthEvery is how often the receive loop proves Valkey is still there when nothing arrives.
const healthEvery = 15 * time.Second

// Hub fans Valkey pub/sub out to WebSocket connections: one SUBSCRIBE per channel however many
// connections listen to it, dropped with the last one. No sticky sessions: a second gateway process
// subscribes the same channels on its own (D2).
type Hub struct {
	client   *redis.Client
	queueMax int
	eta      EtaSource

	mu       sync.Mutex
	pubsub   *redis.PubSub
	channels map[string]map[*Conn]bool
	etaSent  map[string]time.Time
	etaBusy  map[string]bool

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewHub builds the hub; queueMax is WS_QUEUE_MAX. eta may be nil: no ETA frames.
func NewHub(client *redis.Client, queueMax int, eta EtaSource) *Hub {
	return &Hub{
		client:   client,
		queueMax: queueMax,
		eta:      eta,
		channels: map[string]map[*Conn]bool{},
		etaSent:  map[string]time.Time{},
		etaBusy:  map[string]bool{},
	}
}

// SetEta sets the ETA source once the services it needs exist; call it before Start.
func (h *Hub) SetEta(eta EtaSource) { h.eta = eta }

// QueueMax is the per-connection limit new connections are built with.
func (h *Hub) QueueMax() int { return h.queueMax }

// Start runs the receive loop until Shutdown.
func (h *Hub) Start(ctx context.Context) {
	h.ctx, h.cancel = context.WithCancel(context.WithoutCancel(ctx))
	h.mu.Lock()
	h.pubsub = h.client.Subscribe(h.ctx)
	h.mu.Unlock()
	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
		h.receive()
	}()
	log.Println("✅ Realtime hub started")
}

// Shutdown closes every connection 1012 — clients reconnect to another process — and stops.
func (h *Hub) Shutdown() {
	h.closeAll("server restarting")
	if h.cancel != nil {
		h.cancel()
	}
	h.mu.Lock()
	if h.pubsub != nil {
		_ = h.pubsub.Close()
	}
	h.mu.Unlock()
	h.wg.Wait()
}

// Subscribe adds conn to channel, subscribing Valkey when it is the channel's first listener.
func (h *Hub) Subscribe(conn *Conn, channel string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	listeners, ok := h.channels[channel]
	if !ok {
		if err := h.pubsub.Subscribe(h.ctx, channel); err != nil {
			return err
		}
		listeners = map[*Conn]bool{}
		h.channels[channel] = listeners
	}
	listeners[conn] = true
	conn.mu.Lock()
	conn.channels[channel] = true
	conn.mu.Unlock()
	return nil
}

// Unsubscribe removes conn from channel, unsubscribing Valkey with the last listener.
func (h *Hub) Unsubscribe(conn *Conn, channel string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.drop(conn, channel)
}

// Remove takes conn off every channel it listened to — its socket is gone.
func (h *Hub) Remove(conn *Conn) {
	conn.mu.Lock()
	channels := make([]string, 0, len(conn.channels))
	for ch := range conn.channels {
		channels = append(channels, ch)
	}
	conn.mu.Unlock()
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, ch := range channels {
		h.drop(conn, ch)
	}
}

// drop is Unsubscribe with h.mu held.
func (h *Hub) drop(conn *Conn, channel string) {
	conn.mu.Lock()
	delete(conn.channels, channel)
	conn.mu.Unlock()
	listeners, ok := h.channels[channel]
	if !ok {
		return
	}
	delete(listeners, conn)
	if len(listeners) == 0 {
		delete(h.channels, channel)
		delete(h.etaSent, channel)
		_ = h.pubsub.Unsubscribe(h.ctx, channel)
	}
}

// receive reads the shared subscription. Losing Valkey closes every connection 1012 and starts over
// with an empty subscription: the clients reconnect, resubscribe and resync from the snapshots.
func (h *Hub) receive() {
	for h.ctx.Err() == nil {
		h.mu.Lock()
		ps := h.pubsub
		h.mu.Unlock()
		msg, err := ps.ReceiveTimeout(h.ctx, healthEvery)
		if h.ctx.Err() != nil {
			return
		}
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			if err = ps.Ping(h.ctx); err == nil {
				continue
			}
		}
		if err != nil {
			h.lost(err)
			continue
		}
		if m, ok := msg.(*redis.Message); ok {
			h.dispatch(m.Channel, m.Payload)
		}
	}
}

// lost is Valkey gone: every connection closed 1012, a fresh subscription, a pause before retrying.
func (h *Hub) lost(err error) {
	log.Printf("⚠️  realtime hub: Valkey lost: %v", err)
	h.closeAll("valkey unavailable")
	h.mu.Lock()
	_ = h.pubsub.Close()
	h.pubsub = h.client.Subscribe(h.ctx)
	h.mu.Unlock()
	select {
	case <-h.ctx.Done():
	case <-time.After(time.Second):
	}
}

func (h *Hub) closeAll(reason string) {
	h.mu.Lock()
	conns := map[*Conn]bool{}
	for _, listeners := range h.channels {
		for c := range listeners {
			conns[c] = true
		}
	}
	h.channels = map[string]map[*Conn]bool{}
	h.etaSent = map[string]time.Time{}
	h.mu.Unlock()
	for c := range conns {
		c.Close(StatusServiceRestart, reason)
	}
}

// dispatch hands one published frame to every listener of its channel. A position on a trip or rider
// channel someone is watching also refreshes that trip's ETA (D1, MOBILE-010).
func (h *Hub) dispatch(channel, payload string) {
	var frame Frame
	if err := json.Unmarshal([]byte(payload), &frame); err != nil {
		return
	}
	h.mu.Lock()
	listeners := make([]*Conn, 0, len(h.channels[channel]))
	for c := range h.channels[channel] {
		listeners = append(listeners, c)
	}
	h.mu.Unlock()
	for _, c := range listeners {
		c.Deliver(channel, frame)
	}
	if frame.Type == "position" && len(listeners) > 0 && h.eta != nil {
		if tripID, riderID, ok := etaTarget(channel, frame.Data); ok {
			h.refreshEta(channel, listeners[0].Session, tripID, riderID)
		}
	}
}

// etaTarget is the trip a position frame's ETA is for: the channel's own on trip:{id}, the frame's
// trip_id on rider:{id}, which also names the rider. ok false on any other channel.
func etaTarget(channel string, data json.RawMessage) (tripID uuid.UUID, riderID *uuid.UUID, ok bool) {
	if id, found := strings.CutPrefix(channel, "trip:"); found {
		tripID, err := uuid.Parse(id)
		return tripID, nil, err == nil
	}
	id, found := strings.CutPrefix(channel, "rider:")
	if !found {
		return uuid.Nil, nil, false
	}
	rider, err := uuid.Parse(id)
	if err != nil {
		return uuid.Nil, nil, false
	}
	var position struct {
		TripID *uuid.UUID `json:"trip_id"`
	}
	if json.Unmarshal(data, &position) != nil || position.TripID == nil {
		return uuid.Nil, nil, false
	}
	return *position.TripID, &rider, true
}

// refreshEta computes the channel's ETA off the receive loop, one computation per channel at a time,
// and sends it to the channel's listeners when it is newer than the last one sent. The ETA source
// caches 30 s, so a position every 3 s costs a Valkey read, not a route call.
func (h *Hub) refreshEta(channel string, session Session, tripID uuid.UUID, riderID *uuid.UUID) {
	h.mu.Lock()
	if h.etaBusy[channel] {
		h.mu.Unlock()
		return
	}
	h.etaBusy[channel] = true
	sentAt := h.etaSent[channel]
	h.mu.Unlock()

	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
		defer func() {
			h.mu.Lock()
			delete(h.etaBusy, channel)
			h.mu.Unlock()
		}()
		data, computedAt, ok := h.eta(h.ctx, session, tripID, riderID, sentAt)
		if !ok {
			return
		}
		h.mu.Lock()
		if _, watched := h.channels[channel]; !watched || h.etaSent[channel].Equal(computedAt) {
			h.mu.Unlock()
			return
		}
		h.etaSent[channel] = computedAt
		h.mu.Unlock()
		h.dispatchLocal(channel, Frame{Type: "eta", Data: data})
	}()
}

// dispatchLocal delivers a frame this process made itself to its own listeners of channel.
func (h *Hub) dispatchLocal(channel string, frame Frame) {
	h.mu.Lock()
	listeners := make([]*Conn, 0, len(h.channels[channel]))
	for c := range h.channels[channel] {
		listeners = append(listeners, c)
	}
	h.mu.Unlock()
	for _, c := range listeners {
		c.Deliver(channel, frame)
	}
}
