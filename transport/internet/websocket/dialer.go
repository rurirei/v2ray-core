package websocket

import (
	gotls "crypto/tls"
	"net/http"
	"time"

	"v2ray.com/core/common/net"
	"v2ray.com/core/transport/internet"
	"v2ray.com/core/transport/internet/tls"

	"github.com/gorilla/websocket"
)

const (
	protocolName    = "ws"
	protocolNameTLS = "wss"
)

type DialSetting struct {
	Path      string
	TLSConfig tls.Config
}

func Dial(setting DialSetting, dialTCPFunc internet.DialTCPFunc) internet.DialTCPFunc {
	return func(src, dst net.Address) (net.Conn, error) {
		uri := func() string {
			name := func() string {
				if len(setting.TLSConfig.ServerName) > 0 {
					return protocolNameTLS
				}
				return protocolName
			}()

			return name + "://" + dst.DomainPreferredAddress() + setting.Path
		}()

		dialer := &websocket.Dialer{
			NetDial: func(network, address string) (net.Conn, error) {
				dst, err := net.ParseAddress(network, address)
				if err != nil {
					return nil, err
				}

				return dialTCPFunc(src, dst)
			},
			TLSClientConfig: func() *gotls.Config {
				if len(setting.TLSConfig.ServerName) > 0 {
					return setting.TLSConfig.BuildTLSClient()
				}
				return nil
			}(),
			ReadBufferSize:   4 * 1024,
			WriteBufferSize:  4 * 1024,
			HandshakeTimeout: 8 * time.Second,
		}

		conn, _, err := dialer.Dial(uri, http.Header{
			"Host": []string{setting.TLSConfig.ServerName},
		})
		if err != nil {
			return nil, err
		}

		return &httpConn{
			conn:       conn,
			remoteAddr: conn.RemoteAddr(),
		}, nil
	}
}
