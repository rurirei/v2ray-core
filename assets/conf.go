package assets

import (
	"os"
	"path/filepath"

	"v2ray.com/core/common/io"
)

var (
	ConfReadOption = struct {
		WorkingPath, FilePath, Filename, FileSuffix string
	}{
		WorkingPath: getConfExecutableDir(),
		FilePath:    "conf",
		Filename:    "conf",
		FileSuffix:  ".json",
	}
)

func getConfExecutableDir() string {
	exec, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Dir(exec)
}

type ConfFileReaderFunc = func() (io.ReadCloser, error)

var ConfFileReader = func(path string) (io.ReadCloser, error) {
	return os.Open(path)
}

func ConfFileFileReader() (io.ReadCloser, error) {
	return ConfFileReader(filepath.Join(ConfReadOption.WorkingPath, ConfReadOption.FilePath, ConfReadOption.Filename+ConfReadOption.FileSuffix))
}
