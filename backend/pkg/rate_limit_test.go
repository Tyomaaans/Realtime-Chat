package pkg

import (
	"testing"
	"time"
)

func TestTokenBucketRefillAndRetry(t *testing.T) {
	now := time.Now()
	bucket := NewTokenBucket(2, 2, now)
	if bucket.Take(now) != 0 || bucket.Take(now) != 0 || bucket.Take(now) != 500*time.Millisecond {
		t.Fatal("burst or retry delay is incorrect")
	}

	if bucket.Take(now.Add(500*time.Millisecond)) != 0 || bucket.Take(now.Add(500*time.Millisecond)) != 500*time.Millisecond {
		t.Fatal("refill did not restore exactly one token")
	}
}
