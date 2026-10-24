package domain

type ChatState string

const (
	Idle    ChatState = "idle"
	Waiting ChatState = "waiting"
	Matched ChatState = "matched"
)

type Client struct {
	ID          string
	State       ChatState
	RoomID      string
	LastPartner string
}
