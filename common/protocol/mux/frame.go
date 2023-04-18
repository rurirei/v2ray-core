package mux

import (
	"encoding/binary"

	"v2ray.com/core/common"
	"v2ray.com/core/common/bitmask"
	"v2ray.com/core/common/buffer"
	"v2ray.com/core/common/io"
	"v2ray.com/core/common/net"
	"v2ray.com/core/common/protocol"
	"v2ray.com/core/common/serial"
)

var (
	errNotNew = newError("not new")
)

type sessionID = uint16

type sessionOption = bitmask.Byte

const (
	sessionOptionData  sessionOption = 0x01
	sessionOptionError sessionOption = 0x02
)

type sessionStatus = byte

const (
	sessionStatusNew       sessionStatus = 0x01
	sessionStatusKeep      sessionStatus = 0x02
	sessionStatusEnd       sessionStatus = 0x03
	sessionStatusKeepAlive sessionStatus = 0x04
)

type sessionTargetNetwork = byte

const (
	sessionTargetNetworkTCP sessionTargetNetwork = 0x01
	sessionTargetNetworkUDP sessionTargetNetwork = 0x02
)

var addrParser = protocol.NewAddressParser(
	protocol.AddressFamilyByte(byte(protocol.HostLengthIPv4), net.HostLengthIPv4),
	protocol.AddressFamilyByte(byte(protocol.HostLengthIPv6), net.HostLengthIPv6),
	protocol.AddressFamilyByte(byte(protocol.HostLengthDomain), net.HostLengthDomain),
	protocol.PortThenAddress(),
)

/*
Frame format
2 bytes - length
2 bytes - session id
1 bytes - status
1 bytes - option

1 byte - network
2 bytes - port
n bytes - address

*/

type frameMetadata struct {
	target net.Address
	id     sessionID
	option sessionOption
	status sessionStatus
}

func (f frameMetadata) WriteTo(b *buffer.Buffer) error {
	lenBytes := b.Extend(2)

	len0 := b.Len()
	sessionBytes := b.Extend(2)
	binary.BigEndian.PutUint16(sessionBytes, f.id)

	_ = b.WriteByte(f.status)
	_ = b.WriteByte(byte(f.option))

	switch f.status {
	case sessionStatusNew:
		switch f.target.Network {
		case net.Network_TCP:
			_ = b.WriteByte(sessionTargetNetworkTCP)
		case net.Network_UDP:
			_ = b.WriteByte(sessionTargetNetworkUDP)
		default:
			return common.ErrUnknownNetwork
		}

		if err := addrParser.WriteAddress(b, f.target); err != nil {
			return err
		}
	}

	len1 := b.Len()
	binary.BigEndian.PutUint16(lenBytes, uint16(len1-len0))
	return nil
}

// unmarshalFromReader reads buffer from the given reader.
func unmarshalFromReader(reader io.Reader) (frameMetadata, error) {
	metaLen, err := serial.ReadUint16(reader)
	if err != nil {
		return frameMetadata{}, err
	}
	if metaLen > 512 {
		return frameMetadata{}, newError("invalid metaLen %d", metaLen)
	}

	b := buffer.New()
	defer b.Release()

	if _, err := b.ReadFullFrom(reader, int(metaLen)); err != nil {
		return frameMetadata{}, err
	}

	return unmarshalFromBuffer(b)
}

// unmarshalFromBuffer reads a frameMetadata from the given buffer.
func unmarshalFromBuffer(b *buffer.Buffer) (frameMetadata, error) {
	if bLen := b.Len(); bLen < 4 {
		return frameMetadata{}, newError("insufficient buffer %d", bLen)
	}

	id := binary.BigEndian.Uint16(b.BytesTo(2))
	status := b.Byte(2)
	option := bitmask.Byte(b.Byte(3))

	switch status {
	case sessionStatusNew:
		if bLen := b.Len(); bLen < 8 {
			return frameMetadata{}, newError("insufficient buffer %d", bLen)
		}
		network := b.Byte(4)
		b.Advance(5)

		address, err := addrParser.ReadAddress(nil, b)
		if err != nil {
			return frameMetadata{}, newError("failed to parse address").WithError(err)
		}

		target, err := func() (net.Address, error) {
			switch network {
			case sessionTargetNetworkTCP:
				return net.AddressFromHostPort(net.Network_TCP, address), nil
			case sessionTargetNetworkUDP:
				return net.AddressFromHostPort(net.Network_UDP, address), nil
			default:
				return net.Address{}, common.ErrUnknownNetwork
			}
		}()
		if err != nil {
			return frameMetadata{}, err
		}

		return frameMetadata{
			target: target,
			id:     id,
			option: option,
			status: status,
		}, nil
	default:
		return frameMetadata{
			id:     id,
			option: option,
			status: status,
		}, errNotNew
	}
}
