package middleware

import (
	"net/http"
	"sync"
	"time"

	"realtime-chat/internal/config"
	"realtime-chat/pkg"
)

type ipEntry struct {
	bucket   pkg.TokenBucket
	lastSeen time.Time
	active   int
}

type Admission struct {
	mu      sync.Mutex
	cfg     config.AppConfig
	entries map[string]*ipEntry
	active  int
	closed  bool
}

func NewAdmission(cfg config.AppConfig) *Admission {
	return &Admission{
		cfg:     cfg,
		entries: make(map[string]*ipEntry),
	}
}

func (a *Admission) Reserve(ip string, now time.Time) (func(), int, time.Duration) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed || a.active >= a.cfg.MaxConnections {
		return nil, http.StatusServiceUnavailable, time.Second
	}

	entry := a.entries[ip]
	if entry == nil {
		if len(a.entries) >= a.cfg.IPLimiterEntries {
			a.cleanup(now)
			if len(a.entries) >= a.cfg.IPLimiterEntries {
				return nil, http.StatusTooManyRequests, time.Minute
			}
		}

		entry = &ipEntry{bucket: pkg.NewTokenBucket(a.cfg.HandshakeBurst, float64(a.cfg.HandshakesPerMinute)/60, now)}
		a.entries[ip] = entry
	}

	entry.lastSeen = now
	if retry := entry.bucket.Take(now); retry > 0 {
		return nil, http.StatusTooManyRequests, retry
	}

	if entry.active >= a.cfg.MaxConnectionsPerIP {
		return nil, http.StatusTooManyRequests, time.Second
	}

	entry.active++
	a.active++
	var once sync.Once
	release := func() {
		once.Do(func() {
			a.mu.Lock()
			defer a.mu.Unlock()
			entry.active--
			entry.lastSeen = time.Now()
			a.active--
		})
	}
	return release, 0, 0
}

func (a *Admission) Cleanup(now time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.cleanup(now)
}

func (a *Admission) cleanup(now time.Time) {
	for ip, entry := range a.entries {
		if entry.active == 0 && now.Sub(entry.lastSeen) >= a.cfg.IPLimiterTTL {
			delete(a.entries, ip)
		}
	}
}

func (a *Admission) Close() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.closed = true
}
