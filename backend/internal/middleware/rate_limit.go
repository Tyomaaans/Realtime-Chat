package middleware

import (
	"math"
	"sync"
	"time"

	"realtime-chat/internal/config"
)

type Bucket struct {
	tokens   float64
	capacity float64
	rate     float64
	updated  time.Time
}

func NewBucket(capacity int, rate float64, now time.Time) Bucket {
	return Bucket{tokens: float64(capacity), capacity: float64(capacity), rate: rate, updated: now}
}

func (b *Bucket) Take(now time.Time) time.Duration {
	elapsed := math.Max(0, now.Sub(b.updated).Seconds())
	b.tokens = math.Min(b.capacity, b.tokens+elapsed*b.rate)
	b.updated = now
	if b.tokens < 1 {
		return time.Duration(math.Ceil((1-b.tokens)/b.rate*1000)) * time.Millisecond
	}
	b.tokens--
	return 0
}

type ipEntry struct {
	bucket   Bucket
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
	return &Admission{cfg: cfg, entries: make(map[string]*ipEntry)}
}

func (a *Admission) Reserve(ip string, now time.Time) (func(), int, time.Duration) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed || a.active >= a.cfg.MaxConnections {
		return nil, 503, time.Second
	}
	entry := a.entries[ip]
	if entry == nil {
		if len(a.entries) >= a.cfg.IPLimiterEntries {
			a.cleanup(now)
			if len(a.entries) >= a.cfg.IPLimiterEntries {
				return nil, 429, time.Minute
			}
		}
		entry = &ipEntry{bucket: NewBucket(a.cfg.HandshakeBurst, float64(a.cfg.HandshakesPerMinute)/60, now)}
		a.entries[ip] = entry
	}
	entry.lastSeen = now
	if retry := entry.bucket.Take(now); retry > 0 {
		return nil, 429, retry
	}
	if entry.active >= a.cfg.MaxConnectionsPerIP {
		return nil, 429, time.Second
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
