package vmess

import (
	"runtime"

	"golang.org/x/sys/cpu"
)

type Security int32

const (
	Security_UNKNOWN           Security = 0
	Security_LEGACY            Security = 1
	Security_AUTO              Security = 2
	Security_AES_128_GCM       Security = 3
	Security_CHACHA20_POLY1305 Security = 4
	Security_NONE              Security = 5
	Security_ZERO              Security = 6
)

var (
	Security_Name = map[int32]Security{
		0: Security_UNKNOWN,
		1: Security_LEGACY,
		2: Security_AUTO,
		3: Security_AES_128_GCM,
		4: Security_CHACHA20_POLY1305,
		5: Security_NONE,
		6: Security_ZERO,
	}
)

var (
	hasGCMAsmAMD64 = cpu.X86.HasAES && cpu.X86.HasPCLMULQDQ
	hasGCMAsmARM64 = cpu.ARM64.HasAES && cpu.ARM64.HasPMULL
	// Keep in sync with crypto/aes/cipher_s390x.go.
	hasGCMAsmS390X = cpu.S390X.HasAES && cpu.S390X.HasAESCBC && cpu.S390X.HasAESCTR && (cpu.S390X.HasGHASH || cpu.S390X.HasAESGCM)

	hasAESGCMHardwareSupport = runtime.GOARCH == "amd64" && hasGCMAsmAMD64 || runtime.GOARCH == "arm64" && hasGCMAsmARM64 || runtime.GOARCH == "s390x" && hasGCMAsmS390X
)

func AutoSecurity() Security {
	if hasAESGCMHardwareSupport {
		return Security_AES_128_GCM
	}
	return Security_CHACHA20_POLY1305
}
