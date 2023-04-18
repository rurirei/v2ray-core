package mux

import (
	"v2ray.com/core/common"
	"v2ray.com/core/common/buffer"
	"v2ray.com/core/common/net"
	"v2ray.com/core/common/protocol"
	"v2ray.com/core/common/serial"
)

type Writer interface {
	buffer.Writer
}

type sessionWriterMetadata struct {
	target net.Address
	id     sessionID

	transferType protocol.TransferType
}

type sessionWriter struct {
	writer buffer.Writer

	followup bool
	hasError bool

	metadata sessionWriterMetadata
}

func (w *sessionWriter) WriteMultiBuffer(mb buffer.MultiBuffer) error {
	defer buffer.ReleaseMulti(mb)

	if mb == nil || mb.IsEmpty() {
		return w.writeMetaOnly()
	}

	for mb != nil && !mb.IsEmpty() {
		var chunk buffer.MultiBuffer

		switch w.metadata.transferType {
		case protocol.TransferTypeStream:
			mb, chunk = buffer.SplitSize(mb, 8*1024)
		case protocol.TransferTypePacket:
			mb0, b := buffer.SplitFirst(mb)
			mb = mb0
			chunk = buffer.MultiBuffer{b}
		default:
			return common.ErrUnknownNetwork
		}

		if err := w.writeMetaWithFrame(chunk); err != nil {
			return err
		}
	}

	return nil
}

func (w *sessionWriter) Close() error {
	meta := w.getNextMeta()
	meta.status = sessionStatusEnd
	if w.hasError {
		meta.option.Set(sessionOptionError)
	}

	b := buffer.New()
	_ = meta.WriteTo(b)

	_ = w.writer.WriteMultiBuffer(buffer.MultiBuffer{b})

	return nil
}

func (w *sessionWriter) writeMetaWithFrame(mb buffer.MultiBuffer) error {
	meta := w.getNextMeta()
	meta.option.Set(sessionOptionData)

	b := buffer.New()
	if err := meta.WriteTo(b); err != nil {
		return err
	}
	if _, err := serial.WriteUint16(b, uint16(mb.Len())); err != nil {
		return err
	}

	if len(mb)+1 > 64*1024*1024 {
		return newError("value too large")
	}
	sliceSize := len(mb) + 1
	mb0 := make(buffer.MultiBuffer, 0, sliceSize)
	mb0 = append(mb0, b)
	mb0 = append(mb0, mb...)
	return w.writer.WriteMultiBuffer(mb0)
}

func (w *sessionWriter) writeMetaOnly() error {
	meta := w.getNextMeta()

	b := buffer.New()
	if err := meta.WriteTo(b); err != nil {
		return err
	}

	return w.writer.WriteMultiBuffer(buffer.MultiBuffer{b})
}

func (w *sessionWriter) getNextMeta() frameMetadata {
	meta := frameMetadata{
		target: w.metadata.target,
		id:     w.metadata.id,
	}

	if w.followup {
		meta.status = sessionStatusKeep
	} else {
		w.followup = true
		meta.status = sessionStatusNew
	}

	return meta
}
