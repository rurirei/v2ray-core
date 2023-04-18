package trojan

import (
	"encoding/binary"

	"v2ray.com/core/common/buffer"
	"v2ray.com/core/common/io"
	"v2ray.com/core/common/net"
	"v2ray.com/core/common/protocol"
	"v2ray.com/core/common/protocol/trojan"
	"v2ray.com/core/proxy/trojan/cipher"
)

var addrParser = protocol.NewAddressParser(
	protocol.AddressFamilyByte(0x01, net.HostLengthIPv4),
	protocol.AddressFamilyByte(0x04, net.HostLengthIPv6),
	protocol.AddressFamilyByte(0x03, net.HostLengthDomain),
)

var crlf = []byte{'\r', '\n'}

func WriteRequestHeader(writer io.Writer, address net.Address, user cipher.User) error {
	buf := buffer.New()
	defer buf.Release()

	command := trojan.RequestCommandFromNetwork(address.Network, false)

	if _, err := buf.Write(user.Key); err != nil {
		return err
	}
	if _, err := buf.Write(crlf); err != nil {
		return err
	}
	if err := buf.WriteByte(byte(command)); err != nil {
		return err
	}
	if err := addrParser.WriteAddress(buf, address); err != nil {
		return err
	}
	if _, err := buf.Write(crlf); err != nil {
		return err
	}

	_, err := buf.ReadToWriter(writer, true)
	return err
}

func ParseRequestHeader(reader io.Reader) (net.Address, error) {
	var crlf [2]byte
	var command [1]byte
	var hash [56]byte

	if _, err := io.ReadFull(reader, hash[:]); err != nil {
		return net.Address{}, newError("failed to read user hash").WithError(err)
	}

	if _, err := io.ReadFull(reader, crlf[:]); err != nil {
		return net.Address{}, newError("failed to read crlf").WithError(err)
	}

	if _, err := io.ReadFull(reader, command[:]); err != nil {
		return net.Address{}, newError("failed to read command").WithError(err)
	}

	network := net.Network_TCP
	if command[0] == byte(trojan.RequestCommandUDP) {
		network = net.Network_UDP
	}

	address, err := addrParser.ReadAddress(nil, reader)
	if err != nil {
		return net.Address{}, newError("failed to read address and port").WithError(err)
	}

	if _, err := io.ReadFull(reader, crlf[:]); err != nil {
		return net.Address{}, newError("failed to read crlf").WithError(err)
	}

	return net.AddressFromHostPort(net.Network(network), address), nil
}

type tcpWriter struct {
	io.Writer
}

func (w *tcpWriter) WriteMultiBuffer(mb buffer.MultiBuffer) error {
	for _, b := range mb {
		if !b.IsEmpty() {
			if _, err := b.ReadToWriter(w, true); err != nil {
				return err
			}

			b.Release()
		}
	}
	return nil
}

type udpWriter struct {
	io.Writer
}

func (w *udpWriter) WriteTo(payload []byte, address net.Address) (int, error) {
	buf := buffer.New()
	defer buf.Release()

	length := len(payload)
	lengthBuf := [2]byte{}
	binary.BigEndian.PutUint16(lengthBuf[:], uint16(length))
	if err := addrParser.WriteAddress(buf, address); err != nil {
		return 0, err
	}
	if _, err := buf.Write(lengthBuf[:]); err != nil {
		return 0, err
	}
	if _, err := buf.Write(crlf); err != nil {
		return 0, err
	}
	if _, err := buf.Write(payload); err != nil {
		return 0, err
	}

	if _, err := buf.ReadToWriter(w, true); err != nil {
		return 0, err
	}

	return length, nil
}

type udpReader struct {
	Reader io.Reader
}

func (r *udpReader) ReadMultiBuffer() (buffer.MultiBuffer, error) {
	if _, err := addrParser.ReadAddress(nil, r.Reader); err != nil {
		return nil, newError("failed to read address and port").WithError(err)
	}

	var lengthBuf [2]byte
	if _, err := io.ReadFull(r.Reader, lengthBuf[:]); err != nil {
		return nil, newError("failed to read payload length").WithError(err)
	}

	remain := int(binary.BigEndian.Uint16(lengthBuf[:]))
	if remain > trojan.MaxLength {
		return nil, newError("oversize payload")
	}

	var crlf [2]byte
	if _, err := io.ReadFull(r.Reader, crlf[:]); err != nil {
		return nil, newError("failed to read crlf").WithError(err)
	}

	var mb buffer.MultiBuffer

	for remain > 0 {
		length := buffer.Size
		if remain < length {
			length = remain
		}

		b := buffer.New()
		mb = append(mb, b)
		n, err := b.ReadFullFrom(r.Reader, length)
		if err != nil {
			buffer.ReleaseMulti(mb)
			return nil, newError("failed to read payload").WithError(err)
		}

		remain -= int(n)
	}

	return mb, nil
}
