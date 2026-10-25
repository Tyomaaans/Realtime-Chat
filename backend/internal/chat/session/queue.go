package session

import (
	"time"

	"realtime-chat/internal/chat/domain"
	"realtime-chat/pkg"
)

func (h *Hub) join(c *client, requestID string) error {
	if entry := h.queued[c.ID]; entry != nil {
		waiting := entry.Value.(waitEntry)
		h.emit(c, ToWaitingResponse(requestID, waiting.expiresAt))
		return nil
	}

	for element := h.queue.Front(); element != nil; element = element.Next() {
		waiting := element.Value.(waitEntry)
		peer := waiting.client
		if peer.ID == c.ID || peer.LastPartner == c.ID || c.LastPartner == peer.ID || peer.closed() {
			continue
		}

		id, err := pkg.NewID()
		if err != nil {
			return ErrInternal
		}

		h.dequeue(peer)
		h.rooms[id] = domain.RoomEntity{
			ID:      id,
			Members: [2]string{peer.ID, c.ID},
		}
		c.State, peer.State = domain.Matched, domain.Matched
		c.RoomID, peer.RoomID = id, id
		c.LastPartner, peer.LastPartner = peer.ID, c.ID
		h.emit(peer, ChatEventResponse{
			Type:      "chat.matched",
			RoomID:    id,
			RequestID: waiting.requestID,
			Payload:   StateResponse{State: "matched"},
		})
		if c.State == domain.Matched {
			h.emit(c, ChatEventResponse{
				Type:      "chat.matched",
				RoomID:    id,
				RequestID: requestID,
				Payload:   StateResponse{State: "matched"},
			})
		} else if h.clients[c.ID] == c {
			return h.join(c, requestID)
		}

		return nil
	}

	if h.queue.Len() >= h.cfg.QueueCapacity {
		return ErrQueueFull
	}

	expiresAt := time.Now().Add(h.cfg.QueueWait)
	c.State = domain.Waiting
	h.queued[c.ID] = h.queue.PushBack(waitEntry{
		client:    c,
		requestID: requestID,
		expiresAt: expiresAt,
	})
	h.emit(c, ToWaitingResponse(requestID, expiresAt))
	return nil
}

func (h *Hub) dequeue(c *client) {
	if element := h.queued[c.ID]; element != nil {
		h.queue.Remove(element)
		delete(h.queued, c.ID)
	}

	c.State = domain.Idle
}

func (h *Hub) expire(now time.Time) {
	for element := h.queue.Front(); element != nil; {
		next := element.Next()
		waiting := element.Value.(waitEntry)
		if !now.Before(waiting.expiresAt) {
			h.dequeue(waiting.client)
			h.emit(waiting.client, ChatEventResponse{
				Type: "queue.cancelled",
				Payload: StateResponse{
					State:  "idle",
					Reason: "queue_timeout",
				},
			})
		}

		element = next
	}
}
