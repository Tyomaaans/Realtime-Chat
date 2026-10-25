package session

import (
	"errors"
	"time"
)

func ToErrorResponse(err error) ErrorResponse {
	switch {
	case errors.Is(err, ErrInvalidPayload):
		return ErrorResponse{
			Code:    "invalid_payload",
			Message: "The command payload is invalid.",
		}
	case errors.Is(err, ErrUnknownEvent):
		return ErrorResponse{
			Code:    "unknown_event",
			Message: "This command is unsupported.",
		}
	case errors.Is(err, ErrInvalidState):
		return ErrorResponse{
			Code:    "invalid_state",
			Message: "This action is unavailable in the current state.",
		}
	case errors.Is(err, ErrStaleRoom):
		return ErrorResponse{
			Code:    "stale_room",
			Message: "This conversation has ended.",
		}
	case errors.Is(err, ErrRateLimited):
		return ErrorResponse{
			Code:    "rate_limited",
			Message: "Please slow down and try again.",
		}
	case errors.Is(err, ErrQueueFull):
		return ErrorResponse{
			Code:    "queue_full",
			Message: "The waiting queue is full. Please try again shortly.",
		}
	case errors.Is(err, ErrServerBusy):
		return ErrorResponse{
			Code:    "server_busy",
			Message: "The server is busy. Please try again shortly.",
		}
	default:
		return ErrorResponse{
			Code:    "internal_error",
			Message: "The server could not complete this action.",
		}
	}
}

func ToWaitingResponse(requestID string, expiresAt time.Time) ChatEventResponse {
	return ChatEventResponse{
		Type:      "queue.waiting",
		RequestID: requestID,
		Payload: StateResponse{
			State:     "waiting",
			ExpiresAt: expiresAt.UTC().Format(time.RFC3339Nano),
		},
	}
}
