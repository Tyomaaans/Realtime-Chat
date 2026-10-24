package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"realtime-chat/internal/chat"
	"realtime-chat/internal/middleware"
)

func NewRouter(chatHandler *chat.ChatHandler, hub *chat.Hub, logger zerolog.Logger) (*gin.Engine, error) {
	r := gin.New()
	if err := r.SetTrustedProxies(nil); err != nil {
		return nil, err
	}
	r.Use(middleware.RequestContext(logger))
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/ready", func(c *gin.Context) {
		if !hub.Ready() {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})
	r.GET("/v1/chat/ws", chatHandler.Connect)
	return r, nil
}
