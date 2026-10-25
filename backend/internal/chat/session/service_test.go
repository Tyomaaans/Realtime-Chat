package session

import (
	"errors"
	"strings"
	"testing"
)

func TestMessageValidationRunsBeforeHubSubmission(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		wantErr bool
	}{
		{
			name:    "empty",
			text:    "",
			wantErr: true,
		},
		{
			name: "ASCII limit",
			text: strings.Repeat("a", 2000),
		},
		{
			name: "Unicode limit",
			text: strings.Repeat("\u754c", 2000),
		},
		{
			name: "four-byte Unicode",
			text: strings.Repeat("\U0001F642", 2000),
		},
		{
			name:    "above character limit",
			text:    strings.Repeat("a", 2001),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hub, clients := memoryHub(t, 1)
			service := NewChatService(hub, hub.cfg)
			err := service.HandleCommand(clients[0], ChatCommandRequest{
				Type: "chat.send",
				text: tt.text,
			})
			if errors.Is(err, ErrInvalidPayload) != tt.wantErr {
				t.Fatalf("message validation error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr && len(hub.commands) != 0 {
				t.Fatal("invalid message reached the hub")
			}
		})
	}

	hub, clients := memoryHub(t, 1)
	cfg := hub.cfg
	cfg.MessageCharacters, cfg.MessageBytes = 4, 4
	if err := NewChatService(hub, cfg).HandleCommand(clients[0], ChatCommandRequest{
		Type: "chat.send",
		text: "\u754c\u754c",
	}); !errors.Is(err, ErrInvalidPayload) {
		t.Fatal("independent byte limit was not enforced")
	}
}
