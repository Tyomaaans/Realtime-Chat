package chat

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"unicode/utf8"
)

type Command struct {
	Type      string          `json:"type"`
	RequestID string          `json:"request_id"`
	RoomID    string          `json:"room_id,omitempty"`
	Payload   json.RawMessage `json:"payload"`
	text      string
	typing    bool
}

type Event struct {
	Type      string `json:"type"`
	RequestID string `json:"request_id,omitempty"`
	RoomID    string `json:"room_id,omitempty"`
	Payload   any    `json:"payload"`
	control   int
	data      []byte
}

type StatePayload struct {
	State     string `json:"state"`
	Reason    string `json:"reason,omitempty"`
	ExpiresAt string `json:"expires_at,omitempty"`
}

type MessagePayload struct {
	MessageID string `json:"message_id"`
	Text      string `json:"text"`
	Sender    string `json:"sender"`
	SentAt    string `json:"sent_at"`
}

type TypingPayload struct {
	Typing bool   `json:"typing"`
	Sender string `json:"sender"`
}

func parseCommand(data []byte) (Command, error) {
	var command Command
	if !utf8.Valid(data) || !uniqueJSONKeys(json.NewDecoder(bytes.NewReader(data))) {
		return command, ErrInvalidPayload
	}
	fields, err := decodeStrict(data, &command, "type", "request_id", "room_id", "payload")
	if err != nil {
		return Command{}, ErrInvalidPayload
	}
	if strings.TrimSpace(command.RequestID) == "" || utf8.RuneCountInString(command.RequestID) > 64 || len(command.Payload) == 0 || command.Payload[0] != '{' {
		return Command{}, ErrInvalidPayload
	}
	switch command.Type {
	case "queue.join", "queue.cancel":
		if _, exists := fields["room_id"]; exists {
			return command, ErrInvalidPayload
		}
		if _, err := decodeStrict(command.Payload, &struct{}{}); err != nil {
			return command, ErrInvalidPayload
		}
	case "chat.stop", "chat.next":
		_, err := decodeStrict(command.Payload, &struct{}{})
		if command.RoomID == "" || err != nil {
			return command, ErrInvalidPayload
		}
	case "chat.send":
		var payload struct {
			Text *string `json:"text"`
		}
		_, err := decodeStrict(command.Payload, &payload, "text")
		if command.RoomID == "" || err != nil || payload.Text == nil {
			return command, ErrInvalidPayload
		}
		command.text = strings.TrimSpace(*payload.Text)
	case "chat.typing":
		var payload struct {
			Typing *bool `json:"typing"`
		}
		_, err := decodeStrict(command.Payload, &payload, "typing")
		if command.RoomID == "" || err != nil || payload.Typing == nil {
			return command, ErrInvalidPayload
		}
		command.typing = *payload.Typing
	default:
		return command, ErrUnknownEvent
	}
	command.Payload = nil
	return command, nil
}

func decodeStrict(data []byte, target any, fields ...string) (map[string]json.RawMessage, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return nil, err
	}
	if object == nil {
		return nil, ErrInvalidPayload
	}
	for name, value := range object {
		if !slices.Contains(fields, name) || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return nil, ErrInvalidPayload
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return nil, err
	}
	return object, nil
}

func uniqueJSONKeys(decoder *json.Decoder) bool {
	token, err := decoder.Token()
	if err != nil {
		return false
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return true
	}
	switch delimiter {
	case '{':
		keys := make(map[string]bool)
		for decoder.More() {
			key, err := decoder.Token()
			name, ok := key.(string)
			if err != nil || !ok || keys[name] {
				return false
			}
			keys[name] = true
			if !uniqueJSONKeys(decoder) {
				return false
			}
		}
	case '[':
		for decoder.More() {
			if !uniqueJSONKeys(decoder) {
				return false
			}
		}
	default:
		return false
	}
	_, err = decoder.Token()
	return err == nil
}
