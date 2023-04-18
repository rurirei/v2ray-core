package io

import (
	"io"
)

type (
	Reader = io.Reader
	Writer = io.Writer
	Closer = io.Closer

	ReadWriter      = io.ReadWriter
	ReadCloser      = io.ReadCloser
	ReadWriteCloser = io.ReadWriteCloser

	WriteCloser = io.WriteCloser
)

var (
	EOF              = io.EOF
	ErrUnexpectedEOF = io.ErrUnexpectedEOF
	ErrClosedPipe    = io.ErrClosedPipe

	ReadFull = io.ReadFull
	ReadAll  = io.ReadAll

	LimitReader = io.LimitReader
)
