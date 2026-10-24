package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"realtime-chat/internal/chat"
	"realtime-chat/internal/config"
	"realtime-chat/internal/middleware"
	"realtime-chat/internal/router"
)

func main() {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, logger); err != nil {
		logger.Error().Err(err).Str("event", "application_stopped").Msg("application failed")
		os.Exit(1)
	}
}

func run(ctx context.Context, logger zerolog.Logger) error {
	cfg, err := config.NewConfig()
	if err != nil {
		return err
	}

	gin.SetMode(gin.ReleaseMode)
	hub := chat.NewHub(cfg)
	admission := middleware.NewAdmission(cfg)
	chatSvc := chat.NewChatService(hub, cfg)
	chatHandler := chat.NewChatHandler(chatSvc, hub, admission, cfg)
	r, err := router.NewRouter(chatHandler, hub, logger)
	if err != nil {
		return err
	}

	hubContext, cancelHub := context.WithCancel(context.Background())
	defer cancelHub()
	go hub.Run(hubContext)

	monitorDone := make(chan struct{})
	go func() {
		defer close(monitorDone)
		monitor(hubContext, hub, admission, logger)
	}()

	server := &http.Server{
		Addr:              cfg.Address,
		Handler:           r,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}
	serverErrors := make(chan error, 1)
	go func() { serverErrors <- server.ListenAndServe() }()
	logger.Info().Str("event", "application_started").Msg("local HTTP server started")

	select {
	case <-ctx.Done():
	case err = <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
	}

	admission.Close()
	hub.StopAdmission()
	cancelHub()

	shutdown, cancelShutdown := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancelShutdown()
	shutdownErr := server.Shutdown(shutdown)
	if shutdownErr != nil {
		_ = server.Close()
	}
	if waitErr := hub.Wait(shutdown); waitErr != nil {
		return waitErr
	}

	<-monitorDone
	if err != nil {
		return err
	}
	if shutdownErr != nil {
		return shutdownErr
	}

	logger.Info().Str("event", "application_stopped").Msg("connections closed")
	return nil
}

func monitor(ctx context.Context, hub *chat.Hub, admission *middleware.Admission, logger zerolog.Logger) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			admission.Cleanup(now)
			check, cancel := context.WithTimeout(ctx, time.Second)
			snapshot, err := hub.Snapshot(check)
			cancel()
			if err != nil {
				continue
			}
			var memory runtime.MemStats
			runtime.ReadMemStats(&memory)
			logger.Info().Str("event", "hub_status").Int("connections", snapshot.Connections).
				Int("waiting", snapshot.Waiting).Int("rooms", snapshot.Rooms).
				Int("goroutines", runtime.NumGoroutine()).Uint64("heap_bytes", memory.HeapAlloc).Msg("hub status")
		}
	}
}
