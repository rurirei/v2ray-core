package quic

import (
	"crypto"
	"crypto/aes"
	"crypto/tls"
	"encoding/binary"

	"golang.org/x/crypto/hkdf"

	"v2ray.com/core/common/buffer"
	"v2ray.com/core/common/bytespool"
	"v2ray.com/core/common/net"
	"v2ray.com/core/common/protocol/sniffer"
	tls_proto "v2ray.com/core/common/protocol/tls"

	"github.com/quic-go/quic-go/quicvarint"
)

const (
	versionDraft29 uint32 = 0xff00001d
	version1       uint32 = 0x1
)

var (
	quicSaltOld = []byte{0xaf, 0xbf, 0xec, 0x28, 0x99, 0x93, 0xd2, 0x4c, 0x9e, 0x97, 0x86, 0xf1, 0x9c, 0x61, 0x11, 0xe0, 0x43, 0x90, 0xa8, 0x99}
	quicSalt    = []byte{0x38, 0x76, 0x2c, 0xf7, 0xf5, 0x59, 0x34, 0xb3, 0x4d, 0x17, 0x9a, 0xe6, 0xa4, 0xc8, 0x0c, 0xad, 0xcc, 0xbb, 0x7f, 0x0a}

	initialSuite = &CipherSuiteTLS13{
		ID:     tls.TLS_AES_128_GCM_SHA256,
		KeyLen: 16,
		AEAD:   AEADAESGCMTLS13,
		Hash:   crypto.SHA256,
	}

	errNotQuic        = newError("not quic")
	errNotQuicInitial = newError("not initial packet")
)

type quicSniffer struct {
}

func NewSniffer() sniffer.Sniffer {
	return &quicSniffer{}
}

func (s *quicSniffer) Protocol() sniffer.Protocol {
	return sniffer.QUIC
}

func (s *quicSniffer) Sniff(b *buffer.Buffer, _ net.IP) (sniffer.SniffResult, error) {
	frameData, err := getFrameData(b.Bytes())
	if err != nil {
		return sniffer.SniffResult{}, err
	}

	domain, err := tls_proto.ReadClientHello(frameData)
	if err != nil {
		return sniffer.SniffResult{}, err
	}

	return sniffer.SniffResult{
		Protocol: s.Protocol(),
		Domain:   domain,
	}, nil
}

func getFrameData(b []byte) ([]byte, error) {
	buf := buffer.FromBytes(b)

	bytes, err := buf.ReadByte()
	if err != nil {
		return nil, err
	}

	isLongHeader := bytes&0x80 > 0
	if !isLongHeader || bytes&0x40 == 0 {
		return nil, errNotQuicInitial
	}

	vb, err := buf.ReadBytes(4)
	if err != nil {
		return nil, err
	}
	versionNumber := binary.BigEndian.Uint32(vb)
	if versionNumber != 0 && bytes&0x40 == 0 {
		return nil, errNotQuic
	}
	if versionNumber != versionDraft29 && versionNumber != version1 {
		return nil, errNotQuic
	}

	if (bytes&0x30)>>4 != 0x0 {
		return nil, errNotQuicInitial
	}

	idLen, err := buf.ReadByte()
	if err != nil {
		return nil, err
	}
	connID, err := buf.ReadBytes(int(idLen))
	if err != nil {
		return nil, err
	}

	if _, err = buf.ReadByte(); err != nil {
		return nil, err
	}
	if _, err = buf.ReadBytes(int(idLen)); err != nil {
		return nil, err
	}

	tokenLen, err := quicvarint.Read(buf)
	if err != nil {
		return nil, err
	}
	if tokenLen > uint64(len(b)) {
		return nil, errNotQuic
	}
	if _, err = buf.ReadBytes(int(tokenLen)); err != nil {
		return nil, err
	}

	packetLen, err := quicvarint.Read(buf)
	if err != nil {
		return nil, err
	}

	hdrLen := len(b) - buf.Len()
	origPNBytes := make([]byte, 4)
	copy(origPNBytes, b[hdrLen:hdrLen+4])

	salt := quicSaltOld
	if versionNumber == version1 {
		salt = quicSalt
	}

	initialSecret := hkdf.Extract(crypto.SHA256.New, connID, salt)
	secret, err := hkdfExpandLabel(crypto.SHA256, initialSecret, []byte{}, "client in", crypto.SHA256.Size())
	if err != nil {
		return nil, err
	}
	hpKey, err := hkdfExpandLabel(initialSuite.Hash, secret, []byte{}, "quic hp", initialSuite.KeyLen)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(hpKey)
	if err != nil {
		return nil, err
	}

	cache := buffer.New()
	defer cache.Release()

	mask := cache.Extend(block.BlockSize())
	block.Encrypt(mask, b[hdrLen+4:hdrLen+4+16])
	b[0] ^= mask[0] & 0xf
	for i := range b[hdrLen : hdrLen+4] {
		b[hdrLen+i] ^= mask[i+1]
	}
	packetNumberLength := b[0]&0x3 + 1
	if packetNumberLength != 1 {
		return nil, errNotQuicInitial
	}

	pn, err := buf.ReadByte()
	if err != nil {
		return nil, err
	}
	packetNumber := uint32(pn)
	if packetNumber != 0 && packetNumber != 1 {
		return nil, errNotQuicInitial
	}

	extHdrLen := hdrLen + int(packetNumberLength)
	copy(b[extHdrLen:hdrLen+4], origPNBytes[packetNumberLength:])
	data := b[extHdrLen : int(packetLen)+hdrLen]

	key, err := hkdfExpandLabel(crypto.SHA256, secret, []byte{}, "quic key", 16)
	if err != nil {
		return nil, err
	}
	iv, err := hkdfExpandLabel(crypto.SHA256, secret, []byte{}, "quic iv", 12)
	if err != nil {
		return nil, err
	}
	cipher := AEADAESGCMTLS13(key, iv)
	nonce := cache.Extend(cipher.NonceSize())
	binary.BigEndian.PutUint64(nonce[len(nonce)-8:], uint64(packetNumber))
	decrypted, err := cipher.Open(b[extHdrLen:extHdrLen], nonce, data, b[:extHdrLen])
	if err != nil {
		return nil, err
	}

	buf = buffer.FromBytes(decrypted)

	cryptoLen := uint(0)
	cryptoData := bytespool.Alloc(buf.Len())
	defer bytespool.Free(cryptoData)

	for !buf.IsEmpty() {
		frameType := byte(0x0) // Default to PADDING frame
		for frameType == 0x0 && !buf.IsEmpty() {
			frameType, err = buf.ReadByte()
			if err != nil {
				return nil, err
			}
		}

		switch frameType {
		case 0x00: // PADDING frame
		case 0x01: // PING frame
		case 0x02, 0x03: // ACK frame
			if _, err := quicvarint.Read(buf); err != nil { // Field: Largest Acknowledged
				return nil, err
			}
			if _, err := quicvarint.Read(buf); err != nil { // Field: ACK Delay
				return nil, err
			}
			ackRangeCount, err := quicvarint.Read(buf) // Field: ACK Range Count
			if err != nil {
				return nil, err
			}
			if _, err := quicvarint.Read(buf); err != nil { // Field: First ACK Range
				return nil, err
			}
			for i := 0; i < int(ackRangeCount); i++ {
				if _, err := quicvarint.Read(buf); err != nil { // Field: ACK Range -> Gap
					return nil, err
				}
				if _, err := quicvarint.Read(buf); err != nil { // Field: ACK Range -> ACK Range Length
					return nil, err
				}
			}
			if frameType == 0x03 {
				if _, err := quicvarint.Read(buf); err != nil { // Field: ECN Counts -> ECT0 Count
					return nil, err
				}
				if _, err := quicvarint.Read(buf); err != nil { // Field: ECN Counts -> ECT1 Count
					return nil, err
				}
				if _, err := quicvarint.Read(buf); err != nil { //nolint:misspell // Field: ECN Counts -> ECT-CE Count
					return nil, err
				}
			}
		case 0x06: // CRYPTO frame, we will use this frame
			offset, err := quicvarint.Read(buf) // Field: Offset
			if err != nil {
				return nil, err
			}
			length, err := quicvarint.Read(buf) // Field: Length
			if err != nil {
				return nil, err
			}
			if length > uint64(buf.Len()) {
				return nil, newError("unexpected length %d", length)
			}
			if cryptoLen < uint(offset+length) {
				cryptoLen = uint(offset + length)
			}
			if _, err := buf.Read(cryptoData[offset : offset+length]); err != nil { // Field: Crypto Data
				return nil, err
			}
		case 0x1c: // CONNECTION_CLOSE frame, only 0x1c is permitted in initial packet
			if _, err := quicvarint.Read(buf); err != nil { // Field: Error Code
				return nil, err
			}
			if _, err := quicvarint.Read(buf); err != nil { // Field: Frame Type
				return nil, err
			}
			length, err := quicvarint.Read(buf) // Field: Reason Phrase Length
			if err != nil {
				return nil, err
			}
			if _, err := buf.ReadBytes(int(length)); err != nil { // Field: Reason Phrase
				return nil, err
			}
		default:
			// Only above frame types are permitted in initial packet.
			// See https://www.rfc-editor.org/rfc/rfc9000.html#section-17.2.2-8
			return nil, errNotQuicInitial
		}
	}

	return cryptoData[:cryptoLen], nil
}

func hkdfExpandLabel(hash crypto.Hash, secret, ctx []byte, label string, length int) ([]byte, error) {
	b := make([]byte, 3, 3+6+len(label)+1+len(ctx))
	binary.BigEndian.PutUint16(b, uint16(length))
	b[2] = uint8(6 + len(label))
	b = append(b, []byte("tls13 ")...)
	b = append(b, []byte(label)...)
	b = b[:3+6+len(label)+1]
	b[3+6+len(label)] = uint8(len(ctx))
	b = append(b, ctx...)

	out := make([]byte, length)
	n, err := hkdf.Expand(hash.New, secret, b).Read(out)
	if err != nil || n != length {
		return nil, newError("quic: HKDF-Expand-Label invocation failed unexpectedly")
	}
	return out, nil
}
