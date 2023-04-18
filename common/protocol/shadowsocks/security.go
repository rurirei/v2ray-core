package shadowsocks

type Security int32

const (
	Security_UNKNOWN           Security = 0
	Security_AES_128_GCM       Security = 1
	Security_AES_256_GCM       Security = 2
	Security_CHACHA20_POLY1305 Security = 3
	Security_NONE              Security = 4
)
