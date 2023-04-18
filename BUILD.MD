## Build

1. place the `geoip` txt file into assets/geoip/, or `geosite` txt file into assets/geosite/
2. run `go build`

## Run

1. place the `conf.conf` json file into the working directory
2. call conf.Loads() like this

```go
import "v2ray.com/core/common/settings/conf"

func init() {
    conf.Loads()
}
```
