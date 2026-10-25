package session

import (
	"errors"
	"strings"
	"testing"
)

func TestParseCommandRejectsInvalidEnvelopes(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{
			name: "missing request",
			data: `{"type":"queue.join","payload":{}}`,
		},
		{
			name: "long request",
			data: `{"type":"queue.join","request_id":"` + strings.Repeat("a", 65) + `","payload":{}}`,
		},
		{
			name: "unknown field",
			data: `{"type":"queue.join","request_id":"c1","sender":"forged","payload":{}}`,
		},
		{
			name: "duplicate key",
			data: `{"type":"queue.join","type":"queue.cancel","request_id":"c1","payload":{}}`,
		},
		{
			name: "nested duplicate",
			data: `{"type":"chat.send","room_id":"room","request_id":"c1","payload":{"text":"one","text":"two"}}`,
		},
		{
			name: "multiple values",
			data: `{"type":"queue.join","request_id":"c1","payload":{}} {}`,
		},
		{
			name: "null payload",
			data: `{"type":"queue.join","request_id":"c1","payload":null}`,
		},
		{
			name: "null text",
			data: `{"type":"chat.send","room_id":"room","request_id":"c1","payload":{"text":null}}`,
		},
		{
			name: "numeric text",
			data: `{"type":"chat.send","room_id":"room","request_id":"c1","payload":{"text":7}}`,
		},
		{
			name: "missing room",
			data: `{"type":"chat.stop","request_id":"c1","payload":{}}`,
		},
		{
			name: "room on join",
			data: `{"type":"queue.join","room_id":"room","request_id":"c1","payload":{}}`,
		},
		{
			name: "empty room on join",
			data: `{"type":"queue.join","room_id":"","request_id":"c1","payload":{}}`,
		},
		{
			name: "null room on join",
			data: `{"type":"queue.join","room_id":null,"request_id":"c1","payload":{}}`,
		},
		{
			name: "uppercase envelope",
			data: `{"TYPE":"queue.join","request_id":"c1","payload":{}}`,
		},
		{
			name: "uppercase payload",
			data: `{"type":"chat.send","room_id":"room","request_id":"c1","payload":{"Text":"Hello"}}`,
		},
		{
			name: "draft on typing",
			data: `{"type":"chat.typing","room_id":"room","request_id":"c1","payload":{"typing":true,"draft":"secret"}}`,
		},
		{
			name: "null typing",
			data: `{"type":"chat.typing","room_id":"room","request_id":"c1","payload":{"typing":null}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ParseChatCommand([]byte(tt.data)); !errors.Is(err, ErrInvalidPayload) {
				t.Fatalf("parse error = %v, want invalid payload", err)
			}
		})
	}

	if _, err := ParseChatCommand([]byte(`{"type":"unknown","request_id":"c1","payload":{}}`)); !errors.Is(err, ErrUnknownEvent) {
		t.Fatalf("unknown event error = %v", err)
	}

	command, err := ParseChatCommand([]byte(`{"type":"chat.send","room_id":"room","request_id":"c1","payload":{"text":"  Hello \u4e16\u754c  "}}`))
	if err != nil || command.text != "Hello \u4e16\u754c" {
		t.Fatalf("Unicode normalization = %q, error = %v", command.text, err)
	}
}
