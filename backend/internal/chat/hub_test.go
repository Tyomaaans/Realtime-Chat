package chat

import (
	"fmt"
	"testing"
	"time"

	"realtime-chat/internal/chat/domain"
	"realtime-chat/internal/config"
)

func memoryHub(t *testing.T, count int) (*Hub, []*client) {
	t.Helper()
	cfg, err := config.NewConfig()
	if err != nil {
		t.Fatal(err)
	}
	hub := NewHub(cfg)
	clients := make([]*client, count)
	for i := range clients {
		c := newClient(fmt.Sprintf("client-%d", i), nil, hub, nil, cfg, func() {})
		hub.clients[c.ID] = c
		clients[i] = c
	}
	return hub, clients
}

func drain(c *client) {
	for len(c.send) > 0 {
		<-c.send
	}
}

func assertMembership(t *testing.T, hub *Hub) {
	t.Helper()
	members := make(map[string]bool)
	for _, room := range hub.rooms {
		if room.Members[0] == room.Members[1] {
			t.Fatal("self match")
		}
		for _, id := range room.Members {
			c := hub.clients[id]
			if c == nil || c.State != domain.Matched || c.RoomID != room.ID || members[id] || hub.queued[id] != nil {
				t.Fatal("invalid or duplicate room membership")
			}
			members[id] = true
		}
	}
	if hub.queue.Len() != len(hub.queued) {
		t.Fatal("queue index contains stale entries")
	}
	for element := hub.queue.Front(); element != nil; element = element.Next() {
		c := element.Value.(waitEntry).client
		if c.State != domain.Waiting || c.RoomID != "" || members[c.ID] || hub.queued[c.ID] != element {
			t.Fatal("invalid queue membership")
		}
		members[c.ID] = true
	}
}

func TestFIFOQueueDuplicateJoinCancellationAndTimeout(t *testing.T) {
	hub, clients := memoryHub(t, 5)
	for i, c := range clients {
		hub.join(c, fmt.Sprintf("join-%d", i))
	}
	if len(hub.rooms) != 2 || hub.queue.Len() != 1 || clients[0].RoomID != clients[1].RoomID || clients[2].RoomID != clients[3].RoomID {
		t.Fatal("FIFO matching or odd queue count is incorrect")
	}
	hub.join(clients[4], "repeat")
	if hub.queue.Len() != 1 {
		t.Fatal("duplicate join created another entry")
	}
	hub.handle(clients[4], Command{Type: "queue.cancel", RequestID: "cancel"})
	if clients[4].State != domain.Idle || hub.queue.Len() != 0 {
		t.Fatal("cancellation did not remove queue membership")
	}
	hub.join(clients[4], "again")
	hub.expire(time.Now().Add(3 * time.Minute))
	if clients[4].State != domain.Idle || hub.queue.Len() != 0 || len(hub.queued) != 0 {
		t.Fatal("timeout retained queue membership")
	}
	assertMembership(t, hub)
}

func TestNextRejectsOldRoomsAndAvoidsImmediateRematch(t *testing.T) {
	hub, clients := memoryHub(t, 3)
	a, b, c := clients[0], clients[1], clients[2]
	hub.join(a, "a")
	hub.join(b, "b")
	oldRoom := a.RoomID
	hub.handle(a, Command{Type: "chat.next", RequestID: "next-a", RoomID: oldRoom})
	hub.handle(b, Command{Type: "chat.next", RequestID: "next-b", RoomID: oldRoom})
	if b.State != domain.Idle || a.State != domain.Waiting {
		t.Fatal("simultaneous Next queued the former partner automatically")
	}
	hub.join(b, "find-b")
	if hub.queue.Len() != 2 || len(hub.rooms) != 0 {
		t.Fatal("former partners were immediately rematched")
	}
	hub.join(c, "c")
	newRoom := a.RoomID
	if newRoom == "" || newRoom == oldRoom || newRoom != c.RoomID {
		t.Fatal("new visitor did not match the oldest eligible visitor")
	}
	hub.handle(a, Command{Type: "chat.stop", RequestID: "stale", RoomID: oldRoom})
	if a.RoomID != newRoom || len(hub.rooms) != 1 {
		t.Fatal("stale room command ended a new conversation")
	}
	hub.remove(a, "disconnected")
	hub.remove(a, "disconnected")
	if c.State != domain.Idle || c.LastPartner != "" || len(hub.rooms) != 0 {
		t.Fatal("disconnect retained a room or last-partner reference")
	}
	assertMembership(t, hub)
}

func TestRelayDoesNotCrossRoomsOrForwardRequestIDs(t *testing.T) {
	hub, clients := memoryHub(t, 4)
	for _, c := range clients {
		hub.join(c, c.ID)
	}
	for _, c := range clients {
		drain(c)
	}
	room := clients[0].RoomID
	hub.handle(clients[0], Command{Type: "chat.send", RoomID: room, RequestID: "message", text: "Hello"})
	self, peer := <-clients[0].send, <-clients[1].send
	if self.RequestID != "message" || peer.RequestID != "" || peer.RoomID != room || len(clients[2].send) != 0 || len(clients[3].send) != 0 {
		t.Fatal("message crossed room or correlation boundaries")
	}
	if self.Payload.(MessagePayload).Sender != "self" || peer.Payload.(MessagePayload).Sender != "peer" || self.Payload.(MessagePayload).MessageID != peer.Payload.(MessagePayload).MessageID {
		t.Fatal("server echo metadata is incorrect")
	}
	hub.handle(clients[0], Command{Type: "queue.cancel", RequestID: "cancel"})
	if clients[0].RoomID != room {
		t.Fatal("cancel silently ended a matched room")
	}
	assertMembership(t, hub)
}

func TestFullBuffersDisconnectWithoutSendingAfterClose(t *testing.T) {
	hub, clients := memoryHub(t, 2)
	a, b := clients[0], clients[1]
	hub.join(a, "a")
	hub.join(b, "b")
	drain(a)
	drain(b)
	for range cap(b.send) {
		hub.emit(b, Event{Type: "filler"})
	}
	hub.emit(b, Event{Type: "overflow"})
	hub.emit(b, Event{Type: "after-close"})
	hub.remove(b, "again")
	if len(hub.clients) != 1 || len(hub.rooms) != 0 || a.State != domain.Idle || !b.closed() {
		t.Fatal("slow consumer cleanup failed")
	}
	hub.commands = make(chan hubCommand, 1)
	if !hub.submit(hubCommand{kind: "command", client: a}) || hub.submit(hubCommand{kind: "command", client: a}) || !a.closed() {
		t.Fatal("full command channel did not close its submitter")
	}
	assertMembership(t, hub)
}

func TestPerConnectionLimitsAndViolationWindow(t *testing.T) {
	_, clients := memoryHub(t, 1)
	c := clients[0]
	now := time.Now()
	for range 5 {
		if c.limit("chat.send", now) != 0 {
			t.Fatal("message burst was rejected early")
		}
	}
	if c.limit("chat.send", now) <= 0 {
		t.Fatal("message rate was not limited")
	}
	for range 4 {
		if c.violation(now) {
			t.Fatal("policy threshold triggered early")
		}
	}
	if !c.violation(now) || c.violation(now.Add(31*time.Second)) {
		t.Fatal("policy window or fifth-violation threshold is incorrect")
	}
}

func TestNextIntoFullQueueReturnsTheResultingIdleState(t *testing.T) {
	hub, clients := memoryHub(t, 3)
	hub.cfg.QueueCapacity = 1
	hub.join(clients[0], "a")
	hub.join(clients[1], "b")
	clients[2].LastPartner = clients[0].ID
	hub.join(clients[2], "waiting")
	room := clients[0].RoomID
	drain(clients[0])
	hub.handle(clients[0], Command{Type: "chat.next", RequestID: "next", RoomID: room})
	event := <-clients[0].send
	payload := event.Payload.(ErrorPayload)
	if event.Type != "error" || event.RoomID != room || event.RequestID != "next" || payload.Code != "queue_full" || payload.State != "idle" || clients[0].RoomID != "" {
		t.Fatal("queue rejection left the requester in the old room")
	}
	assertMembership(t, hub)
}
