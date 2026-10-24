package chat

import (
	"unicode/utf8"

	"realtime-chat/internal/config"
)

// Interface

type ChatService interface {
	HandleCommand(client *client, command Command) error
}

// Implementation

type chatService struct {
	hub *Hub
	cfg config.AppConfig
}

func NewChatService(hub *Hub, cfg config.AppConfig) ChatService {
	return &chatService{hub: hub, cfg: cfg}
}

func (s *chatService) HandleCommand(client *client, command Command) error {
	if command.Type == "chat.send" && (command.text == "" || utf8.RuneCountInString(command.text) > s.cfg.MessageCharacters || len(command.text) > s.cfg.MessageBytes) {
		return ErrInvalidPayload
	}

	if !s.hub.submit(hubCommand{kind: "command", client: client, command: command}) {
		return ErrServerBusy
	}

	return nil
}
