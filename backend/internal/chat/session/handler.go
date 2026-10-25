package session

import (
	"math"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"realtime-chat/internal/config"
	"realtime-chat/internal/middleware"
	"realtime-chat/pkg"
)

type ChatHandler struct {
	chatSvc   ChatService
	hub       *Hub
	admission *middleware.Admission
	cfg       config.AppConfig
	upgrader  websocket.Upgrader
}

func NewChatHandler(
	chatSvc ChatService,
	hub *Hub,
	admission *middleware.Admission,
	cfg config.AppConfig,
) *ChatHandler {
	handler := &ChatHandler{
		chatSvc:   chatSvc,
		hub:       hub,
		admission: admission,
		cfg:       cfg,
	}
	handler.upgrader = websocket.Upgrader{
		HandshakeTimeout:  cfg.HandshakeTimeout,
		ReadBufferSize:    1024,
		WriteBufferSize:   1024,
		EnableCompression: false,
		CheckOrigin:       handler.allowedOrigin,
	}
	return handler
}

func (h *ChatHandler) Connect(c *gin.Context) {
	if !h.allowedOrigin(c.Request) {
		c.AbortWithStatusJSON(http.StatusForbidden, ToErrorResponse(ErrInvalidPayload))
		return
	}

	if !h.hub.Ready() {
		c.Header("Retry-After", "1")
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, ToErrorResponse(ErrServerBusy))
		return
	}

	ip, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, ToErrorResponse(ErrInvalidPayload))
		return
	}

	release, status, retry := h.admission.Reserve(ip, time.Now())
	if status != 0 {
		c.Header("Retry-After", strconv.Itoa(int(math.Ceil(retry.Seconds()))))
		err := ErrRateLimited
		if status == http.StatusServiceUnavailable {
			err = ErrServerBusy
		}

		c.AbortWithStatusJSON(status, ToErrorResponse(err))
		return
	}

	id, err := pkg.NewID()
	if err != nil {
		release()
		c.AbortWithStatusJSON(http.StatusInternalServerError, ToErrorResponse(ErrInternal))
		return
	}

	c.Status(http.StatusSwitchingProtocols)
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		release()
		return
	}

	client := newClient(id, conn, h.hub, h.chatSvc, h.cfg, release)
	if err := h.hub.register(client); err != nil {
		_ = conn.Close()
		release()
		return
	}
}

func (h *ChatHandler) allowedOrigin(request *http.Request) bool {
	values := request.Header.Values("Origin")
	if len(values) != 1 {
		return false
	}

	for _, origin := range h.cfg.Origins {
		if values[0] == origin {
			return true
		}
	}

	return false
}
