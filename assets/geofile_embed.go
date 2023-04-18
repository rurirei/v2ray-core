package assets

import (
	"embed"
	"path/filepath"

	"v2ray.com/core/common/io"
)

var (
	//go:embed geoip geosite
	localGeoFSFile embed.FS
)

var GeoFSFileReader = func(path string) (io.ReadCloser, error) {
	return localGeoFSFile.Open(path)
}

func GeoIPFSFileReader(filename string) (io.ReadCloser, error) {
	return GeoFSFileReader(filepath.ToSlash(filepath.Join(GeoReadOption.IPFilePath, filename+GeoReadOption.IPFileSuffix)))
}

func GeoSiteFSFileReader(filename string) (io.ReadCloser, error) {
	return GeoFSFileReader(filepath.ToSlash(filepath.Join(GeoReadOption.SiteFilePath, filename+GeoReadOption.SiteFileSuffix)))
}
