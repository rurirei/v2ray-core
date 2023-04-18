package setting

type ListenName byte

const (
	ListenHTTP ListenName = iota
	ListenSocks
	ListenDokodemo
	ListenVmess
)

type ProxyName byte

const (
	ProxyFreedom ProxyName = iota
	ProxyHTTP
	ProxyVmess
	ProxyDNS
	ProxyBlock
)

type TransportName byte

const (
	TransportTCP TransportName = iota
	TransportHTTP
)

type SecurityName byte

const (
	SecurityNone SecurityName = iota
	SecurityTLS
)
