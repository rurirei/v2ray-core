package websocket

import (
	"net/http"
	"time"

	"v2ray.com/core/common/net"
	http_proto "v2ray.com/core/common/protocol/http"
	"v2ray.com/core/transport/internet"

	"github.com/gorilla/websocket"
)

type httpServer struct {
	server  *http.Server
	handler *httpHandler
}

func (l *httpServer) Receive() <-chan net.Conn {
	return l.handler.ch
}

func (l *httpServer) Addr() net.Addr {
	return nil
}

func (l *httpServer) Close() error {
	close(l.handler.ch)

	return l.server.Close()
}

type httpHandler struct {
	ch chan net.Conn

	path     string
	upgrader *websocket.Upgrader
}

func (h *httpHandler) ServeHTTP(responseWriter http.ResponseWriter, request *http.Request) {
	handle := func() error {
		if path := request.URL.Path; path != h.path {
			responseWriter.WriteHeader(http.StatusNotFound)
			return newError("path not found %s", path)
		}

		conn, err := h.upgrader.Upgrade(responseWriter, request, nil)
		if err != nil {
			return newError("failed to convert websocket httpConn").WithError(err)
		}

		forwardedAddr, err := net.ParseHost(http_proto.ParseXForwardedFor(request.Header)[0])
		if err != nil {
			return err
		}

		remoteAddr := conn.RemoteAddr()
		if forwardedAddr.IsIPHost() {
			remoteAddr = &net.TCPAddr{
				IP: forwardedAddr.IP,
			}
		}

		select {
		case h.ch <- &httpConn{
			conn:       conn,
			remoteAddr: remoteAddr,
		}:
		}

		return nil
	}

	if err := handle(); err != nil {
		newError("failed to handle").WithError(err).AtDebug().Logging()
	}
}

type ListenSetting struct {
	Path string
}

func Listen(setting ListenSetting, listenerFunc internet.ListenerFunc) internet.ListenerFunc {
	return func(address net.Address) (internet.Listener, error) {
		listener, err := listenerFunc(address)
		if err != nil {
			return nil, err
		}

		handler := &httpHandler{
			ch:   make(chan net.Conn),
			path: setting.Path,
			upgrader: &websocket.Upgrader{
				ReadBufferSize:   4 * 1024,
				WriteBufferSize:  4 * 1024,
				HandshakeTimeout: 4 * time.Second,
				CheckOrigin: func(r *http.Request) bool {
					return true
				},
			},
		}

		s := &httpServer{
			server: &http.Server{
				Handler:           handler,
				ReadHeaderTimeout: 4 * time.Second,
				MaxHeaderBytes:    http.DefaultMaxHeaderBytes,
			},
			handler: handler,
		}

		go func() {
			if err := s.server.Serve(listener.(net.Listener)); err != nil {
				newError("failed to serve").WithError(err).AtDebug().Logging()
			}
		}()

		return s, nil
	}
}
