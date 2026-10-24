package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type AppConfig struct {
	Address             string
	Origins             []string
	MaxConnections      int
	MaxConnectionsPerIP int
	QueueCapacity       int
	CommandBuffer       int
	OutboundBuffer      int
	FrameBytes          int
	MessageCharacters   int
	MessageBytes        int
	PingInterval        time.Duration
	PongWait            time.Duration
	WriteWait           time.Duration
	HandshakeTimeout    time.Duration
	ReadHeaderTimeout   time.Duration
	QueueWait           time.Duration
	ShutdownTimeout     time.Duration
	HandshakeBurst      int
	HandshakesPerMinute int
	IPLimiterTTL        time.Duration
	IPLimiterEntries    int
}

func NewConfig() (AppConfig, error) {
	cfg := AppConfig{
		Address:             "127.0.0.1:8080",
		Origins:             []string{"http://localhost:5173"},
		MaxConnections:      500,
		MaxConnectionsPerIP: 5,
		QueueCapacity:       500,
		CommandBuffer:       256,
		OutboundBuffer:      32,
		FrameBytes:          8192,
		MessageCharacters:   2000,
		MessageBytes:        8000,
		PingInterval:        25 * time.Second,
		PongWait:            60 * time.Second,
		WriteWait:           10 * time.Second,
		HandshakeTimeout:    5 * time.Second,
		ReadHeaderTimeout:   5 * time.Second,
		QueueWait:           120 * time.Second,
		ShutdownTimeout:     10 * time.Second,
		HandshakeBurst:      10,
		HandshakesPerMinute: 10,
		IPLimiterTTL:        10 * time.Minute,
		IPLimiterEntries:    10000,
	}
	if value, ok := os.LookupEnv("CHAT_ADDRESS"); ok {
		cfg.Address = value
	}
	if value, ok := os.LookupEnv("CHAT_ORIGINS"); ok {
		cfg.Origins = strings.Split(value, ",")
	}
	integers := []struct {
		name  string
		value *int
	}{
		{"MAX_CONNECTIONS", &cfg.MaxConnections},
		{"MAX_CONNECTIONS_PER_IP", &cfg.MaxConnectionsPerIP},
		{"QUEUE_CAPACITY", &cfg.QueueCapacity},
		{"COMMAND_BUFFER", &cfg.CommandBuffer},
		{"OUTBOUND_BUFFER", &cfg.OutboundBuffer},
		{"FRAME_BYTES", &cfg.FrameBytes},
		{"MESSAGE_CHARACTERS", &cfg.MessageCharacters},
		{"MESSAGE_BYTES", &cfg.MessageBytes},
		{"HANDSHAKE_BURST", &cfg.HandshakeBurst},
		{"HANDSHAKES_PER_MINUTE", &cfg.HandshakesPerMinute},
		{"IP_LIMITER_ENTRIES", &cfg.IPLimiterEntries},
	}
	for _, setting := range integers {
		if value, ok := os.LookupEnv("CHAT_" + setting.name); ok {
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed <= 0 || parsed > 1000000 {
				return AppConfig{}, fmt.Errorf("CHAT_%s must be between 1 and 1000000", setting.name)
			}
			*setting.value = parsed
		}
	}
	durations := []struct {
		name  string
		value *time.Duration
	}{
		{"PING_INTERVAL", &cfg.PingInterval},
		{"PONG_WAIT", &cfg.PongWait},
		{"WRITE_WAIT", &cfg.WriteWait},
		{"HANDSHAKE_TIMEOUT", &cfg.HandshakeTimeout},
		{"READ_HEADER_TIMEOUT", &cfg.ReadHeaderTimeout},
		{"QUEUE_WAIT", &cfg.QueueWait},
		{"SHUTDOWN_TIMEOUT", &cfg.ShutdownTimeout},
		{"IP_LIMITER_TTL", &cfg.IPLimiterTTL},
	}
	for _, setting := range durations {
		if value, ok := os.LookupEnv("CHAT_" + setting.name); ok {
			parsed, err := time.ParseDuration(value)
			if err != nil || parsed <= 0 || parsed > 24*time.Hour {
				return AppConfig{}, fmt.Errorf("CHAT_%s must be a positive duration up to 24h", setting.name)
			}
			*setting.value = parsed
		}
	}
	if err := cfg.Validate(); err != nil {
		return AppConfig{}, err
	}
	return cfg, nil
}

func (c AppConfig) Validate() error {
	host, port, err := net.SplitHostPort(c.Address)
	ip := net.ParseIP(host)
	portNumber, portErr := strconv.Atoi(port)
	if err != nil || ip == nil || !ip.IsLoopback() || portErr != nil || portNumber < 1 || portNumber > 65535 {
		return fmt.Errorf("CHAT_ADDRESS must contain a loopback IP and a port from 1 to 65535")
	}
	if c.QueueCapacity > c.MaxConnections || c.MaxConnectionsPerIP > c.MaxConnections {
		return fmt.Errorf("queue and per-IP capacities cannot exceed connection capacity")
	}
	if c.MessageBytes > c.FrameBytes || c.MessageCharacters > c.MessageBytes || c.FrameBytes > 1<<20 {
		return fmt.Errorf("message limits must fit the frame limit; frames cannot exceed 1 MiB")
	}
	if c.PingInterval >= c.PongWait || c.ShutdownTimeout > 10*time.Second {
		return fmt.Errorf("ping must be shorter than pong wait; shutdown cannot exceed 10s")
	}
	for _, origin := range c.Origins {
		parsed, err := url.Parse(origin)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || strings.Contains(origin, "*") {
			return fmt.Errorf("CHAT_ORIGINS must contain exact origins without paths or wildcards")
		}
		if origin != parsed.Scheme+"://"+parsed.Host || parsed.Hostname() == "" || strings.HasSuffix(parsed.Host, ":") {
			return fmt.Errorf("CHAT_ORIGINS must contain exact scheme, host, and optional port")
		}
		if parsed.Port() != "" {
			port, err := strconv.Atoi(parsed.Port())
			if err != nil || port < 1 || port > 65535 {
				return fmt.Errorf("CHAT_ORIGINS ports must be between 1 and 65535")
			}
		}
	}
	return nil
}
