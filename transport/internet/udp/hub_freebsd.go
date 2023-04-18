//go:build freebsd

package udp

import (
	"bytes"
	"encoding/gob"

	"v2ray.com/core/common/io"
	"v2ray.com/core/common/net"
)

// RetrieveOriginalDest from stored laddr, caddr
func RetrieveOriginalDest(_ []byte) (net.Address, error) {
	/*dec := gob.NewDecoder(bytes.NewBuffer(oob))
	var la, ra net.UDPAddr
	dec.Decode(&la)
	dec.Decode(&ra)
	ip, port, err := internet.OriginalDst(&la, &ra)
	if err != nil {
		return net.Address{}, err
	}
	return net.ParseAddress(net.Network_UDP,net.JoinHostPort(net.IP(ip).String(), net.Port(port).String()))*/

	return net.Address{}, newError("not supported")
}

// ReadUDPMsg stores laddr, caddr for later use
func ReadUDPMsg(conn *net.GoUDPConn, payload []byte, oob []byte) (int, int, int, *net.UDPAddr, error) {
	nBytes, addr, err := conn.ReadFromUDP(payload)
	if err != nil {
		return nBytes, 0, 0, addr, err
	}

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	enc.Encode(conn.LocalAddr().(*net.UDPAddr))
	enc.Encode(addr)

	var reader io.Reader = &buf
	noob, _ := reader.Read(oob)

	return nBytes, noob, 0, addr, err
}
