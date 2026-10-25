package session

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/gorilla/websocket"

	"realtime-chat/internal/chat/domain"
	"realtime-chat/internal/config"
	"realtime-chat/pkg"
)

type client struct {
	domain.ClientEntity
	conn       *websocket.Conn
	hub        *Hub
	chatSvc    ChatService
	cfg        config.AppConfig
	send       chan ChatEventResponse
	done       chan struct{}
	closeOnce  sync.Once
	closeCode  atomic.Int32
	closeBy    atomic.Int64
	remaining  atomic.Int32
	release    func()
	commands   pkg.TokenBucket
	messages   pkg.TokenBucket
	queue      pkg.TokenBucket
	typing     pkg.TokenBucket
	violations []time.Time
}

func newClient(
	id string,
	conn *websocket.Conn,
	hub *Hub,
	chatSvc ChatService,
	cfg config.AppConfig,
	release func(),
) *client {
	now := time.Now()
	c := &client{
		ClientEntity: domain.ClientEntity{
			ID:    id,
			State: domain.Idle,
		},
		conn:     conn,
		hub:      hub,
		chatSvc:  chatSvc,
		cfg:      cfg,
		release:  release,
		send:     make(chan ChatEventResponse, cfg.OutboundBuffer),
		done:     make(chan struct{}),
		commands: pkg.NewTokenBucket(20, 10, now),
		messages: pkg.NewTokenBucket(5, 2, now),
		queue:    pkg.NewTokenBucket(3, 0.1, now),
		typing:   pkg.NewTokenBucket(2, 2, now),
	}
	c.remaining.Store(2)
	return c
}

func (c *client) readPump(wg *sync.WaitGroup) {
	defer c.pumpDone(wg)
	defer c.terminate(websocket.CloseNormalClosure)
	c.conn.SetReadLimit(int64(c.cfg.FrameBytes))
	if err := c.conn.SetReadDeadline(time.Now().Add(c.cfg.PongWait)); err != nil {
		return
	}

	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(c.cfg.PongWait))
	})
	c.conn.SetPingHandler(func(data string) error {
		if !c.hub.submit(hubCommand{
			kind:   "event",
			client: c,
			event: ChatEventResponse{
				control: websocket.PongMessage,
				data:    []byte(data),
			},
		}) {
			return ErrServerBusy
		}

		return nil
	})
	c.conn.SetCloseHandler(func(code int, _ string) error {
		if code == websocket.CloseNoStatusReceived || code == websocket.CloseAbnormalClosure {
			code = websocket.CloseNormalClosure
		}

		c.terminate(code)
		return nil
	})
	for {
		messageType, data, err := c.conn.ReadMessage()
		if err != nil {
			return
		}

		if messageType != websocket.TextMessage {
			c.terminate(websocket.CloseUnsupportedData)
			return
		}

		if !utf8.Valid(data) {
			c.terminate(websocket.CloseInvalidFramePayloadData)
			return
		}

		command, err := ParseChatCommand(data)
		retry := c.limit(command.Type, time.Now())
		if retry > 0 {
			err = ErrRateLimited
		}

		if err == nil {
			err = c.chatSvc.HandleCommand(c, command)
		}

		if err == nil {
			continue
		}

		if errors.Is(err, ErrServerBusy) {
			c.terminate(websocket.ClosePolicyViolation)
			return
		}

		payload := ToErrorResponse(err)
		payload.RetryAfterMS = retry.Milliseconds()
		if !c.hub.submit(hubCommand{
			kind:   "event",
			client: c,
			event: ChatEventResponse{
				Type:      "error",
				RequestID: command.RequestID,
				RoomID:    command.RoomID,
				Payload:   payload,
			},
		}) {
			return
		}

		if c.violation(time.Now()) {
			c.terminate(websocket.ClosePolicyViolation)
			return
		}
	}
}

func (c *client) writePump(wg *sync.WaitGroup) {
	defer c.pumpDone(wg)
	defer c.conn.Close()
	defer c.terminate(websocket.CloseNormalClosure)
	ticker := time.NewTicker(c.cfg.PingInterval)
	defer ticker.Stop()
	for {
		if c.closed() {
			c.writeClose()
			return
		}

		select {
		case <-c.done:
			c.writeClose()
			return
		case event, ok := <-c.send:
			if !ok {
				c.writeClose()
				return
			}

			if err := c.conn.SetWriteDeadline(time.Now().Add(c.cfg.WriteWait)); err != nil {
				return
			}

			if event.control != 0 {
				if err := c.conn.WriteMessage(event.control, event.data); err != nil {
					return
				}

				continue
			}

			if err := c.conn.WriteJSON(event); err != nil {
				return
			}
		case <-ticker.C:
			if err := c.conn.SetWriteDeadline(time.Now().Add(c.cfg.WriteWait)); err != nil {
				return
			}

			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *client) pumpDone(wg *sync.WaitGroup) {
	if c.remaining.Add(-1) == 0 {
		c.release()
	}

	wg.Done()
}

func (c *client) writeClose() {
	deadline := time.Unix(0, c.closeBy.Load())
	_ = c.conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(int(c.closeCode.Load()), ""), deadline)
}

func (c *client) terminate(code int) {
	c.closeOnce.Do(func() {
		c.closeCode.Store(int32(code))
		c.closeBy.Store(time.Now().Add(min(c.cfg.WriteWait, c.cfg.ShutdownTimeout)).UnixNano())
		close(c.done)
	})
}

func (c *client) closed() bool {
	select {
	case <-c.done:
		return true
	default:
		return false
	}
}

func (c *client) limit(eventType string, now time.Time) time.Duration {
	retry := c.commands.Take(now)
	switch eventType {
	case "chat.send":
		retry = max(retry, c.messages.Take(now))
	case "queue.join", "queue.cancel", "chat.next":
		retry = max(retry, c.queue.Take(now))
	case "chat.typing":
		retry = max(retry, c.typing.Take(now))
	}

	return retry
}

func (c *client) violation(now time.Time) bool {
	active := c.violations[:0]
	for _, previous := range c.violations {
		if now.Sub(previous) <= 30*time.Second {
			active = append(active, previous)
		}
	}

	c.violations = append(active, now)
	return len(c.violations) >= 5
}
