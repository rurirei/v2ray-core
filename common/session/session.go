package session

import (
	"math/rand"

	"v2ray.com/core/common/net"
)

// ID of a session.
type ID uint32

// NewID generates a new ID. The generated ID is high likely to be unique, but not cryptographically secure.
// The generated ID will never be 0.
func NewID() ID {
	for {
		id := ID(rand.Uint32())
		if id != 0 {
			return id
		}
	}
}

// Inbound is the metadata of an inbound connection.
type Inbound struct {
	// Source address of the inbound connection.
	Source net.Address
	// Gateway address
	Gateway net.Address
	// Tag of the inbound proxy that handles the connection.
	Tag string
}

type Mux struct {
	// Enabled show the mux outbound is used
	Enabled bool
}
