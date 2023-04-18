package socks

import (
	"v2ray.com/core/common"
	"v2ray.com/core/common/buffer"
	"v2ray.com/core/common/io"
	"v2ray.com/core/common/net"
	"v2ray.com/core/common/protocol"
	"v2ray.com/core/common/protocol/socks"
)

const (
	socks5Version socks.Version = 0x05
	socks4Version socks.Version = 0x04

	cmdTCPConnect    = 0x01
	cmdTCPBind       = 0x02
	cmdUDPAssociate  = 0x03
	cmdTorResolve    = 0xF0
	cmdTorResolvePTR = 0xF1

	socks4RequestGranted  = 90
	socks4RequestRejected = 91

	authNotRequired = 0x00
	// authGssAPI           = 0x01
	authPassword         = 0x02
	authNoMatchingMethod = 0xFF

	statusSuccess       = 0x00
	statusCmdNotSupport = 0x07
)

var addrParser = protocol.NewAddressParser(
	protocol.AddressFamilyByte(0x01, net.HostLengthIPv4),
	protocol.AddressFamilyByte(0x04, net.HostLengthIPv6),
	protocol.AddressFamilyByte(0x03, net.HostLengthDomain),
)

type ServerSession struct {
	Address ServerAddress
}

type ServerAddress struct {
	Listen, Client, Conf net.Address
}

// Handshake performs a Socks5 handshake.
func (s *ServerSession) Handshake(writer io.Writer, reader io.Reader) (protocol.RequestHeader, error) {
	buf := buffer.New()
	if _, err := buf.ReadFullFrom(reader, 2); err != nil {
		buf.Release()
		return protocol.RequestHeader{}, newError("insufficient header").WithError(err)
	}

	version := buf.Byte(0)
	cmd := buf.Byte(1)
	buf.Release()

	switch socks.Version(version) {
	case socks5Version:
		return s.handshake5(cmd, writer, reader)
	default:
		return protocol.RequestHeader{}, common.ErrUnknownNetwork
	}
}

func (s *ServerSession) handshake5(nMethod byte, writer io.Writer, reader io.Reader) (protocol.RequestHeader, error) {
	request := protocol.RequestHeader{}

	if _, err := s.auth5(nMethod, writer, reader); err != nil {
		return protocol.RequestHeader{}, err
	}

	buf := buffer.New()
	defer buf.Release()

	if _, err := buf.ReadFullFrom(reader, 3); err != nil {
		return protocol.RequestHeader{}, newError("failed to read request").WithError(err)
	}

	cmd := buf.Byte(1)

	switch cmd {
	case cmdTCPConnect, cmdTorResolve, cmdTorResolvePTR:
		// We don't have a solution for Tor case now. Simply treat it as connect command.
		request.Command.Socks = socks.RequestCommandTCP
	case cmdUDPAssociate:
		request.Command.Socks = socks.RequestCommandUDP
	case cmdTCPBind:
		err := writeSocks5Response(writer, statusCmdNotSupport, net.AnyUDPAddress)
		return protocol.RequestHeader{}, newError("TCP bind is not supported.").WithError(err)
	default:
		err := writeSocks5Response(writer, statusCmdNotSupport, net.AnyUDPAddress)
		return protocol.RequestHeader{}, newError("unknown command %d", cmd).WithError(err)
	}

	request.Version.Socks = socks5Version

	address, err := addrParser.ReadAddress(nil, reader)
	if err != nil {
		return protocol.RequestHeader{}, newError("failed to read address").WithError(err)
	}
	request.Address = protocol.RequestAddress{
		Address: address,
	}

	responseAddress, err := func() (net.Address, error) {
		switch request.Command.Socks {
		case socks.RequestCommandTCP:
			return s.Address.Listen, nil
		case socks.RequestCommandUDP:
			if s.Address.Conf.IsOneHost() {
				// use configured IP as remote address in the response to UdpAssociate
				return s.Address.Conf, nil
			}
			if s.Address.Client.IP.Equal(net.LocalhostIPv4) || s.Address.Client.IP.Equal(net.LocalhostIPv6) {
				// For localhost clients use loopback IP
				return s.Address.Client, nil
			}
			// For non-localhost clients use inbound listening address
			return s.Address.Listen, nil
		default:
			return net.Address{}, common.ErrUnknownNetwork
		}
	}()
	if err != nil {
		return protocol.RequestHeader{}, err
	}

	if err := writeSocks5Response(writer, statusSuccess, responseAddress); err != nil {
		return protocol.RequestHeader{}, err
	}

	return request, nil
}

func (s *ServerSession) auth5(nMethod byte, writer io.Writer, reader io.Reader) (string, error) {
	buf := buffer.New()
	defer buf.Release()

	if _, err := buf.ReadFullFrom(reader, int(nMethod)); err != nil {
		return "", newError("failed to read auth methods").WithError(err)
	}

	var expectedAuth byte = authNotRequired
	/*if s.config.AuthType == AuthType_PASSWORD {
		expectedAuth = authPassword
	}*/

	if !hasAuthMethod(expectedAuth, buf.BytesRange(0, int(nMethod))) {
		err := writeSocks5AuthenticationResponse(writer, socks5Version, authNoMatchingMethod)
		return "", newError("no matching auth method").WithError(err)
	}

	if err := writeSocks5AuthenticationResponse(writer, socks5Version, expectedAuth); err != nil {
		return "", newError("failed to write auth response").WithError(err)
	}

	/*if expectedAuth == authPassword {
		username, password, err := ReadUsernamePassword(reader)
		if err != nil {
			return "", newError("failed to read username and password for authentication").WithError(err)
		}

		if !s.config.HasAccount(username, password) {
			writeSocks5AuthenticationResponse(writer, 0x01, 0xFF)
			return "", newError("invalid username or password")
		}

		if err := writeSocks5AuthenticationResponse(writer, 0x01, 0x00); err != nil {
			return "", newError("failed to write auth response").Base(err)
		}
		return username, nil
	}*/

	return "", nil
}

func writeSocks5Response(writer io.Writer, errCode byte, address net.Address) error {
	buf := buffer.New()
	defer buf.Release()

	if _, err := buf.Write([]byte{byte(socks5Version), errCode, 0x00 /* reserved */}); err != nil {
		return err
	}

	if err := addrParser.WriteAddress(buf, address); err != nil {
		return err
	}

	return buffer.WriteAllBytes(writer, buf.Bytes())
}

func writeSocks5AuthenticationResponse(writer io.Writer, version socks.Version, auth byte) error {
	return buffer.WriteAllBytes(writer, []byte{byte(version), auth})
}

func hasAuthMethod(expectedAuth byte, authCandidates []byte) bool {
	for _, a := range authCandidates {
		if a == expectedAuth {
			return true
		}
	}
	return false
}

// ReadUsernamePassword reads Socks 5 username/password message from the given reader.
// +----+------+----------+------+----------+
// |VER | ULEN |  UNAME   | PLEN |  PASSWD  |
// +----+------+----------+------+----------+
// | 1  |  1   | 1 to 255 |  1   | 1 to 255 |
// +----+------+----------+------+----------+
func ReadUsernamePassword(reader io.Reader) (string, string, error) {
	buf := buffer.New()
	defer buf.Release()

	if _, err := buf.ReadFullFrom(reader, 2); err != nil {
		return "", "", err
	}
	nUsername := buf.Byte(1)

	buf.Clear()
	if _, err := buf.ReadFullFrom(reader, int(nUsername)); err != nil {
		return "", "", err
	}
	username := buf.String()

	buf.Clear()
	if _, err := buf.ReadFullFrom(reader, 1); err != nil {
		return "", "", err
	}
	nPassword := buf.Byte(0)

	buf.Clear()
	if _, err := buf.ReadFullFrom(reader, int(nPassword)); err != nil {
		return "", "", err
	}
	password := buf.String()
	return username, password, nil
}

type udpReader struct {
	Reader io.Reader
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

	request, err := DecodeUDPPacket(buf)
	if err != nil {
		return net.Address{}, err
	}

	return request.Address.AsAddress(net.Network_UDP), nil
}

type udpWriter struct {
	Request protocol.RequestHeader
	Writer  io.WriteCloser
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

func (w *udpWriter) Close() error {
	return w.Writer.Close()
}

func DecodeUDPPacket(payload *buffer.Buffer) (protocol.RequestHeader, error) {
	if payload.Len() < 5 {
		return protocol.RequestHeader{}, newError("insufficient length of packet.")
	}
	request := protocol.RequestHeader{
		Command: protocol.RequestCommand{
			Socks: socks.RequestCommandUDP,
		},
		Version: protocol.RequestVersion{
			Socks: socks5Version,
		},
	}

	// packet[0] and packet[1] are reserved
	if payload.Byte(2) != 0 /* fragments */ {
		return protocol.RequestHeader{}, newError("discarding fragmented payload.")
	}

	payload.Advance(3)

	address, err := addrParser.ReadAddress(nil, payload)
	if err != nil {
		return protocol.RequestHeader{}, newError("failed to read UDP header").WithError(err)
	}
	request.Address = protocol.RequestAddress{
		Address: address,
	}
	return request, nil
}

func EncodeUDPPacket(request protocol.RequestHeader, data []byte) (*buffer.Buffer, error) {
	buf := buffer.New()

	if _, err := buf.Write([]byte{0, 0, 0 /* Fragment */}); err != nil {
		defer buf.Release()
		return nil, err
	}

	if err := addrParser.WriteAddress(buf, request.Address.Address); err != nil {
		defer buf.Release()
		return nil, err
	}

	if _, err := buf.Write(data); err != nil {
		defer buf.Release()
		return nil, err
	}

	return buf, nil
}

func ClientHandshake(request protocol.RequestHeader, writer io.Writer, reader io.Reader) (protocol.RequestHeader, error) {
	authByte := byte(authNotRequired)
	/*if request.User != nil {
		authByte = byte(authPassword)
	}*/

	buf := buffer.New()
	defer buf.Release()

	if _, err := buf.Write([]byte{byte(socks5Version), 0x01, authByte}); err != nil {
		return protocol.RequestHeader{}, err
	}
	/*if authByte == authPassword {
		account := request.User.Account.(*Account)

		common.Must(buf.WriteByte(0x01))
		common.Must(buf.WriteByte(byte(len(account.Username))))
		common.Must2(buf.WriteString(account.Username))
		common.Must(buf.WriteByte(byte(len(account.Password))))
		common.Must2(buf.WriteString(account.Password))
	}*/

	if err := buffer.WriteAllBytes(writer, buf.Bytes()); err != nil {
		return protocol.RequestHeader{}, err
	}

	buf.Clear()
	if _, err := buf.ReadFullFrom(reader, 2); err != nil {
		return protocol.RequestHeader{}, err
	}

	if buf.Byte(0) != byte(socks5Version) {
		return protocol.RequestHeader{}, newError("unexpected server version %d", buf.Byte(0))
	}
	if buf.Byte(1) != authByte {
		return protocol.RequestHeader{}, newError("auth method not supported.")
	}

	if authByte == authPassword {
		buf.Clear()
		if _, err := buf.ReadFullFrom(reader, 2); err != nil {
			return protocol.RequestHeader{}, err
		}
		if buf.Byte(1) != 0x00 {
			return protocol.RequestHeader{}, newError("server rejects account %d", buf.Byte(1))
		}
	}

	buf.Clear()

	command := byte(cmdTCPConnect)
	if request.Command.Socks == socks.RequestCommandUDP {
		command = byte(cmdUDPAssociate)
	}
	if _, err := buf.Write([]byte{byte(socks5Version), command, 0x00 /* reserved */}); err != nil {
		return protocol.RequestHeader{}, err
	}
	if err := addrParser.WriteAddress(buf, request.Address.Address); err != nil {
		return protocol.RequestHeader{}, err
	}

	if err := buffer.WriteAllBytes(writer, buf.Bytes()); err != nil {
		return protocol.RequestHeader{}, err
	}

	buf.Clear()
	if _, err := buf.ReadFullFrom(reader, 3); err != nil {
		return protocol.RequestHeader{}, err
	}

	resp := buf.Byte(1)
	if resp != 0x00 {
		return protocol.RequestHeader{}, newError("server rejects request %d", resp)
	}

	buf.Clear()

	address, err := addrParser.ReadAddress(buf, reader)
	if err != nil {
		return protocol.RequestHeader{}, err
	}

	if request.Command.Socks == socks.RequestCommandUDP {
		udpRequest := protocol.RequestHeader{
			Command: protocol.RequestCommand{
				Socks: socks.RequestCommandUDP,
			},
			Version: protocol.RequestVersion{
				Socks: socks5Version,
			},
			Address: protocol.RequestAddress{
				Address: address,
			},
		}
		return udpRequest, nil
	}

	return protocol.RequestHeader{}, nil
}
