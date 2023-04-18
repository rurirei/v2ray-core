package loader

import (
	"v2ray.com/core/common/net"

	"github.com/cretz/bine/tor"
	// _ "github.com/cretz/tor-static"
)

var (
	localTor *tor.Tor
)

func BuildTorClient() (net.Dialer, error) {
	if err := buildTor(); err != nil {
		return nil, err
	}

	dialer, err := localTor.Dialer(nil, nil)
	if err != nil {
		return nil, err
	}

	return net.NewDialer(dialer.Dial, net.LocalLookupIPFunc), nil
}

func buildTor() error {
	t, err := tor.Start(nil, &tor.StartConf{
		// ProcessCreator:         embedded.NewCreator(),
		// UseEmbeddedControlConn: true,
	})
	if err != nil {
		return err
	}

	localTor = t

	return nil
}
