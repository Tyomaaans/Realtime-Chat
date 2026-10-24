package chat

import "errors"

var (
	ErrInvalidPayload = errors.New("invalid payload")
	ErrUnknownEvent   = errors.New("unknown event")
	ErrInvalidState   = errors.New("invalid state")
	ErrStaleRoom      = errors.New("stale room")
	ErrRateLimited    = errors.New("rate limited")
	ErrQueueFull      = errors.New("queue full")
	ErrServerBusy     = errors.New("server busy")
	ErrInternal       = errors.New("internal error")
)

type ErrorPayload struct {
	Code         string `json:"code"`
	Message      string `json:"message"`
	State        string `json:"state,omitempty"`
	RetryAfterMS int64  `json:"retry_after_ms,omitempty"`
}

func publicError(err error) ErrorPayload {
	switch {
	case errors.Is(err, ErrInvalidPayload):
		return ErrorPayload{Code: "invalid_payload", Message: "The command payload is invalid."}
	case errors.Is(err, ErrUnknownEvent):
		return ErrorPayload{Code: "unknown_event", Message: "This command is unsupported."}
	case errors.Is(err, ErrInvalidState):
		return ErrorPayload{Code: "invalid_state", Message: "This action is unavailable in the current state."}
	case errors.Is(err, ErrStaleRoom):
		return ErrorPayload{Code: "stale_room", Message: "This conversation has ended."}
	case errors.Is(err, ErrRateLimited):
		return ErrorPayload{Code: "rate_limited", Message: "Please slow down and try again."}
	case errors.Is(err, ErrQueueFull):
		return ErrorPayload{Code: "queue_full", Message: "The waiting queue is full. Please try again shortly."}
	case errors.Is(err, ErrServerBusy):
		return ErrorPayload{Code: "server_busy", Message: "The server is busy. Please try again shortly."}
	default:
		return ErrorPayload{Code: "internal_error", Message: "The server could not complete this action."}
	}
}
