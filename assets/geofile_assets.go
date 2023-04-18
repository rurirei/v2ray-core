package assets

import (
	"os"
	"path/filepath"

	"v2ray.com/core/common/io"
)

var (
	GeoReadOption = struct {
		WorkingPath, IPFilePath, IPFileSuffix, SiteFilePath, SiteFileSuffix string
	}{
		WorkingPath:    getGeoExecutableDir(),
		IPFilePath:     "geoip",
		IPFileSuffix:   ".txt",
		SiteFilePath:   "geosite",
		SiteFileSuffix: ".txt",
	}
)

func getGeoExecutableDir() string {
	exec, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Dir(exec)
}

type GeoFileReaderFunc = func(string) (io.ReadCloser, error)

var GeoFileReader = func(path string) (io.ReadCloser, error) {
	return os.Open(path)
}

func GeoIPFileReader(filename string) (io.ReadCloser, error) {
	return GeoFileReader(filepath.Join(GeoReadOption.WorkingPath, GeoReadOption.IPFilePath, filename+GeoReadOption.IPFileSuffix))
}

func GeoSiteFileReader(filename string) (io.ReadCloser, error) {
	return GeoFileReader(filepath.Join(GeoReadOption.WorkingPath, GeoReadOption.SiteFilePath, filename+GeoReadOption.SiteFileSuffix))
}
