package fakedns

import (
	"github.com/v2fly/v2ray-core/v4/common"
)

type SniffHeader struct {
	domain string
}

func (h *SniffHeader) Protocol() string {
	return "fakedns"
}

func (h *SniffHeader) Domain() string {
	return h.domain
}

func SniffFakeDNS(b []byte) (*SniffHeader, error) {
	// h := &SniffHeader{}

	return nil, common.ErrNoClue
}
