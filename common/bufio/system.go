package bufio

import (
	"bufio"

	"v2ray.com/core/common/io"
)

const (
	Size = 2 * 1024
)

type (
	Writer  = bufio.Writer
	Reader  = bufio.Reader
	Scanner = bufio.Scanner
)

var (
	NewWriter = func(w io.Writer) *Writer {
		return bufio.NewWriterSize(w, Size)
	}

	NewReader = func(r io.Reader) *Reader {
		return bufio.NewReaderSize(r, Size)
	}

	NewScanner = bufio.NewScanner
)
