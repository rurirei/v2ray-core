package geofile

import (
	"net/netip"

	"v2ray.com/core/assets"
	"v2ray.com/core/common/bufio"
)

func LoadIPCIDR(s []string) ([]netip.Prefix, error) {
	cidrs := make([]netip.Prefix, 0, len(s))

	for _, cidr := range s {
		iprange, err := netip.ParsePrefix(cidr)
		if err != nil {
			return nil, err
		}

		cidrs = append(cidrs, iprange)
	}

	return cidrs, nil
}

func LoadIPStr(filename string) ([]string, error) {
	return ReadLine(filename, assets.GeoIPFSFileReader)
}

func LoadSite(filename string) ([]string, error) {
	return ReadLine(filename, assets.GeoSiteFSFileReader)
}

func ReadLine(filename string, reader assets.GeoFileReaderFunc) ([]string, error) {
	file, err := reader(filename)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = file.Close()
	}()

	return bufio.ReadLine(file)
}

//go:generate go run v2ray.com/core/common/errors/errorgen
