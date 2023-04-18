package dns

import (
	"strings"
)

// TrimFqdn denormalize domain make sure it not ends with '.'
func TrimFqdn(domain string) string {
	return strings.TrimSuffix(domain, ".")
}

// GetFqdn normalize domain make sure it ends with '.'
func GetFqdn(domain string) string {
	if IsFqdn(domain) {
		return domain
	}
	return domain + "."
}

// IsFqdn check a fqdn
func IsFqdn(domain string) bool {
	return strings.HasSuffix(domain, ".")
}
