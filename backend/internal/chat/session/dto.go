package session

import "encoding/json"

type ChatCommandRequest struct {
	Type      string          `json:"type"`
	RequestID string          `json:"request_id"`
	RoomID    string          `json:"room_id,omitempty"`
	Payload   json.RawMessage `json:"payload"`
	text      string
	typing    bool
}

type ChatEventResponse struct {
	Type      string `json:"type"`
	RequestID string `json:"request_id,omitempty"`
	RoomID    string `json:"room_id,omitempty"`
	Payload   any    `json:"payload"`
	control   int
	data      []byte
}

type StateResponse struct {
	State     string `json:"state"`
	Reason    string `json:"reason,omitempty"`
	ExpiresAt string `json:"expires_at,omitempty"`
}

type MessageResponse struct {
	MessageID string `json:"message_id"`
	Text      string `json:"text"`
	Sender    string `json:"sender"`
	SentAt    string `json:"sent_at"`
}

type TypingResponse struct {
	Typing bool   `json:"typing"`
	Sender string `json:"sender"`
}

type ErrorResponse struct {
	Code         string `json:"code"`
	Message      string `json:"message"`
	State        string `json:"state,omitempty"`
	RetryAfterMS int64  `json:"retry_after_ms,omitempty"`
}
