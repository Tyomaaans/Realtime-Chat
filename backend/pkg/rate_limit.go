package pkg

import (
	"math"
	"time"
)

type TokenBucket struct {
	tokens   float64
	capacity float64
	rate     float64
	updated  time.Time
}

func NewTokenBucket(capacity int, rate float64, now time.Time) TokenBucket {
	return TokenBucket{
		tokens:   float64(capacity),
		capacity: float64(capacity),
		rate:     rate,
		updated:  now,
	}
}

func (b *TokenBucket) Take(now time.Time) time.Duration {
	elapsed := math.Max(0, now.Sub(b.updated).Seconds())
	b.tokens = math.Min(b.capacity, b.tokens+elapsed*b.rate)
	b.updated = now
	if b.tokens < 1 {
		return time.Duration(math.Ceil((1-b.tokens)/b.rate*1000)) * time.Millisecond
	}

	b.tokens--
	return 0
}
