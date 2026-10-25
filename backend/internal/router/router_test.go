package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"realtime-chat/internal/chat/session"
	"realtime-chat/internal/config"
)

func TestHealthAndReadinessFollowHubLifetime(t *testing.T) {
	cfg, _ := config.NewConfig()
	hub := session.NewHub(cfg)
	router, err := NewRouter(nil, hub, zerolog.Nop())
	if err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{"/health", "/ready"} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		want := 200
		if path == "/ready" {
			want = 503
		}

		if recorder.Code != want {
			t.Fatalf("%s status = %d, want %d", path, recorder.Code, want)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)
	check, stopCheck := context.WithTimeout(context.Background(), time.Second)
	defer stopCheck()
	if _, err := hub.Snapshot(check); err != nil {
		t.Fatal(err)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/ready", nil))
	if recorder.Code != http.StatusOK {
		t.Fatal("running hub is not ready")
	}

	hub.StopAdmission()
	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/ready", nil))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatal("readiness did not reject after admission stopped")
	}

	cancel()
	shutdown, stop := context.WithTimeout(context.Background(), time.Second)
	defer stop()
	if err := hub.Wait(shutdown); err != nil {
		t.Fatal(err)
	}
}
