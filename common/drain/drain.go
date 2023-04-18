package drain

import (
	"v2ray.com/core/common/io"
)

type Drainer interface {
	AcknowledgeReceive(size int)
	Drain(reader io.Reader) error
}

//go:generate go run v2ray.com/core/common/errors/errorgen
