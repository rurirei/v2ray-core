package tun

import (
	"v2ray.com/core/common/net"
	"v2ray.com/core/transport/internet"
)

var (
	localTCPHub chan net.Conn
	localUDPHub chan net.Conn
)

type hub struct {
	ch chan net.Conn
}

func (h *hub) Receive() <-chan net.Conn {
	return h.ch
}

func (h *hub) Addr() net.Addr {
	return nil
}

func (h *hub) Close() error {
	return nil
}

func ListenUDP(_ net.Address) (internet.Hub, error) {
	return &hub{
		ch: localUDPHub,
	}, nil
}

func ListenTCP(_ net.Address) (internet.Listener, error) {
	return &hub{
		ch: localTCPHub,
	}, nil
}

func RegisterUDPHub(ch chan net.Conn) {
	localUDPHub = ch
}

func RegisterTCPHub(ch chan net.Conn) {
	localTCPHub = ch
}
