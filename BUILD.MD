## Build

1. place the `geoip` txt file into assets/geoip/, or `geosite` txt file into assets/geosite/
2. run `go build`

## Run

1. place the `conf.conf` json file into the working directory
2. run the bin file by `go build`, then ok
3. you could also call conf.Loads() like this
```go
import "v2ray.com/core/common/settings/conf"

func init() {
    conf.Loads()
}
```
