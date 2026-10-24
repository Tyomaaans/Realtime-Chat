package chat

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"realtime-chat/internal/config"
	"realtime-chat/internal/middleware"
)

type testServer struct {
	hub     *Hub
	server  *httptest.Server
	cancel  context.CancelFunc
	clients chan *client
}

type serviceCapture struct {
	service ChatService
	clients chan *client
}

func (s *serviceCapture) HandleCommand(c *client, command Command) error {
	select {
	case s.clients <- c:
	default:
	}
	return s.service.HandleCommand(c, command)
}

func startServer(t *testing.T, configure func(*config.AppConfig)) *testServer {
	t.Helper()
	cfg, err := config.NewConfig()
	if err != nil {
		t.Fatal(err)
	}
	cfg.HandshakeBurst, cfg.HandshakesPerMinute, cfg.MaxConnectionsPerIP = 1000, 1000, 500
	if configure != nil {
		configure(&cfg)
	}
	hub := NewHub(cfg)
	service := NewChatService(hub, cfg)
	clients := make(chan *client, 1)
	handler := NewChatHandler(&serviceCapture{service: service, clients: clients}, hub, middleware.NewAdmission(cfg), cfg)
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.GET("/v1/chat/ws", handler.Connect)
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)
	<-hub.started
	server := &testServer{hub: hub, server: httptest.NewServer(router), cancel: cancel, clients: clients}
	t.Cleanup(func() {
		cancel()
		server.server.Close()
		shutdown, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		if err := hub.Wait(shutdown); err != nil {
			t.Errorf("hub shutdown failed: %v", err)
		}
	})
	return server
}

func (s *testServer) connect(t *testing.T) *websocket.Conn {
	t.Helper()
	header := http.Header{"Origin": []string{"http://localhost:5173"}}
	conn, response, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(s.server.URL, "http")+"/v1/chat/ws", header)
	if response != nil && response.Body != nil {
		defer response.Body.Close()
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	readEvent(t, conn, "session.ready")
	return conn
}

type receivedEvent struct {
	Type      string          `json:"type"`
	RequestID string          `json:"request_id"`
	RoomID    string          `json:"room_id"`
	Payload   json.RawMessage `json:"payload"`
}

func readEvent(t *testing.T, conn *websocket.Conn, expected string) receivedEvent {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var event receivedEvent
	if err := conn.ReadJSON(&event); err != nil {
		t.Fatal(err)
	}
	if event.Type != expected {
		t.Fatalf("event type = %s, want %s", event.Type, expected)
	}
	return event
}

func sendCommand(t *testing.T, conn *websocket.Conn, eventType, room string, payload any) {
	t.Helper()
	_ = conn.SetWriteDeadline(time.Now().Add(time.Second))
	if err := conn.WriteJSON(Event{Type: eventType, RequestID: "request", RoomID: room, Payload: payload}); err != nil {
		t.Fatal(err)
	}
}

func eventually(t *testing.T, check func() bool) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	for {
		if check() {
			return
		}
		select {
		case <-ticker.C:
		case <-deadline.C:
			t.Fatal("condition did not become true before its deadline")
		}
	}
}

func TestRealClientsMatchRelayTypingNextAndStop(t *testing.T) {
	server := startServer(t, nil)
	a, b, c := server.connect(t), server.connect(t), server.connect(t)
	sendCommand(t, a, "queue.join", "", struct{}{})
	readEvent(t, a, "queue.waiting")
	sendCommand(t, b, "queue.join", "", struct{}{})
	room := readEvent(t, a, "chat.matched").RoomID
	if readEvent(t, b, "chat.matched").RoomID != room {
		t.Fatal("clients received different rooms")
	}
	sendCommand(t, a, "chat.send", room, map[string]string{"text": "  Hello \u4e16\u754c  "})
	self, peer := readEvent(t, a, "chat.message"), readEvent(t, b, "chat.message")
	var message MessagePayload
	if err := json.Unmarshal(peer.Payload, &message); err != nil || message.Text != "Hello \u4e16\u754c" || message.Sender != "peer" || message.MessageID == "" || self.RequestID != "request" || peer.RequestID != "" {
		t.Fatal("relay metadata or Unicode text is incorrect")
	}
	sendCommand(t, a, "chat.typing", room, map[string]bool{"typing": true})
	readEvent(t, a, "chat.typing")
	readEvent(t, b, "chat.typing")
	sendCommand(t, a, "chat.next", room, struct{}{})
	readEvent(t, a, "queue.waiting")
	readEvent(t, b, "peer.left")
	sendCommand(t, c, "queue.join", "", struct{}{})
	newRoom := readEvent(t, a, "chat.matched").RoomID
	readEvent(t, c, "chat.matched")
	sendCommand(t, a, "chat.stop", room, struct{}{})
	readEvent(t, a, "error")
	sendCommand(t, a, "chat.stop", newRoom, struct{}{})
	readEvent(t, a, "peer.left")
	readEvent(t, c, "peer.left")
}

func TestOriginsCapacityAndFailedUpgradeRollback(t *testing.T) {
	server := startServer(t, func(cfg *config.AppConfig) { cfg.MaxConnections = 1 })
	for _, origin := range []string{"", "null", "http://unknown.local", "http://localhost:5173/"} {
		request, _ := http.NewRequest(http.MethodGet, server.server.URL+"/v1/chat/ws", nil)
		if origin != "" {
			request.Header.Set("Origin", origin)
		}
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusForbidden {
			t.Fatalf("origin %q status = %d", origin, response.StatusCode)
		}
	}
	request, _ := http.NewRequest(http.MethodGet, server.server.URL+"/v1/chat/ws", nil)
	request.Header.Set("Origin", "http://localhost:5173")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid upgrade status = %d", response.StatusCode)
	}
	conn := server.connect(t)
	_, response, err = websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.server.URL, "http")+"/v1/chat/ws", http.Header{"Origin": []string{"http://localhost:5173"}})
	if err == nil || response == nil || response.StatusCode != http.StatusServiceUnavailable || response.Header.Get("Retry-After") == "" {
		t.Fatal("full server did not reject the extra connection")
	}
	response.Body.Close()
	conn.Close()
}

func TestUnsupportedFramesAndPayloadPolicy(t *testing.T) {
	tests := []struct {
		name string
		kind int
		data []byte
		code int
	}{
		{name: "binary", kind: websocket.BinaryMessage, data: []byte("binary"), code: 1003},
		{name: "invalid UTF-8", kind: websocket.TextMessage, data: []byte{0xff}, code: 1007},
		{name: "oversized", kind: websocket.TextMessage, data: []byte(strings.Repeat("a", 8193)), code: 1009},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := startServer(t, nil)
			conn := server.connect(t)
			if err := conn.WriteMessage(tt.kind, tt.data); err != nil {
				t.Fatal(err)
			}
			_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			_, _, err := conn.ReadMessage()
			if !websocket.IsCloseError(err, tt.code) {
				t.Fatalf("close = %v, want %d", err, tt.code)
			}
		})
	}
	server := startServer(t, nil)
	conn := server.connect(t)
	for range 4 {
		if err := conn.WriteMessage(websocket.TextMessage, []byte("{}")); err != nil {
			t.Fatal(err)
		}
		readEvent(t, conn, "error")
	}
	conn.WriteMessage(websocket.TextMessage, []byte("{}"))
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			if !websocket.IsCloseError(err, 1008) {
				t.Fatalf("repeated policy violation close = %v", err)
			}
			break
		}
	}
}

func TestConcurrentNextDisconnectAndPongTimeout(t *testing.T) {
	server := startServer(t, nil)
	a, b := server.connect(t), server.connect(t)
	sendCommand(t, a, "queue.join", "", struct{}{})
	readEvent(t, a, "queue.waiting")
	sendCommand(t, b, "queue.join", "", struct{}{})
	room := readEvent(t, a, "chat.matched").RoomID
	readEvent(t, b, "chat.matched")
	var wg sync.WaitGroup
	for _, conn := range []*websocket.Conn{a, b} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = conn.WriteJSON(Event{Type: "chat.next", RequestID: "next", RoomID: room, Payload: struct{}{}})
			_ = conn.Close()
		}()
	}
	wg.Wait()
	eventually(t, func() bool {
		snapshot, err := server.hub.Snapshot(context.Background())
		return err == nil && snapshot == (Snapshot{})
	})
	timeoutServer := startServer(t, func(cfg *config.AppConfig) {
		cfg.PingInterval, cfg.PongWait, cfg.WriteWait = 25*time.Millisecond, 100*time.Millisecond, 50*time.Millisecond
	})
	timeoutServer.connect(t)
	eventually(t, func() bool {
		snapshot, err := timeoutServer.hub.Snapshot(context.Background())
		return err == nil && snapshot.Connections == 0
	})
}

func TestShutdownClosesActiveSocketsAndClearsState(t *testing.T) {
	server := startServer(t, nil)
	clients := []*websocket.Conn{server.connect(t), server.connect(t), server.connect(t)}
	for _, conn := range clients {
		sendCommand(t, conn, "queue.join", "", struct{}{})
	}
	server.cancel()
	for _, conn := range clients {
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				if !websocket.IsCloseError(err, 1001) {
					t.Fatalf("shutdown close = %v", err)
				}
				break
			}
		}
	}
	<-server.hub.done
	if server.hub.Ready() || len(server.hub.clients) != 0 || len(server.hub.rooms) != 0 || server.hub.queue.Len() != 0 {
		t.Fatal("shutdown retained live state")
	}
}

func TestSlowSocketCannotBlockHubOrRetainCapacity(t *testing.T) {
	server := startServer(t, func(cfg *config.AppConfig) { cfg.WriteWait = 50 * time.Millisecond })
	conn := server.connect(t)
	if err := conn.UnderlyingConn().(*net.TCPConn).SetReadBuffer(1024); err != nil {
		t.Fatal(err)
	}
	sendCommand(t, conn, "queue.join", "", struct{}{})
	c := <-server.clients
	if err := c.conn.UnderlyingConn().(*net.TCPConn).SetWriteBuffer(1024); err != nil {
		t.Fatal(err)
	}
	for range 100 {
		server.hub.submit(hubCommand{kind: "event", client: c, event: Event{Type: "chat.message", RoomID: "test-room", Payload: MessagePayload{Text: strings.Repeat("a", 2000)}}})
	}
	eventually(t, func() bool {
		snapshot, err := server.hub.Snapshot(context.Background())
		return err == nil && snapshot.Connections == 0
	})
	server.connect(t)
}

func TestHandshakeRateAndForwardedHeadersCannotBypassIPQuota(t *testing.T) {
	server := startServer(t, func(cfg *config.AppConfig) { cfg.MaxConnectionsPerIP = 5 })
	for range 5 {
		server.connect(t)
	}
	header := http.Header{"Origin": []string{"http://localhost:5173"}, "X-Forwarded-For": []string{"192.0.2.123"}}
	_, response, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.server.URL, "http")+"/v1/chat/ws", header)
	if err == nil || response == nil || response.StatusCode != 429 {
		t.Fatal("a forwarded IP bypassed the loopback quota")
	}
	response.Body.Close()
	limited := startServer(t, func(cfg *config.AppConfig) { cfg.HandshakeBurst, cfg.HandshakesPerMinute = 1, 1 })
	limited.connect(t)
	_, response, err = websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(limited.server.URL, "http")+"/v1/chat/ws", header)
	if err == nil || response == nil || response.StatusCode != 429 || response.Header.Get("Retry-After") == "" {
		t.Fatal("handshake rate did not reject a second upgrade")
	}
	response.Body.Close()
}
