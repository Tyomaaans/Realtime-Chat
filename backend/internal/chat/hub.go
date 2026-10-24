package chat

import (
	"container/list"
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"realtime-chat/internal/chat/domain"
	"realtime-chat/internal/config"
)

type hubCommand struct {
	kind     string
	client   *client
	command  Command
	event    Event
	response chan error
	snapshot chan Snapshot
}

type waitEntry struct {
	client    *client
	requestID string
	expiresAt time.Time
}

type Snapshot struct {
	Connections int
	Waiting     int
	Rooms       int
}

type Hub struct {
	cfg       config.AppConfig
	commands  chan hubCommand
	done      chan struct{}
	started   chan struct{}
	accepting atomic.Bool
	pumps     sync.WaitGroup
	clients   map[string]*client
	rooms     map[string]domain.Room
	queue     *list.List
	queued    map[string]*list.Element
}

func NewHub(cfg config.AppConfig) *Hub {
	return &Hub{
		cfg:      cfg,
		commands: make(chan hubCommand, cfg.CommandBuffer),
		done:     make(chan struct{}),
		started:  make(chan struct{}),
		clients:  make(map[string]*client),
		rooms:    make(map[string]domain.Room),
		queue:    list.New(),
		queued:   make(map[string]*list.Element),
	}
}

func (h *Hub) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	defer close(h.done)
	h.accepting.Store(true)
	close(h.started)
	for {
		select {
		case <-ctx.Done():
			h.accepting.Store(false)
			for _, c := range h.clients {
				c.terminate(websocket.CloseGoingAway)
				h.remove(c, "disconnected")
			}
			return
		case command := <-h.commands:
			h.removeClosed()
			switch command.kind {
			case "register":
				if !h.accepting.Load() {
					command.response <- ErrServerBusy
					continue
				}
				c := command.client
				h.clients[c.ID] = c
				h.pumps.Add(2)
				go c.readPump(&h.pumps)
				go c.writePump(&h.pumps)
				h.emit(c, Event{Type: "session.ready", Payload: StatePayload{State: "idle"}})
				command.response <- nil
			case "command":
				if h.clients[command.client.ID] == command.client {
					h.handle(command.client, command.command)
				}
			case "event":
				if h.clients[command.client.ID] == command.client {
					h.emit(command.client, command.event)
				}
			case "snapshot":
				command.snapshot <- Snapshot{Connections: len(h.clients), Waiting: h.queue.Len(), Rooms: len(h.rooms)}
			}
		case now := <-ticker.C:
			h.removeClosed()
			h.expire(now)
		}
	}
}

func (h *Hub) Ready() bool {
	return h.accepting.Load()
}

func (h *Hub) StopAdmission() {
	h.accepting.Store(false)
}

func (h *Hub) Wait(ctx context.Context) error {
	select {
	case <-h.done:
	case <-ctx.Done():
		return ctx.Err()
	}
	finished := make(chan struct{})
	go func() {
		h.pumps.Wait()
		close(finished)
	}()
	select {
	case <-finished:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (h *Hub) Snapshot(ctx context.Context) (Snapshot, error) {
	response := make(chan Snapshot, 1)
	if !h.submit(hubCommand{kind: "snapshot", snapshot: response}) {
		return Snapshot{}, ErrServerBusy
	}
	select {
	case snapshot := <-response:
		return snapshot, nil
	case <-h.done:
		return Snapshot{}, ErrServerBusy
	case <-ctx.Done():
		return Snapshot{}, ctx.Err()
	}
}

func (h *Hub) register(c *client) error {
	response := make(chan error, 1)
	if !h.submit(hubCommand{kind: "register", client: c, response: response}) {
		return ErrServerBusy
	}
	select {
	case err := <-response:
		return err
	case <-h.done:
		return ErrServerBusy
	}
}

func (h *Hub) submit(command hubCommand) bool {
	select {
	case <-h.done:
		return false
	default:
	}
	select {
	case h.commands <- command:
		return true
	default:
		if command.client != nil {
			command.client.terminate(websocket.ClosePolicyViolation)
		}
		return false
	}
}

func (h *Hub) handle(c *client, command Command) {
	if command.Type == "queue.join" {
		if c.State == domain.Matched {
			h.reject(c, command, ErrInvalidState)
			return
		}
		h.join(c, command.RequestID)
		return
	}
	if command.Type == "queue.cancel" {
		if c.State != domain.Waiting {
			h.reject(c, command, ErrInvalidState)
			return
		}
		h.dequeue(c)
		h.emit(c, Event{Type: "queue.cancelled", RequestID: command.RequestID, Payload: StatePayload{State: "idle", Reason: "cancelled"}})
		return
	}
	if c.State != domain.Matched {
		h.reject(c, command, ErrInvalidState)
		return
	}
	if c.RoomID != command.RoomID {
		h.reject(c, command, ErrStaleRoom)
		return
	}
	room, ok := h.rooms[c.RoomID]
	if !ok || (room.Members[0] != c.ID && room.Members[1] != c.ID) {
		h.reject(c, command, ErrStaleRoom)
		return
	}
	peerID := room.Members[0]
	if peerID == c.ID {
		peerID = room.Members[1]
	}
	peer := h.clients[peerID]
	switch command.Type {
	case "chat.stop", "chat.next":
		reason := "stopped"
		if command.Type == "chat.next" {
			reason = "next"
		}
		h.endRoom(c, reason)
		if command.Type == "chat.next" {
			h.join(c, command.RequestID)
			return
		}
		h.emit(c, Event{Type: "peer.left", RequestID: command.RequestID, RoomID: command.RoomID, Payload: StatePayload{State: "idle", Reason: reason}})
	case "chat.send":
		id, err := randomID()
		if err != nil {
			h.reject(c, command, ErrInternal)
			return
		}
		payload := MessagePayload{MessageID: id, Text: command.text, Sender: "peer", SentAt: time.Now().UTC().Format(time.RFC3339Nano)}
		h.emit(peer, Event{Type: "chat.message", RoomID: command.RoomID, Payload: payload})
		payload.Sender = "self"
		h.emit(c, Event{Type: "chat.message", RequestID: command.RequestID, RoomID: command.RoomID, Payload: payload})
	case "chat.typing":
		h.emit(peer, Event{Type: "chat.typing", RoomID: command.RoomID, Payload: TypingPayload{Typing: command.typing, Sender: "peer"}})
		h.emit(c, Event{Type: "chat.typing", RequestID: command.RequestID, RoomID: command.RoomID, Payload: TypingPayload{Typing: command.typing, Sender: "self"}})
	}
}

func (h *Hub) join(c *client, requestID string) {
	if entry := h.queued[c.ID]; entry != nil {
		waiting := entry.Value.(waitEntry)
		h.emit(c, waitingEvent(requestID, waiting.expiresAt))
		return
	}
	for element := h.queue.Front(); element != nil; element = element.Next() {
		waiting := element.Value.(waitEntry)
		peer := waiting.client
		if peer.ID == c.ID || peer.LastPartner == c.ID || c.LastPartner == peer.ID || peer.closed() {
			continue
		}
		id, err := randomID()
		if err != nil {
			h.reject(c, Command{RequestID: requestID}, ErrInternal)
			return
		}
		h.dequeue(peer)
		h.rooms[id] = domain.Room{ID: id, Members: [2]string{peer.ID, c.ID}}
		c.State, peer.State = domain.Matched, domain.Matched
		c.RoomID, peer.RoomID = id, id
		c.LastPartner, peer.LastPartner = peer.ID, c.ID
		h.emit(peer, Event{Type: "chat.matched", RoomID: id, RequestID: waiting.requestID, Payload: StatePayload{State: "matched"}})
		if c.State == domain.Matched {
			h.emit(c, Event{Type: "chat.matched", RoomID: id, RequestID: requestID, Payload: StatePayload{State: "matched"}})
		} else if h.clients[c.ID] == c {
			h.join(c, requestID)
		}
		return
	}
	if h.queue.Len() >= h.cfg.QueueCapacity {
		h.reject(c, Command{RequestID: requestID}, ErrQueueFull)
		return
	}
	expiresAt := time.Now().Add(h.cfg.QueueWait)
	c.State = domain.Waiting
	h.queued[c.ID] = h.queue.PushBack(waitEntry{client: c, requestID: requestID, expiresAt: expiresAt})
	h.emit(c, waitingEvent(requestID, expiresAt))
}

func (h *Hub) dequeue(c *client) {
	if element := h.queued[c.ID]; element != nil {
		h.queue.Remove(element)
		delete(h.queued, c.ID)
	}
	c.State = domain.Idle
}

func (h *Hub) endRoom(c *client, reason string) {
	room, ok := h.rooms[c.RoomID]
	if !ok {
		return
	}
	delete(h.rooms, room.ID)
	c.RoomID, c.State = "", domain.Idle
	for _, id := range room.Members {
		if id == c.ID {
			continue
		}
		if peer := h.clients[id]; peer != nil {
			peer.RoomID, peer.State = "", domain.Idle
			h.emit(peer, Event{Type: "peer.left", RoomID: room.ID, Payload: StatePayload{State: "idle", Reason: reason}})
		}
	}
}

func (h *Hub) remove(c *client, reason string) {
	if h.clients[c.ID] != c {
		return
	}
	delete(h.clients, c.ID)
	h.dequeue(c)
	h.endRoom(c, reason)
	for _, other := range h.clients {
		if other.LastPartner == c.ID {
			other.LastPartner = ""
		}
	}
	c.LastPartner = ""
	c.terminate(websocket.CloseNormalClosure)
	close(c.send)
}

func (h *Hub) removeClosed() {
	for _, c := range h.clients {
		if c.closed() {
			h.remove(c, "disconnected")
		}
	}
}

func (h *Hub) expire(now time.Time) {
	for element := h.queue.Front(); element != nil; {
		next := element.Next()
		waiting := element.Value.(waitEntry)
		if !now.Before(waiting.expiresAt) {
			h.dequeue(waiting.client)
			h.emit(waiting.client, Event{Type: "queue.cancelled", Payload: StatePayload{State: "idle", Reason: "queue_timeout"}})
		}
		element = next
	}
}

func (h *Hub) emit(c *client, event Event) {
	if c == nil || h.clients[c.ID] != c {
		return
	}
	select {
	case c.send <- event:
	default:
		c.terminate(websocket.ClosePolicyViolation)
		h.remove(c, "slow_consumer")
	}
}

func (h *Hub) reject(c *client, command Command, err error) {
	payload := publicError(err)
	payload.State = string(c.State)
	h.emit(c, Event{Type: "error", RequestID: command.RequestID, RoomID: command.RoomID, Payload: payload})
}

func waitingEvent(requestID string, expiresAt time.Time) Event {
	return Event{Type: "queue.waiting", RequestID: requestID, Payload: StatePayload{State: "waiting", ExpiresAt: expiresAt.UTC().Format(time.RFC3339Nano)}}
}

func randomID() (string, error) {
	var data [16]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(data[:]), nil
}
