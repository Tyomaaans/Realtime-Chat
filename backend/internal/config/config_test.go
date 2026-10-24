package config

import "testing"

func TestNewConfigValidatesLocalSettings(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
	}{
		{name: "public address", key: "CHAT_ADDRESS", value: "0.0.0.0:8080"},
		{name: "invalid port", key: "CHAT_ADDRESS", value: "127.0.0.1:0"},
		{name: "empty origin", key: "CHAT_ORIGINS", value: ""},
		{name: "wildcard origin", key: "CHAT_ORIGINS", value: "http://*.localhost:5173"},
		{name: "origin path", key: "CHAT_ORIGINS", value: "http://localhost:5173/"},
		{name: "origin query marker", key: "CHAT_ORIGINS", value: "http://localhost:5173?"},
		{name: "origin invalid port", key: "CHAT_ORIGINS", value: "http://localhost:99999"},
		{name: "empty integer", key: "CHAT_MAX_CONNECTIONS", value: ""},
		{name: "negative buffer", key: "CHAT_COMMAND_BUFFER", value: "-1"},
		{name: "queue above capacity", key: "CHAT_QUEUE_CAPACITY", value: "501"},
		{name: "invalid duration", key: "CHAT_WRITE_WAIT", value: "ten"},
		{name: "ping after deadline", key: "CHAT_PING_INTERVAL", value: "60s"},
		{name: "long shutdown", key: "CHAT_SHUTDOWN_TIMEOUT", value: "11s"},
		{name: "text exceeds frame", key: "CHAT_FRAME_BYTES", value: "1000"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(tt.key, tt.value)
			if _, err := NewConfig(); err == nil {
				t.Fatal("invalid configuration was accepted")
			}
		})
	}
	cfg, err := NewConfig()
	if err != nil || cfg.Address != "127.0.0.1:8080" || cfg.MaxConnections != 500 {
		t.Fatalf("default configuration = %#v, error = %v", cfg, err)
	}
}
