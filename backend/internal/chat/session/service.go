package session

import (
	"unicode/utf8"

	"realtime-chat/internal/config"
)

// Interface

type ChatService interface {
	HandleCommand(client *client, command ChatCommandRequest) error
}

// Implementation

type chatService struct {
	hub *Hub
	cfg config.AppConfig
}

func NewChatService(hub *Hub, cfg config.AppConfig) ChatService {
	return &chatService{
		hub: hub,
		cfg: cfg,
	}
}

func (s *chatService) HandleCommand(client *client, command ChatCommandRequest) error {
	if command.Type == "chat.send" {
		characters := utf8.RuneCountInString(command.text)
		messageBytes := len(command.text)
		if characters == 0 || characters > s.cfg.MessageCharacters || messageBytes > s.cfg.MessageBytes {
			return ErrInvalidPayload
		}
	}

	if !s.hub.submit(hubCommand{
		kind:    "command",
		client:  client,
		command: command,
	}) {
		return ErrServerBusy
	}

	return nil
}
