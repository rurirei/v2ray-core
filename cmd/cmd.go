package cmd

import (
	"v2ray.com/core/common/setting/conf"
)

func init() {
	if err := conf.Loads(); err != nil {
		panic(err)
	}
}
