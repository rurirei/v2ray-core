package vmess

import (
	"v2ray.com/core/common/bitmask"
)

type RequestOption = bitmask.Byte

const (
	// RequestOptionChunkStream indicates request payload is chunked. Each chunk consists of length, authentication and payload.
	RequestOptionChunkStream RequestOption = 0x01

	// RequestOptionConnectionReuse indicates client side expects to reuse the connection.
	RequestOptionConnectionReuse RequestOption = 0x02

	RequestOptionChunkMasking RequestOption = 0x04

	RequestOptionGlobalPadding RequestOption = 0x08

	RequestOptionAuthenticatedLength RequestOption = 0x10
)

type ResponseOption = bitmask.Byte

const (
	ResponseOptionConnectionReuse ResponseOption = 0x01
)
