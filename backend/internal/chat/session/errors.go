package session

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
