package session

import (
	"time"

	"realtime-chat/internal/chat/domain"
	"realtime-chat/pkg"
)

func (h *Hub) handle(c *client, command ChatCommandRequest) {
	if command.Type == "queue.join" {
		if c.State == domain.Matched {
			h.reject(c, command, ErrInvalidState)
			return
		}

		if err := h.join(c, command.RequestID); err != nil {
			h.reject(c, command, err)
		}

		return
	}

	if command.Type == "queue.cancel" {
		if c.State != domain.Waiting {
			h.reject(c, command, ErrInvalidState)
			return
		}

		h.dequeue(c)
		h.emit(c, ChatEventResponse{
			Type:      "queue.cancelled",
			RequestID: command.RequestID,
			Payload: StateResponse{
				State:  "idle",
				Reason: "cancelled",
			},
		})
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
			if err := h.join(c, command.RequestID); err != nil {
				h.reject(c, command, err)
			}

			return
		}

		h.emit(c, ChatEventResponse{
			Type:      "peer.left",
			RequestID: command.RequestID,
			RoomID:    command.RoomID,
			Payload: StateResponse{
				State:  "idle",
				Reason: reason,
			},
		})
	case "chat.send":
		id, err := pkg.NewID()
		if err != nil {
			h.reject(c, command, ErrInternal)
			return
		}

		payload := MessageResponse{
			MessageID: id,
			Text:      command.text,
			Sender:    "peer",
			SentAt:    time.Now().UTC().Format(time.RFC3339Nano),
		}
		h.emit(peer, ChatEventResponse{
			Type:    "chat.message",
			RoomID:  command.RoomID,
			Payload: payload,
		})
		payload.Sender = "self"
		h.emit(c, ChatEventResponse{
			Type:      "chat.message",
			RequestID: command.RequestID,
			RoomID:    command.RoomID,
			Payload:   payload,
		})
	case "chat.typing":
		h.emit(peer, ChatEventResponse{
			Type:   "chat.typing",
			RoomID: command.RoomID,
			Payload: TypingResponse{
				Typing: command.typing,
				Sender: "peer",
			},
		})
		h.emit(c, ChatEventResponse{
			Type:      "chat.typing",
			RequestID: command.RequestID,
			RoomID:    command.RoomID,
			Payload: TypingResponse{
				Typing: command.typing,
				Sender: "self",
			},
		})
	}
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
			h.emit(peer, ChatEventResponse{
				Type:   "peer.left",
				RoomID: room.ID,
				Payload: StateResponse{
					State:  "idle",
					Reason: reason,
				},
			})
		}
	}
}
