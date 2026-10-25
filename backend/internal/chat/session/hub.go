package session

import (
	"container/list"
	"context"
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
	command  ChatCommandRequest
	event    ChatEventResponse
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
	rooms     map[string]domain.RoomEntity
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
		rooms:    make(map[string]domain.RoomEntity),
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
				h.emit(c, ChatEventResponse{
					Type:    "session.ready",
					Payload: StateResponse{State: "idle"},
				})
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
				command.snapshot <- Snapshot{
					Connections: len(h.clients),
					Waiting:     h.queue.Len(),
					Rooms:       len(h.rooms),
				}
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
	if !h.submit(hubCommand{
		kind:     "snapshot",
		snapshot: response,
	}) {
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
	if !h.submit(hubCommand{
		kind:     "register",
		client:   c,
		response: response,
	}) {
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

func (h *Hub) emit(c *client, event ChatEventResponse) {
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

func (h *Hub) reject(c *client, command ChatCommandRequest, err error) {
	payload := ToErrorResponse(err)
	payload.State = string(c.State)
	h.emit(c, ChatEventResponse{
		Type:      "error",
		RequestID: command.RequestID,
		RoomID:    command.RoomID,
		Payload:   payload,
	})
}
