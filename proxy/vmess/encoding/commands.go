package encoding

import (
	"encoding/binary"

	"v2ray.com/core/common/buffer"
	"v2ray.com/core/common/io"
	"v2ray.com/core/common/protocol/vmess"
)

var (
	errCommandTypeMismatch = newError("ResponseCommand type mismatch.")
	errUnknownCommand      = newError("Unknown command.")
	errCommandTooLarge     = newError("ResponseCommand too large.")

	errInsufficientLength = newError("insufficient length.")
)

func MarshalCommand(command vmess.ResponseCommand, writer io.Writer) error {
	var cmdID byte = 1

	buf := buffer.New()
	defer buf.Release()

	err := marshal(command, buf)
	if err != nil {
		return err
	}

	auth := Authenticate(buf.Bytes())
	length := buf.Len() + 4
	if length > 255 {
		return errCommandTooLarge
	}

	_, _ = writer.Write([]byte{cmdID, byte(length), byte(auth >> 24), byte(auth >> 16), byte(auth >> 8), byte(auth)})
	_, _ = buf.ReadToWriter(writer, true)
	return nil
}

func UnmarshalCommand(cmdID byte, data []byte) (vmess.ResponseCommand, error) {
	if len(data) <= 4 {
		return vmess.ResponseCommand{}, errInsufficientLength
	}
	expectedAuth := Authenticate(data[4:])
	actualAuth := binary.BigEndian.Uint32(data[:4])
	if expectedAuth != actualAuth {
		return vmess.ResponseCommand{}, newError("invalid auth")
	}

	switch cmdID {
	case 1:
		return unmarshal(data[4:])
	default:
		return vmess.ResponseCommand{}, errUnknownCommand
	}
}

func marshal(_ vmess.ResponseCommand, _ io.Writer) error {
	return errUnknownCommand
}

func unmarshal(_ []byte) (vmess.ResponseCommand, error) {
	return vmess.ResponseCommand{}, errUnknownCommand
}
