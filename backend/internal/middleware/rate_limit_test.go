package middleware

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"realtime-chat/internal/config"
)

func TestBucketRefillAndRetry(t *testing.T) {
	now := time.Now()
	bucket := NewBucket(2, 2, now)
	if bucket.Take(now) != 0 || bucket.Take(now) != 0 || bucket.Take(now) != 500*time.Millisecond {
		t.Fatal("burst or retry delay is incorrect")
	}
	if bucket.Take(now.Add(500*time.Millisecond)) != 0 || bucket.Take(now.Add(500*time.Millisecond)) != 500*time.Millisecond {
		t.Fatal("refill did not restore exactly one token")
	}
}

func TestConcurrentReservationsRespectCapacityAndReleaseOnce(t *testing.T) {
	cfg, _ := config.NewConfig()
	cfg.MaxConnections, cfg.MaxConnectionsPerIP = 12, 12
	admission := NewAdmission(cfg)
	var wg sync.WaitGroup
	releases := make(chan func(), 100)
	start := make(chan struct{})
	for i := range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			release, status, _ := admission.Reserve(fmt.Sprintf("192.0.2.%d", i), time.Now())
			if status == 0 {
				releases <- release
			}
		}()
	}
	close(start)
	wg.Wait()
	close(releases)
	if len(releases) != 12 || admission.active != 12 {
		t.Fatalf("reserved = %d, active = %d", len(releases), admission.active)
	}
	for release := range releases {
		release()
		release()
	}
	if admission.active != 0 {
		t.Fatalf("active after release = %d", admission.active)
	}
}

func TestAdmissionLimitsTTLCapacityAndShutdown(t *testing.T) {
	cfg, _ := config.NewConfig()
	cfg.IPLimiterEntries, cfg.HandshakeBurst, cfg.HandshakesPerMinute = 2, 1, 1
	admission := NewAdmission(cfg)
	now := time.Now()
	release, status, _ := admission.Reserve("192.0.2.1", now)
	if status != 0 {
		t.Fatal("first reservation failed")
	}
	if _, status, retry := admission.Reserve("192.0.2.1", now); status != 429 || retry <= 0 {
		t.Fatal("handshake bucket was not enforced")
	}
	second, status, _ := admission.Reserve("192.0.2.2", now)
	if status != 0 {
		t.Fatal("second identity failed")
	}
	if _, status, _ := admission.Reserve("192.0.2.3", now); status != 429 {
		t.Fatal("limiter entry capacity was not enforced")
	}
	admission.Cleanup(now.Add(20 * time.Minute))
	if len(admission.entries) != 2 {
		t.Fatal("active quotas were evicted")
	}
	release()
	second()
	third, status, _ := admission.Reserve("192.0.2.3", now.Add(20*time.Minute))
	if status != 0 || len(admission.entries) != 1 {
		t.Fatal("expired entries were not removed before a new reservation")
	}
	third()
	admission.Close()
	if _, status, _ := admission.Reserve("192.0.2.4", now); status != 503 {
		t.Fatal("shutdown still accepts reservations")
	}
}

func TestDefaultPerIPQuota(t *testing.T) {
	cfg, _ := config.NewConfig()
	admission := NewAdmission(cfg)
	for range 5 {
		release, status, _ := admission.Reserve("127.0.0.1", time.Now())
		if status != 0 {
			t.Fatal("default capacity rejected an allowed connection")
		}
		defer release()
	}
	if _, status, _ := admission.Reserve("127.0.0.1", time.Now()); status != 429 {
		t.Fatal("sixth connection was accepted")
	}
}
