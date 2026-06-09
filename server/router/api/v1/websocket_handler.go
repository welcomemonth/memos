package v1

import (
	"github.com/labstack/echo/v5"
	"github.com/usememos/memos/internal/webrtc"
)

type websocketRouteRegistrar interface {
	GET(path string, h echo.HandlerFunc, m ...echo.MiddlewareFunc) echo.RouteInfo
}

func RegisterWebRTCSignalRoute(router websocketRouteRegistrar, hub *webrtc.Hub) {
	router.GET("/api/v1/webrtc/signal", func(c *echo.Context) error {
		return hub.HandleSignaling(c)
	})
}
