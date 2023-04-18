package encoding

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/sha256"
	"encoding/binary"
	"hash/fnv"
	"sync"
	"time"

	"golang.org/x/crypto/chacha20poly1305"

	"v2ray.com/core/common/bitmask"
	"v2ray.com/core/common/buffer"
	"v2ray.com/core/common/crypto"
	"v2ray.com/core/common/drain"
	"v2ray.com/core/common/io"
	"v2ray.com/core/common/protocol"
	"v2ray.com/core/common/protocol/vmess"
	"v2ray.com/core/common/task"
	vmessaead "v2ray.com/core/proxy/vmess/aead"
	"v2ray.com/core/proxy/vmess/validator"
)

const (
	sessionHistoryPeriod = 30 * time.Second
)

type sessionID struct {
	user  [16]byte
	key   [16]byte
	nonce [16]byte
}

// SessionHistory keeps track of historical session ids, to prevent replay attacks.
type SessionHistory struct {
	sync.Mutex

	cache map[sessionID]time.Time
	task  task.Timer
}

// NewSessionHistory creates a new SessionHistory object.
func NewSessionHistory() *SessionHistory {
	h := &SessionHistory{
		cache: make(map[sessionID]time.Time, 128),
		task:  task.NewTimer(),
	}

	go h.task.Period(func() {
		_ = h.removeExpiredEntries()
	}, sessionHistoryPeriod)

	return h
}

// Close implements common.Closable.
func (h *SessionHistory) Close() error {
	return h.task.Close()
}

func (h *SessionHistory) addIfNotExits(session sessionID) bool {
	h.Lock()
	defer h.Unlock()

	if expire, found := h.cache[session]; found && expire.After(time.Now()) {
		return false
	}

	h.cache[session] = time.Now().Add(time.Minute * 3)
	return true
}

func (h *SessionHistory) removeExpiredEntries() error {
	now := time.Now()

	h.Lock()
	defer h.Unlock()

	if len(h.cache) == 0 {
		return newError("nothing to do")
	}

	for session, expire := range h.cache {
		if expire.Before(now) {
			delete(h.cache, session)
		}
	}

	if len(h.cache) == 0 {
		h.cache = make(map[sessionID]time.Time, 128)
	}

	return nil
}

// ServerSession keeps information for a session in VMess server.
type ServerSession struct {
	userValidator   *validator.TimedUserValidator
	sessionHistory  *SessionHistory
	requestBodyKey  [16]byte
	requestBodyIV   [16]byte
	responseBodyKey [16]byte
	responseBodyIV  [16]byte
	responseWriter  io.Writer
	responseHeader  byte

	isAEADRequest bool

	isAEADForced bool
}

// NewServerSession creates a new ServerSession, using the given UserValidator.
// The ServerSession instance doesn't take ownership of the validator.
func NewServerSession(validator *validator.TimedUserValidator, sessionHistory *SessionHistory) *ServerSession {
	return &ServerSession{
		userValidator:  validator,
		sessionHistory: sessionHistory,
	}
}

// SetAEADForced sets isAEADForced for a ServerSession.
func (s *ServerSession) SetAEADForced(isAEADForced bool) {
	s.isAEADForced = isAEADForced
}

func parseSecurityType(b byte) vmess.Security {
	if s, f := vmess.Security_Name[int32(b)]; f {
		return s
	}
	return vmess.Security_UNKNOWN
}

// DecodeRequestHeader decodes and returns (if successful) a RequestHeader from an input stream.
func (s *ServerSession) DecodeRequestHeader(reader io.Reader) (protocol.RequestHeader, error) {
	buf := buffer.New()

	drainer, err := drain.NewBehaviorSeedLimitedDrainer(int64(s.userValidator.GetBehaviorSeed()), 16+38, 3266, 64)
	if err != nil {
		return protocol.RequestHeader{}, newError("failed to initialize drainer").WithError(err)
	}

	drainConnection := func(e error) error {
		// We read a deterministic generated length of data before closing the connection to offset padding read pattern
		drainer.AcknowledgeReceive(buf.Len())
		return drain.WithError(drainer, reader, e)
	}

	defer buf.Release()

	if _, err := buf.ReadFullFrom(reader, vmess.IDBytesLen); err != nil {
		return protocol.RequestHeader{}, newError("failed to read request header").WithError(err)
	}

	var decryptor io.Reader

	user, foundAEAD, errorAEAD := s.userValidator.GetAEAD(buf.Bytes())

	var fixedSizeAuthID [16]byte
	copy(fixedSizeAuthID[:], buf.Bytes())

	switch {
	case foundAEAD:
		var fixedSizeCmdKey [16]byte
		copy(fixedSizeCmdKey[:], user.ID.CmdKey())
		aeadData, shouldDrain, bytesRead, errorReason := vmessaead.OpenVMessAEADHeader(fixedSizeCmdKey, fixedSizeAuthID, reader)
		if errorReason != nil {
			if shouldDrain {
				drainer.AcknowledgeReceive(bytesRead)
				return protocol.RequestHeader{}, drainConnection(newError("AEAD read failed").WithError(errorReason))
			}
			return protocol.RequestHeader{}, drainConnection(newError("AEAD read failed, drain skipped").WithError(errorReason))
		}
		decryptor = bytes.NewReader(aeadData)
		s.isAEADRequest = true

	case errorAEAD == vmessaead.ErrNotFound:
		userLegacy, timestamp, valid, userValidationError := s.userValidator.Get(buf.Bytes())
		if !valid || userValidationError != nil {
			return protocol.RequestHeader{}, drainConnection(newError("invalid user").WithError(userValidationError))
		}
		if s.isAEADForced {
			return protocol.RequestHeader{}, drainConnection(newError("invalid user: VMessAEAD is enforced and a non VMessAEAD connection is received. You can still disable this security feature with environment variable v2ray.vmess.aead.forced = false . You will not be able to enable legacy header workaround in the future."))
		}
		if s.userValidator.ShouldShowLegacyWarn() {
			return protocol.RequestHeader{}, drainConnection(newError("Critical Warning: potentially invalid user: a non VMessAEAD connection is received. From 2022 Jan 1st, this kind of connection will be rejected by default. You should update or replace your client software now. This message will not be shown for further violation on this inbound."))
		}
		user = userLegacy
		iv := hashTimestamp(md5.New(), timestamp)

		aesStream, err := crypto.NewAesDecryptionStream(user.ID.CmdKey(), iv)
		if err != nil {
			return protocol.RequestHeader{}, err
		}
		decryptor = crypto.NewCryptionReader(aesStream, reader)

	default:
		return protocol.RequestHeader{}, drainConnection(newError("invalid user").WithError(errorAEAD))
	}

	drainer.AcknowledgeReceive(buf.Len())
	buf.Clear()
	if _, err := buf.ReadFullFrom(decryptor, 38); err != nil {
		return protocol.RequestHeader{}, newError("failed to read request header").WithError(err)
	}

	request := protocol.RequestHeader{
		Version: protocol.RequestVersion{
			Vmess: buf.Byte(0),
		},
		User: protocol.RequestUser{
			Vmess: user,
		},
	}

	copy(s.requestBodyIV[:], buf.BytesRange(1, 17))   // 16 bytes
	copy(s.requestBodyKey[:], buf.BytesRange(17, 33)) // 16 bytes
	var sid sessionID
	copy(sid.user[:], request.User.Vmess.ID.Bytes())
	sid.key = s.requestBodyKey
	sid.nonce = s.requestBodyIV
	if !s.sessionHistory.addIfNotExits(sid) {
		if !s.isAEADRequest {
			drainErr := s.userValidator.BurnTaintFuse(fixedSizeAuthID[:])
			if drainErr != nil {
				return protocol.RequestHeader{}, drainConnection(newError("duplicated session id, possibly under replay attack, and failed to taint userHash").WithError(drainErr))
			}
			return protocol.RequestHeader{}, drainConnection(newError("duplicated session id, possibly under replay attack, userHash tainted"))
		}
		return protocol.RequestHeader{}, newError("duplicated session id, possibly under replay attack, but this is a AEAD request")
	}

	s.responseHeader = buf.Byte(33)                   // 1 byte
	request.Option.Vmess = bitmask.Byte(buf.Byte(34)) // 1 byte
	paddingLen := int(buf.Byte(35) >> 4)
	request.User.Vmess.Security = parseSecurityType(buf.Byte(35) & 0x0F)
	// 1 bytes reserved
	request.Command.Vmess = vmess.RequestCommand(buf.Byte(37))

	if address, err := addrParser.ReadAddress(buf, decryptor); err == nil {
		request.Address = protocol.RequestAddress{
			Address: address,
		}
	}

	if paddingLen > 0 {
		if _, err := buf.ReadFullFrom(decryptor, paddingLen); err != nil {
			if !s.isAEADRequest {
				burnErr := s.userValidator.BurnTaintFuse(fixedSizeAuthID[:])
				if burnErr != nil {
					return protocol.RequestHeader{}, newError("failed to read padding, failed to taint userHash").WithError(burnErr).WithError(err)
				}
				return protocol.RequestHeader{}, newError("failed to read padding, userHash tainted").WithError(err)
			}
			return protocol.RequestHeader{}, newError("failed to read padding").WithError(err)
		}
	}

	if _, err := buf.ReadFullFrom(decryptor, 4); err != nil {
		if !s.isAEADRequest {
			burnErr := s.userValidator.BurnTaintFuse(fixedSizeAuthID[:])
			if burnErr != nil {
				return protocol.RequestHeader{}, newError("failed to read checksum, failed to taint userHash").WithError(burnErr).WithError(err)
			}
			return protocol.RequestHeader{}, newError("failed to read checksum, userHash tainted").WithError(err)
		}
		return protocol.RequestHeader{}, newError("failed to read checksum").WithError(err)
	}

	fnv1a := fnv.New32a()
	_, _ = fnv1a.Write(buf.BytesTo(-4))
	actualHash := fnv1a.Sum32()
	expectedHash := binary.BigEndian.Uint32(buf.BytesFrom(-4))

	if actualHash != expectedHash {
		if !s.isAEADRequest {
			Autherr := newError("invalid auth, legacy userHash tainted")
			burnErr := s.userValidator.BurnTaintFuse(fixedSizeAuthID[:])
			if burnErr != nil {
				Autherr = newError("invalid auth, can't taint legacy userHash").WithError(burnErr)
			}
			// It is possible that we are under attack described in https://github.com/v2ray/v2ray-core/issues/2523
			return protocol.RequestHeader{}, drainConnection(Autherr)
		}
		return protocol.RequestHeader{}, newError("invalid auth, but this is a AEAD request")
	}

	if request.User.Vmess.Security == vmess.Security_UNKNOWN || request.User.Vmess.Security == vmess.Security_AUTO {
		return protocol.RequestHeader{}, newError("unknown security type: ", request.User.Vmess.Security)
	}

	return request, nil
}

// DecodeRequestBody returns Reader from which caller can fetch decrypted body.
func (s *ServerSession) DecodeRequestBody(request protocol.RequestHeader, reader buffer.BufferedReader) (buffer.Reader, error) {
	var sizeParser crypto.ChunkSizeDecoder = crypto.PlainChunkSizeParser{}
	if request.Option.Vmess.Has(vmess.RequestOptionChunkMasking) {
		sizeParser = NewShakeSizeParser(s.requestBodyIV[:])
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
		aesStream, err := crypto.NewAesDecryptionStream(s.requestBodyKey[:], s.requestBodyIV[:])
		if err != nil {
			return nil, err
		}
		cryptionReader := buffer.NewBufferedReader(buffer.NewIOReader(crypto.NewCryptionReader(aesStream, reader)))
		if request.Option.Vmess.Has(vmess.RequestOptionChunkStream) {
			auth := &crypto.AEADAuthenticator{
				AEAD:                    new(FnvAuthenticator),
				NonceGenerator:          crypto.GenerateEmptyBytes(),
				AdditionalDataGenerator: crypto.GenerateEmptyBytes(),
			}
			return crypto.NewAuthenticationReader(auth, cryptionReader, sizeParser, request.Command.TransferType(request.Command.Vmess.TransferType()), padding), nil
		}
		return cryptionReader, nil

	case vmess.Security_AES_128_GCM:
		aead, err := crypto.NewAesGcm(s.requestBodyKey[:])
		if err != nil {
			return nil, err
		}
		auth := &crypto.AEADAuthenticator{
			AEAD:                    aead,
			NonceGenerator:          GenerateChunkNonce(s.requestBodyIV[:], uint32(aead.NonceSize())),
			AdditionalDataGenerator: crypto.GenerateEmptyBytes(),
		}
		if request.Option.Vmess.Has(vmess.RequestOptionAuthenticatedLength) {
			AuthenticatedLengthKey := vmessaead.KDF16(s.requestBodyKey[:], "auth_len")
			AuthenticatedLengthKeyAEAD, err := crypto.NewAesGcm(AuthenticatedLengthKey)
			if err != nil {
				return nil, err
			}

			lengthAuth := &crypto.AEADAuthenticator{
				AEAD:                    AuthenticatedLengthKeyAEAD,
				NonceGenerator:          GenerateChunkNonce(s.requestBodyIV[:], uint32(aead.NonceSize())),
				AdditionalDataGenerator: crypto.GenerateEmptyBytes(),
			}
			sizeParser = NewAEADSizeParser(lengthAuth)
		}
		return crypto.NewAuthenticationReader(auth, reader, sizeParser, request.Command.TransferType(request.Command.Vmess.TransferType()), padding), nil

	case vmess.Security_CHACHA20_POLY1305:
		aead, _ := chacha20poly1305.New(GenerateChacha20Poly1305Key(s.requestBodyKey[:]))

		auth := &crypto.AEADAuthenticator{
			AEAD:                    aead,
			NonceGenerator:          GenerateChunkNonce(s.requestBodyIV[:], uint32(aead.NonceSize())),
			AdditionalDataGenerator: crypto.GenerateEmptyBytes(),
		}
		if request.Option.Vmess.Has(vmess.RequestOptionAuthenticatedLength) {
			AuthenticatedLengthKey := vmessaead.KDF16(s.requestBodyKey[:], "auth_len")
			AuthenticatedLengthKeyAEAD, _ := chacha20poly1305.New(GenerateChacha20Poly1305Key(AuthenticatedLengthKey))

			lengthAuth := &crypto.AEADAuthenticator{
				AEAD:                    AuthenticatedLengthKeyAEAD,
				NonceGenerator:          GenerateChunkNonce(s.requestBodyIV[:], uint32(aead.NonceSize())),
				AdditionalDataGenerator: crypto.GenerateEmptyBytes(),
			}
			sizeParser = NewAEADSizeParser(lengthAuth)
		}
		return crypto.NewAuthenticationReader(auth, reader, sizeParser, request.Command.TransferType(request.Command.Vmess.TransferType()), padding), nil

	default:
		return nil, newError("invalid option: Security")
	}
}

// EncodeResponseHeader writes encoded response header into the given writer.
func (s *ServerSession) EncodeResponseHeader(header protocol.ResponseHeader, writer buffer.BufferedWriter) error {
	var encryptionWriter io.Writer
	if !s.isAEADRequest {
		s.responseBodyKey = md5.Sum(s.requestBodyKey[:])
		s.responseBodyIV = md5.Sum(s.requestBodyIV[:])
	} else {
		BodyKey := sha256.Sum256(s.requestBodyKey[:])
		copy(s.responseBodyKey[:], BodyKey[:16])
		BodyIV := sha256.Sum256(s.requestBodyIV[:])
		copy(s.responseBodyIV[:], BodyIV[:16])
	}

	aesStream, err := crypto.NewAesEncryptionStream(s.responseBodyKey[:], s.responseBodyIV[:])
	if err != nil {
		return err
	}
	encryptionWriter = crypto.NewCryptionWriter(aesStream, writer)
	s.responseWriter = encryptionWriter

	aeadEncryptedHeaderBuffer := bytes.NewBuffer(nil)

	if s.isAEADRequest {
		encryptionWriter = aeadEncryptedHeaderBuffer
	}

	_, _ = encryptionWriter.Write([]byte{s.responseHeader, byte(header.Option.Vmess)})

	if err := MarshalCommand(header.Command.Vmess, encryptionWriter); err != nil {
		_, _ = encryptionWriter.Write([]byte{0x00, 0x00})
	}

	if s.isAEADRequest {
		aeadResponseHeaderLengthEncryptionKey := vmessaead.KDF16(s.responseBodyKey[:], vmessaead.KDFSaltConstAEADRespHeaderLenKey)
		aeadResponseHeaderLengthEncryptionIV := vmessaead.KDF(s.responseBodyIV[:], vmessaead.KDFSaltConstAEADRespHeaderLenIV)[:12]

		aeadResponseHeaderLengthEncryptionKeyAESBlock, _ := aes.NewCipher(aeadResponseHeaderLengthEncryptionKey)
		aeadResponseHeaderLengthEncryptionAEAD, _ := cipher.NewGCM(aeadResponseHeaderLengthEncryptionKeyAESBlock)

		aeadResponseHeaderLengthEncryptionBuffer := bytes.NewBuffer(nil)

		decryptedResponseHeaderLengthBinaryDeserializeBuffer := uint16(aeadEncryptedHeaderBuffer.Len())

		_ = binary.Write(aeadResponseHeaderLengthEncryptionBuffer, binary.BigEndian, decryptedResponseHeaderLengthBinaryDeserializeBuffer)

		AEADEncryptedLength := aeadResponseHeaderLengthEncryptionAEAD.Seal(nil, aeadResponseHeaderLengthEncryptionIV, aeadResponseHeaderLengthEncryptionBuffer.Bytes(), nil)
		_, _ = io.Copy(writer, bytes.NewReader(AEADEncryptedLength))

		aeadResponseHeaderPayloadEncryptionKey := vmessaead.KDF16(s.responseBodyKey[:], vmessaead.KDFSaltConstAEADRespHeaderPayloadKey)
		aeadResponseHeaderPayloadEncryptionIV := vmessaead.KDF(s.responseBodyIV[:], vmessaead.KDFSaltConstAEADRespHeaderPayloadIV)[:12]

		aeadResponseHeaderPayloadEncryptionKeyAESBlock, _ := aes.NewCipher(aeadResponseHeaderPayloadEncryptionKey)
		aeadResponseHeaderPayloadEncryptionAEAD, _ := cipher.NewGCM(aeadResponseHeaderPayloadEncryptionKeyAESBlock)

		aeadEncryptedHeaderPayload := aeadResponseHeaderPayloadEncryptionAEAD.Seal(nil, aeadResponseHeaderPayloadEncryptionIV, aeadEncryptedHeaderBuffer.Bytes(), nil)
		_, _ = io.Copy(writer, bytes.NewReader(aeadEncryptedHeaderPayload))
	}

	return nil
}

// EncodeResponseBody returns a Writer that auto-encrypt content written by caller.
func (s *ServerSession) EncodeResponseBody(request protocol.RequestHeader, writer buffer.BufferedWriter) (buffer.Writer, error) {
	var sizeParser crypto.ChunkSizeEncoder = crypto.PlainChunkSizeParser{}
	if request.Option.Vmess.Has(vmess.RequestOptionChunkMasking) {
		sizeParser = NewShakeSizeParser(s.responseBodyIV[:])
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
		if request.Option.Vmess.Has(vmess.RequestOptionChunkStream) {
			auth := &crypto.AEADAuthenticator{
				AEAD:                    new(FnvAuthenticator),
				NonceGenerator:          crypto.GenerateEmptyBytes(),
				AdditionalDataGenerator: crypto.GenerateEmptyBytes(),
			}
			return crypto.NewAuthenticationWriter(auth, sizeParser, buffer.NewSequentialWriter(s.responseWriter), request.Command.TransferType(request.Command.Vmess.TransferType()), padding), nil
		}
		return buffer.NewSequentialWriter(s.responseWriter), nil

	case vmess.Security_AES_128_GCM:
		aead, err := crypto.NewAesGcm(s.responseBodyKey[:])
		if err != nil {
			return nil, err
		}
		auth := &crypto.AEADAuthenticator{
			AEAD:                    aead,
			NonceGenerator:          GenerateChunkNonce(s.responseBodyIV[:], uint32(aead.NonceSize())),
			AdditionalDataGenerator: crypto.GenerateEmptyBytes(),
		}
		if request.Option.Vmess.Has(vmess.RequestOptionAuthenticatedLength) {
			AuthenticatedLengthKey := vmessaead.KDF16(s.requestBodyKey[:], "auth_len")
			AuthenticatedLengthKeyAEAD, err := crypto.NewAesGcm(AuthenticatedLengthKey)
			if err != nil {
				return nil, err
			}

			lengthAuth := &crypto.AEADAuthenticator{
				AEAD:                    AuthenticatedLengthKeyAEAD,
				NonceGenerator:          GenerateChunkNonce(s.requestBodyIV[:], uint32(aead.NonceSize())),
				AdditionalDataGenerator: crypto.GenerateEmptyBytes(),
			}
			sizeParser = NewAEADSizeParser(lengthAuth)
		}
		return crypto.NewAuthenticationWriter(auth, sizeParser, writer, request.Command.TransferType(request.Command.Vmess.TransferType()), padding), nil

	case vmess.Security_CHACHA20_POLY1305:
		aead, _ := chacha20poly1305.New(GenerateChacha20Poly1305Key(s.responseBodyKey[:]))

		auth := &crypto.AEADAuthenticator{
			AEAD:                    aead,
			NonceGenerator:          GenerateChunkNonce(s.responseBodyIV[:], uint32(aead.NonceSize())),
			AdditionalDataGenerator: crypto.GenerateEmptyBytes(),
		}
		if request.Option.Vmess.Has(vmess.RequestOptionAuthenticatedLength) {
			AuthenticatedLengthKey := vmessaead.KDF16(s.requestBodyKey[:], "auth_len")
			AuthenticatedLengthKeyAEAD, _ := chacha20poly1305.New(GenerateChacha20Poly1305Key(AuthenticatedLengthKey))

			lengthAuth := &crypto.AEADAuthenticator{
				AEAD:                    AuthenticatedLengthKeyAEAD,
				NonceGenerator:          GenerateChunkNonce(s.requestBodyIV[:], uint32(aead.NonceSize())),
				AdditionalDataGenerator: crypto.GenerateEmptyBytes(),
			}
			sizeParser = NewAEADSizeParser(lengthAuth)
		}
		return crypto.NewAuthenticationWriter(auth, sizeParser, writer, request.Command.TransferType(request.Command.Vmess.TransferType()), padding), nil

	default:
		return nil, newError("invalid option: Security")
	}
}
