package shadowsocks

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"hash/crc32"

	"v2ray.com/core/common/buffer"
	"v2ray.com/core/common/drain"
	"v2ray.com/core/common/io"
	"v2ray.com/core/common/net"
	"v2ray.com/core/common/protocol"
	"v2ray.com/core/common/protocol/shadowsocks"
	"v2ray.com/core/proxy/shadowsocks/cipher"
)

var addrParser = protocol.NewAddressParser(
	protocol.AddressFamilyByte(0x01, net.HostLengthIPv4),
	protocol.AddressFamilyByte(0x04, net.HostLengthIPv6),
	protocol.AddressFamilyByte(0x03, net.HostLengthDomain),
	protocol.WithAddressTypeParser(func(b byte) byte {
		return b & 0x0F
	}),
)

// ReadTCPSession reads a Shadowsocks TCP session from the given reader, returns its header and remaining parts.
func ReadTCPSession(user shadowsocks.User, reader buffer.BufferedReader) (protocol.RequestHeader, buffer.Reader, error) {
	user2, err := cipher.BuildUser(user)
	if err != nil {
		return protocol.RequestHeader{}, nil, err
	}

	hashkdf := hmac.New(sha256.New, []byte("SSBSKDF"))
	hashkdf.Write(user2.Key)

	behaviorSeed := crc32.ChecksumIEEE(hashkdf.Sum(nil))

	drainer, err := drain.NewBehaviorSeedLimitedDrainer(int64(behaviorSeed), 16+38, 3266, 64)
	if err != nil {
		return protocol.RequestHeader{}, nil, newError("failed to initialize drainer").WithError(err)
	}

	buf := buffer.New()
	defer buf.Release()

	ivLen := int(user2.Cipher.IVSize())
	var iv []byte
	if ivLen > 0 {
		if _, err = buf.ReadFullFrom(reader, ivLen); err != nil {
			drainer.AcknowledgeReceive(buf.Len())
			return protocol.RequestHeader{}, nil, drain.WithError(drainer, reader, newError("failed to read IV").WithError(err))
		}

		iv = append([]byte{}, buf.BytesTo(ivLen)...)
	}

	r, err := user2.Cipher.NewDecryptionReader(user2.Key, iv, reader)
	if err != nil {
		drainer.AcknowledgeReceive(buf.Len())
		return protocol.RequestHeader{}, nil, drain.WithError(drainer, reader, newError("failed to initialize decoding stream").WithError(err))
	}
	br := buffer.NewBufferedReader(r)

	request := protocol.RequestHeader{
		Command: protocol.RequestCommand{
			Shadowsocks: shadowsocks.RequestCommandTCP,
		},
		Version: protocol.RequestVersion{
			Shadowsocks: shadowsocks.VersionName,
		},
		User: protocol.RequestUser{
			Shadowsocks: user,
		},
	}

	drainer.AcknowledgeReceive(buf.Len())
	buf.Clear()

	address, err := addrParser.ReadAddress(buf, br)
	if err != nil {
		drainer.AcknowledgeReceive(buf.Len())
		return protocol.RequestHeader{}, nil, drain.WithError(drainer, reader, newError("failed to read address").WithError(err))
	}
	request.Address = protocol.RequestAddress{
		Address: address,
	}

	if ivError := user2.User.CheckIV(iv); ivError != nil {
		drainer.AcknowledgeReceive(buf.Len())
		return protocol.RequestHeader{}, nil, drain.WithError(drainer, reader, newError("failed iv check").WithError(ivError))
	}

	return request, br, nil
}

// WriteTCPRequest writes Shadowsocks request into the given writer, and returns a writer for body.
func WriteTCPRequest(request protocol.RequestHeader, writer buffer.BufferedWriter) (buffer.Writer, error) {
	user2, err := cipher.BuildUser(request.User.Shadowsocks)
	if err != nil {
		return nil, err
	}

	var iv []byte
	if user2.Cipher.IVSize() > 0 {
		iv = make([]byte, user2.Cipher.IVSize())
		_, _ = rand.Read(iv)
		if ivError := user2.User.CheckIV(iv); ivError != nil {
			return nil, newError("failed to mark outgoing iv").WithError(ivError)
		}
		if err := buffer.WriteAllBytes(writer, iv); err != nil {
			return nil, newError("failed to write IV")
		}
	}

	w, err := user2.Cipher.NewEncryptionWriter(user2.Key, iv, writer)
	if err != nil {
		return nil, newError("failed to create encoding stream").WithError(err)
	}

	header := buffer.New()

	if err := addrParser.WriteAddress(header, request.Address.Address); err != nil {
		return nil, newError("failed to write address").WithError(err)
	}

	if err := w.WriteMultiBuffer(buffer.MultiBuffer{header}); err != nil {
		return nil, newError("failed to write header").WithError(err)
	}

	return w, nil
}

func ReadTCPResponse(user shadowsocks.User, reader buffer.BufferedReader) (buffer.Reader, error) {
	user2, err := cipher.BuildUser(user)
	if err != nil {
		return nil, err
	}

	hashkdf := hmac.New(sha256.New, []byte("SSBSKDF"))
	hashkdf.Write(user2.Key)

	behaviorSeed := crc32.ChecksumIEEE(hashkdf.Sum(nil))

	drainer, err := drain.NewBehaviorSeedLimitedDrainer(int64(behaviorSeed), 16+38, 3266, 64)
	if err != nil {
		return nil, newError("failed to initialize drainer").WithError(err)
	}

	var iv []byte
	if user2.Cipher.IVSize() > 0 {
		iv = make([]byte, user2.Cipher.IVSize())
		if n, err := io.ReadFull(reader, iv); err != nil {
			return nil, newError("failed to read IV").WithError(err)
		} else { // nolint: golint
			drainer.AcknowledgeReceive(n)
		}
	}

	if ivError := user2.User.CheckIV(iv); ivError != nil {
		return nil, drain.WithError(drainer, reader, newError("failed iv check").WithError(ivError))
	}

	return user2.Cipher.NewDecryptionReader(user2.Key, iv, reader)
}

func WriteTCPResponse(request protocol.RequestHeader, writer buffer.BufferedWriter) (buffer.Writer, error) {
	user2, err := cipher.BuildUser(request.User.Shadowsocks)
	if err != nil {
		return nil, err
	}

	var iv []byte
	if user2.Cipher.IVSize() > 0 {
		iv = make([]byte, user2.Cipher.IVSize())
		_, _ = rand.Read(iv)
		if ivError := user2.User.CheckIV(iv); ivError != nil {
			return nil, newError("failed to mark outgoing iv").WithError(ivError)
		}
		if err := buffer.WriteAllBytes(writer, iv); err != nil {
			return nil, newError("failed to write IV.").WithError(err)
		}
	}

	return user2.Cipher.NewEncryptionWriter(user2.Key, iv, writer)
}

func EncodeUDPPacket(request protocol.RequestHeader, payload []byte) (*buffer.Buffer, error) {
	buf := buffer.New()

	user2, err := cipher.BuildUser(request.User.Shadowsocks)
	if err != nil {
		defer buf.Release()
		return nil, err
	}

	ivLen := int(user2.Cipher.IVSize())
	if ivLen > 0 {
		_, _ = buf.ReadFullFrom(rand.Reader, ivLen)
	}

	if err := addrParser.WriteAddress(buf, request.Address.Address); err != nil {
		defer buf.Release()
		return nil, newError("failed to write address").WithError(err)
	}

	_, _ = buf.Write(payload)

	if err := user2.Cipher.EncodePacket(user2.Key, buf); err != nil {
		defer buf.Release()
		return nil, newError("failed to encrypt UDP payload").WithError(err)
	}

	return buf, nil
}

func DecodeUDPPacket(user shadowsocks.User, payload *buffer.Buffer) (protocol.RequestHeader, error) {
	user2, err := cipher.BuildUser(user)
	if err != nil {
		return protocol.RequestHeader{}, err
	}

	var iv []byte
	if !user2.Cipher.IsAEAD() && user2.Cipher.IVSize() > 0 {
		// Keep track of IV as it gets removed from payload in DecodePacket.
		iv = make([]byte, user2.Cipher.IVSize())
		copy(iv, payload.BytesTo(int(user2.Cipher.IVSize())))
	}

	if err := user2.Cipher.DecodePacket(user2.Key, payload); err != nil {
		return protocol.RequestHeader{}, newError("failed to decrypt UDP payload").WithError(err)
	}

	request := protocol.RequestHeader{
		Command: protocol.RequestCommand{
			Shadowsocks: shadowsocks.RequestCommandUDP,
		},
		Version: protocol.RequestVersion{
			Shadowsocks: shadowsocks.VersionName,
		},
		User: protocol.RequestUser{
			Shadowsocks: user,
		},
	}

	payload.SetByte(0, payload.Byte(0)&0x0F)

	address, err := addrParser.ReadAddress(nil, payload)
	if err != nil {
		return protocol.RequestHeader{}, newError("failed to parse address").WithError(err)
	}

	request.Address = protocol.RequestAddress{
		Address: address,
	}

	return request, nil
}

type udpReader struct {
	Reader io.Reader
	User   shadowsocks.User
}

func (r *udpReader) ReadMultiBuffer() (buffer.MultiBuffer, error) {
	buf := buffer.New()

	if _, err := r.readFrom(buf); err != nil {
		defer buf.Release()
		return nil, err
	}

	return buffer.MultiBuffer{buf}, nil
}

func (r *udpReader) readFrom(buf *buffer.Buffer) (net.Address, error) {
	if _, err := buf.ReadFrom(r.Reader); err != nil {
		return net.Address{}, err
	}

	request, err := DecodeUDPPacket(r.User, buf)
	if err != nil {
		return net.Address{}, err
	}

	return request.Address.AsAddress(net.Network_UDP), nil
}

type udpWriter struct {
	Writer  io.Writer
	Request protocol.RequestHeader
}

func (w *udpWriter) Write(payload []byte) (int, error) {
	return w.write(w.Request, payload)
}

func (w *udpWriter) WriteTo(payload []byte, address net.Address) (int, error) {
	request, err := func() (protocol.RequestHeader, error) {
		request := w.Request

		request.Address = protocol.RequestAddress{
			Address: address,
		}

		return request, nil
	}()
	if err != nil {
		return 0, err
	}

	return w.write(request, payload)
}

func (w *udpWriter) write(request protocol.RequestHeader, payload []byte) (int, error) {
	packet, err := EncodeUDPPacket(request, payload)
	if err != nil {
		return 0, err
	}
	defer packet.Release()

	return packet.ReadToWriter(w.Writer, true)
}
