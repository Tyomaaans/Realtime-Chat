package middleware

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

func RequestContext(logger zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error().Str("event", "request_panic").Msg("request failed")
				if !c.Writer.Written() {
					c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"code": "internal_error", "message": "The server could not complete this request."})
				}
			}
			logger.Info().Str("event", "request_completed").Int("status", c.Writer.Status()).Dur("duration_ms", time.Since(started)).Msg("request completed")
		}()
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("Cache-Control", "no-store")
		c.Next()
	}
}
