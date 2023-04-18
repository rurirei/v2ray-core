package udp

import (
	"v2ray.com/core/common/buffer"
	"v2ray.com/core/common/net"
)

// Packet is a UDP packet together with its source and destination address.
type Packet struct {
	Payload        *buffer.Buffer
	Source, Target net.Address
}
