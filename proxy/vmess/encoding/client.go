package encoding

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"hash"
	"hash/fnv"

	"golang.org/x/crypto/chacha20poly1305"

	"v2ray.com/core/common/bitmask"
	"v2ray.com/core/common/buffer"
	"v2ray.com/core/common/crypto"
	"v2ray.com/core/common/dice"
	"v2ray.com/core/common/drain"
	"v2ray.com/core/common/io"
	"v2ray.com/core/common/protocol"
	"v2ray.com/core/common/protocol/vmess"
	"v2ray.com/core/common/serial"
	vmessaead "v2ray.com/core/proxy/vmess/aead"
)

func hashTimestamp(h hash.Hash, t protocol.Timestamp) []byte {
	_, _ = serial.WriteUint64(h, uint64(t))
	_, _ = serial.WriteUint64(h, uint64(t))
	_, _ = serial.WriteUint64(h, uint64(t))
	_, _ = serial.WriteUint64(h, uint64(t))
	return h.Sum(nil)
}

// ClientSession stores connection session info for VMess client.
type ClientSession struct {
	idHash          vmess.IDHash
	requestBodyKey  [16]byte
	requestBodyIV   [16]byte
	responseBodyKey [16]byte
	responseBodyIV  [16]byte
	responseReader  io.Reader
	responseHeader  byte

	readDrainer drain.Drainer
}

// NewClientSession creates a new ClientSession.
func NewClientSession(idHash vmess.IDHash, behaviorSeed int64) *ClientSession {
	session := &ClientSession{
		idHash: idHash,
	}

	randomBytes := make([]byte, 33) // 16 + 16 + 1
	_, _ = rand.Read(randomBytes)
	copy(session.requestBodyKey[:], randomBytes[:16])
	copy(session.requestBodyIV[:], randomBytes[16:32])
	session.responseHeader = randomBytes[32]

	{
		BodyKey := sha256.Sum256(session.requestBodyKey[:])
		copy(session.responseBodyKey[:], BodyKey[:16])
		BodyIV := sha256.Sum256(session.requestBodyIV[:])
		copy(session.responseBodyIV[:], BodyIV[:16])
	}
	{
		var err error
		session.readDrainer, err = drain.NewBehaviorSeedLimitedDrainer(behaviorSeed, 18, 3266, 64)
		if err != nil {
			newError("unable to initialize drainer").WithError(err).AtDebug().Logging()
			session.readDrainer = drain.NewNopDrainer()
		}
	}

	return session
}

func (c *ClientSession) EncodeRequestHeader(header protocol.RequestHeader, writer io.Writer) error {
	buf := buffer.New()
	defer buf.Release()

	_ = buf.WriteByte(vmess.VersionName)
	_, _ = buf.Write(c.requestBodyIV[:])
	_, _ = buf.Write(c.requestBodyKey[:])
	_ = buf.WriteByte(c.responseHeader)
	_ = buf.WriteByte(byte(header.Option.Vmess))

	paddingLen := dice.Roll(16)
	security := byte(paddingLen<<4) | byte(header.User.Vmess.Security)
	_, _ = buf.Write([]byte{security, byte(0), byte(header.Command.Vmess)})

	if header.Command.Vmess != vmess.RequestCommandMux {
		if err := addrParser.WriteAddress(buf, header.Address.Address); err != nil {
			return newError("failed to writer address and port").WithError(err)
		}
	}

	if paddingLen > 0 {
		_, _ = buf.ReadFullFrom(rand.Reader, paddingLen)
	}

	{
		fnv1a := fnv.New32a()
		_, _ = buf.ReadToWriter(fnv1a, false)
		hashBytes := buf.Extend(fnv1a.Size())
		fnv1a.Sum(hashBytes[:0])
	}

	{
		var fixedLengthCmdKey [16]byte
		copy(fixedLengthCmdKey[:], header.User.Vmess.ID.CmdKey())
		vmessout, err := vmessaead.SealVMessAEADHeader(fixedLengthCmdKey, buf.Bytes())
		if err != nil {
			return err
		}
		_, _ = io.Copy(writer, bytes.NewReader(vmessout))
	}

	return nil
}

func (c *ClientSession) EncodeRequestBody(request protocol.RequestHeader, writer buffer.BufferedWriter) (buffer.Writer, error) {
	var sizeParser crypto.ChunkSizeEncoder = crypto.PlainChunkSizeParser{}
	if request.Option.Vmess.Has(vmess.RequestOptionChunkMasking) {
		sizeParser = NewShakeSizeParser(c.requestBodyIV[:])
	}
	var padding crypto.PaddingLengthGenerator
	if request.Option.Vmess.Has(vmess.RequestOptionGlobalPadding) {
		var ok bool
		padding, ok = sizeParser.(crypto.PaddingLengthGenerator)
		if !ok {
			return nil, newError("invalid option: RequestOptionGlobalPadding")
		}
	}

	switch request.User.Vmess.Security {
	case vmess.Security_NONE:
		if request.Option.Vmess.Has(vmess.RequestOptionChunkStream) {
			if request.Command.TransferType(request.Command.Vmess.TransferType()) == protocol.TransferTypeStream {
				return crypto.NewChunkStreamWriter(sizeParser, writer), nil
			}
			auth := &crypto.AEADAuthenticator{
				AEAD:                    new(NoOpAuthenticator),
				NonceGenerator:          crypto.GenerateEmptyBytes(),
				AdditionalDataGenerator: crypto.GenerateEmptyBytes(),
			}
			return crypto.NewAuthenticationWriter(auth, sizeParser, writer, protocol.TransferTypePacket, padding), nil
		}

		return writer, nil
	case vmess.Security_LEGACY:
		aesStream, err := crypto.NewAesEncryptionStream(c.requestBodyKey[:], c.requestBodyIV[:])
		if err != nil {
			return nil, err
		}
		cryptionWriter := crypto.NewCryptionWriter(aesStream, writer)
		if request.Option.Vmess.Has(vmess.RequestOptionChunkStream) {
			auth := &crypto.AEADAuthenticator{
				AEAD:                    new(FnvAuthenticator),
				NonceGenerator:          crypto.GenerateEmptyBytes(),
				AdditionalDataGenerator: crypto.GenerateEmptyBytes(),
			}
			return crypto.NewAuthenticationWriter(auth, sizeParser, cryptionWriter, request.Command.TransferType(request.Command.Vmess.TransferType()), padding), nil
		}

		return buffer.NewSequentialWriter(cryptionWriter), nil
	case vmess.Security_AES_128_GCM:
		aead, err := crypto.NewAesGcm(c.requestBodyKey[:])
		if err != nil {
			return nil, err
		}
		auth := &crypto.AEADAuthenticator{
			AEAD:                    aead,
			NonceGenerator:          GenerateChunkNonce(c.requestBodyIV[:], uint32(aead.NonceSize())),
			AdditionalDataGenerator: crypto.GenerateEmptyBytes(),
		}
		if request.Option.Vmess.Has(vmess.RequestOptionAuthenticatedLength) {
			AuthenticatedLengthKey := vmessaead.KDF16(c.requestBodyKey[:], "auth_len")
			AuthenticatedLengthKeyAEAD, err := crypto.NewAesGcm(AuthenticatedLengthKey)
			if err != nil {
				return nil, err
			}

			lengthAuth := &crypto.AEADAuthenticator{
				AEAD:                    AuthenticatedLengthKeyAEAD,
				NonceGenerator:          GenerateChunkNonce(c.requestBodyIV[:], uint32(aead.NonceSize())),
				AdditionalDataGenerator: crypto.GenerateEmptyBytes(),
			}
			sizeParser = NewAEADSizeParser(lengthAuth)
		}
		return crypto.NewAuthenticationWriter(auth, sizeParser, writer, request.Command.TransferType(request.Command.Vmess.TransferType()), padding), nil
	case vmess.Security_CHACHA20_POLY1305:
		aead, err := chacha20poly1305.New(GenerateChacha20Poly1305Key(c.requestBodyKey[:]))
		if err != nil {
			return nil, err
		}

		auth := &crypto.AEADAuthenticator{
			AEAD:                    aead,
			NonceGenerator:          GenerateChunkNonce(c.requestBodyIV[:], uint32(aead.NonceSize())),
			AdditionalDataGenerator: crypto.GenerateEmptyBytes(),
		}
		if request.Option.Vmess.Has(vmess.RequestOptionAuthenticatedLength) {
			AuthenticatedLengthKey := vmessaead.KDF16(c.requestBodyKey[:], "auth_len")
			AuthenticatedLengthKeyAEAD, err := chacha20poly1305.New(GenerateChacha20Poly1305Key(AuthenticatedLengthKey))
			if err != nil {
				return nil, err
			}

			lengthAuth := &crypto.AEADAuthenticator{
				AEAD:                    AuthenticatedLengthKeyAEAD,
				NonceGenerator:          GenerateChunkNonce(c.requestBodyIV[:], uint32(aead.NonceSize())),
				AdditionalDataGenerator: crypto.GenerateEmptyBytes(),
			}
			sizeParser = NewAEADSizeParser(lengthAuth)
		}
		return crypto.NewAuthenticationWriter(auth, sizeParser, writer, request.Command.TransferType(request.Command.Vmess.TransferType()), padding), nil
	default:
		return nil, newError("invalid option: Security")
	}
}

func (c *ClientSession) DecodeResponseHeader(reader io.Reader) (protocol.ResponseHeader, error) {
	{
		aeadResponseHeaderLengthEncryptionKey := vmessaead.KDF16(c.responseBodyKey[:], vmessaead.KDFSaltConstAEADRespHeaderLenKey)
		aeadResponseHeaderLengthEncryptionIV := vmessaead.KDF(c.responseBodyIV[:], vmessaead.KDFSaltConstAEADRespHeaderLenIV)[:12]

		aeadResponseHeaderLengthEncryptionKeyAESBlock, err := aes.NewCipher(aeadResponseHeaderLengthEncryptionKey)
		if err != nil {
			return protocol.ResponseHeader{}, err
		}
		aeadResponseHeaderLengthEncryptionAEAD, err := cipher.NewGCM(aeadResponseHeaderLengthEncryptionKeyAESBlock)
		if err != nil {
			return protocol.ResponseHeader{}, err
		}

		var aeadEncryptedResponseHeaderLength [18]byte
		var decryptedResponseHeaderLength int
		var decryptedResponseHeaderLengthBinaryDeserializeBuffer uint16

		if n, err := io.ReadFull(reader, aeadEncryptedResponseHeaderLength[:]); err != nil {
			c.readDrainer.AcknowledgeReceive(n)
			return protocol.ResponseHeader{}, drain.WithError(c.readDrainer, reader, newError("Unable to Read Header Len").WithError(err))
		} else { // nolint: golint
			c.readDrainer.AcknowledgeReceive(n)
		}
		if decryptedResponseHeaderLengthBinaryBuffer, err := aeadResponseHeaderLengthEncryptionAEAD.Open(nil, aeadResponseHeaderLengthEncryptionIV, aeadEncryptedResponseHeaderLength[:], nil); err != nil {
			return protocol.ResponseHeader{}, drain.WithError(c.readDrainer, reader, newError("Failed To Decrypt Length").WithError(err))
		} else { // nolint: golint
			_ = binary.Read(bytes.NewReader(decryptedResponseHeaderLengthBinaryBuffer), binary.BigEndian, &decryptedResponseHeaderLengthBinaryDeserializeBuffer)
			decryptedResponseHeaderLength = int(decryptedResponseHeaderLengthBinaryDeserializeBuffer)
		}

		aeadResponseHeaderPayloadEncryptionKey := vmessaead.KDF16(c.responseBodyKey[:], vmessaead.KDFSaltConstAEADRespHeaderPayloadKey)
		aeadResponseHeaderPayloadEncryptionIV := vmessaead.KDF(c.responseBodyIV[:], vmessaead.KDFSaltConstAEADRespHeaderPayloadIV)[:12]

		aeadResponseHeaderPayloadEncryptionKeyAESBlock, err := aes.NewCipher(aeadResponseHeaderPayloadEncryptionKey)
		if err != nil {
			return protocol.ResponseHeader{}, err
		}
		aeadResponseHeaderPayloadEncryptionAEAD, err := cipher.NewGCM(aeadResponseHeaderPayloadEncryptionKeyAESBlock)
		if err != nil {
			return protocol.ResponseHeader{}, err
		}

		encryptedResponseHeaderBuffer := make([]byte, decryptedResponseHeaderLength+16)

		if n, err := io.ReadFull(reader, encryptedResponseHeaderBuffer); err != nil {
			c.readDrainer.AcknowledgeReceive(n)
			return protocol.ResponseHeader{}, drain.WithError(c.readDrainer, reader, newError("Unable to Read Header Data").WithError(err))
		} else { // nolint: golint
			c.readDrainer.AcknowledgeReceive(n)
		}

		if decryptedResponseHeaderBuffer, err := aeadResponseHeaderPayloadEncryptionAEAD.Open(nil, aeadResponseHeaderPayloadEncryptionIV, encryptedResponseHeaderBuffer, nil); err != nil {
			return protocol.ResponseHeader{}, drain.WithError(c.readDrainer, reader, newError("Failed To Decrypt Payload").WithError(err))
		} else { // nolint: golint
			c.responseReader = bytes.NewReader(decryptedResponseHeaderBuffer)
		}
	}

	buf := buffer.New()
	defer buf.Release()

	if _, err := buf.ReadFullFrom(c.responseReader, 4); err != nil {
		return protocol.ResponseHeader{}, newError("failed to read response header").WithError(err)
	}

	if buf.Byte(0) != c.responseHeader {
		return protocol.ResponseHeader{}, newError("unexpected response header. Expecting ", int(c.responseHeader), " but actually ", int(buf.Byte(0)))
	}

	header := protocol.ResponseHeader{
		Option: protocol.ResponseOption{
			Vmess: bitmask.Byte(buf.Byte(1)),
		},
	}

	if buf.Byte(2) != 0 {
		cmdID := buf.Byte(2)
		dataLen := int(buf.Byte(3))

		buf.Clear()
		if _, err := buf.ReadFullFrom(c.responseReader, dataLen); err != nil {
			return protocol.ResponseHeader{}, newError("failed to read response command").WithError(err)
		}
		command, err := UnmarshalCommand(cmdID, buf.Bytes())
		if err == nil {
			header.Command = protocol.ResponseCommand{
				Vmess: command,
			}
		}
	}
	{
		aesStream, err := crypto.NewAesDecryptionStream(c.responseBodyKey[:], c.responseBodyIV[:])
		if err != nil {
			return protocol.ResponseHeader{}, err
		}
		c.responseReader = crypto.NewCryptionReader(aesStream, reader)
	}
	return header, nil
}

func (c *ClientSession) DecodeResponseBody(request protocol.RequestHeader, reader buffer.BufferedReader) (buffer.Reader, error) {
	var sizeParser crypto.ChunkSizeDecoder = crypto.PlainChunkSizeParser{}
	if request.Option.Vmess.Has(vmess.RequestOptionChunkMasking) {
		sizeParser = NewShakeSizeParser(c.responseBodyIV[:])
	}
	var padding crypto.PaddingLengthGenerator
	if request.Option.Vmess.Has(vmess.RequestOptionGlobalPadding) {
		var ok bool
		padding, ok = sizeParser.(crypto.PaddingLengthGenerator)
		if !ok {
			return nil, newError("invalid option: RequestOptionGlobalPadding")
		}
	}

	switch request.User.Vmess.Security {
	case vmess.Security_NONE:
		if request.Option.Vmess.Has(vmess.RequestOptionChunkStream) {
			if request.Command.TransferType(request.Command.Vmess.TransferType()) == protocol.TransferTypeStream {
				return crypto.NewChunkStreamReader(sizeParser, reader), nil
			}

			auth := &crypto.AEADAuthenticator{
				AEAD:                    new(NoOpAuthenticator),
				NonceGenerator:          crypto.GenerateEmptyBytes(),
				AdditionalDataGenerator: crypto.GenerateEmptyBytes(),
			}

			return crypto.NewAuthenticationReader(auth, reader, sizeParser, protocol.TransferTypePacket, padding), nil
		}

		return reader, nil
	case vmess.Security_LEGACY:
		if request.Option.Vmess.Has(vmess.RequestOptionChunkStream) {
			auth := &crypto.AEADAuthenticator{
				AEAD:                    new(FnvAuthenticator),
				NonceGenerator:          crypto.GenerateEmptyBytes(),
				AdditionalDataGenerator: crypto.GenerateEmptyBytes(),
			}
			return crypto.NewAuthenticationReader(auth, buffer.NewBufferedReader(buffer.NewIOReader(c.responseReader)), sizeParser, request.Command.TransferType(request.Command.Vmess.TransferType()), padding), nil
		}

		return buffer.NewIOReader(c.responseReader), nil
	case vmess.Security_AES_128_GCM:
		aead, err := crypto.NewAesGcm(c.responseBodyKey[:])
		if err != nil {
			return nil, err
		}

		auth := &crypto.AEADAuthenticator{
			AEAD:                    aead,
			NonceGenerator:          GenerateChunkNonce(c.responseBodyIV[:], uint32(aead.NonceSize())),
			AdditionalDataGenerator: crypto.GenerateEmptyBytes(),
		}
		if request.Option.Vmess.Has(vmess.RequestOptionAuthenticatedLength) {
			AuthenticatedLengthKey := vmessaead.KDF16(c.requestBodyKey[:], "auth_len")
			AuthenticatedLengthKeyAEAD, err := crypto.NewAesGcm(AuthenticatedLengthKey)
			if err != nil {
				return nil, err
			}

			lengthAuth := &crypto.AEADAuthenticator{
				AEAD:                    AuthenticatedLengthKeyAEAD,
				NonceGenerator:          GenerateChunkNonce(c.requestBodyIV[:], uint32(aead.NonceSize())),
				AdditionalDataGenerator: crypto.GenerateEmptyBytes(),
			}
			sizeParser = NewAEADSizeParser(lengthAuth)
		}
		return crypto.NewAuthenticationReader(auth, reader, sizeParser, request.Command.TransferType(request.Command.Vmess.TransferType()), padding), nil
	case vmess.Security_CHACHA20_POLY1305:
		aead, err := chacha20poly1305.New(GenerateChacha20Poly1305Key(c.responseBodyKey[:]))
		if err != nil {
			return nil, err
		}

		auth := &crypto.AEADAuthenticator{
			AEAD:                    aead,
			NonceGenerator:          GenerateChunkNonce(c.responseBodyIV[:], uint32(aead.NonceSize())),
			AdditionalDataGenerator: crypto.GenerateEmptyBytes(),
		}
		if request.Option.Vmess.Has(vmess.RequestOptionAuthenticatedLength) {
			AuthenticatedLengthKey := vmessaead.KDF16(c.requestBodyKey[:], "auth_len")
			AuthenticatedLengthKeyAEAD, err := chacha20poly1305.New(GenerateChacha20Poly1305Key(AuthenticatedLengthKey))
			if err != nil {
				return nil, err
			}

			lengthAuth := &crypto.AEADAuthenticator{
				AEAD:                    AuthenticatedLengthKeyAEAD,
				NonceGenerator:          GenerateChunkNonce(c.requestBodyIV[:], uint32(aead.NonceSize())),
				AdditionalDataGenerator: crypto.GenerateEmptyBytes(),
			}
			sizeParser = NewAEADSizeParser(lengthAuth)
		}
		return crypto.NewAuthenticationReader(auth, reader, sizeParser, request.Command.TransferType(request.Command.Vmess.TransferType()), padding), nil
	default:
		return nil, newError("invalid option: Security")
	}
}

func GenerateChunkNonce(nonce []byte, size uint32) crypto.BytesGenerator {
	c := append([]byte{}, nonce...)
	count := uint16(0)
	return func() []byte {
		binary.BigEndian.PutUint16(c, count)
		count++
		return c[:size]
	}
}
