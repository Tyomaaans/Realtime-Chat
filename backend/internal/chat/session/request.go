package session

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"unicode/utf8"
)

func ParseChatCommand(data []byte) (ChatCommandRequest, error) {
	var command ChatCommandRequest
	if !utf8.Valid(data) || !uniqueJSONKeys(json.NewDecoder(bytes.NewReader(data))) {
		return command, ErrInvalidPayload
	}

	fields, err := decodeStrict(data, &command, "type", "request_id", "room_id", "payload")
	if err != nil {
		return ChatCommandRequest{}, ErrInvalidPayload
	}

	if strings.TrimSpace(command.RequestID) == "" || utf8.RuneCountInString(command.RequestID) > 64 {
		return ChatCommandRequest{}, ErrInvalidPayload
	}

	if len(command.Payload) == 0 || command.Payload[0] != '{' {
		return ChatCommandRequest{}, ErrInvalidPayload
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
