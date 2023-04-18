package conf

import (
	"encoding/json"

	"v2ray.com/core/assets"
	"v2ray.com/core/common/io"
)

type config struct {
	Dns struct {
		Hosts []struct {
			Domain string   `json:"domain,omitempty"`
			Ips6   []string `json:"ips6,omitempty"`
			Ips4   []string `json:"ips4,omitempty"`
		} `json:"hosts,omitempty"`
		Fake []struct {
			Tag   string `json:"tag,omitempty"`
			Cidr6 string `json:"cidr6,omitempty"`
			Cidr4 string `json:"cidr4,omitempty"`
		} `json:"fake,omitempty"`
		Doh []struct {
			Tag string `json:"tag,omitempty"`
			Url string `json:"url,omitempty"`
		} `json:"doh,omitempty"`
	} `json:"dns,omitempty"`
	Inbounds struct {
		Dokodemo []struct {
			Tag     string   `json:"tag,omitempty"`
			Network []string `json:"network,omitempty"`
			Listen  string   `json:"listen,omitempty"`
			Mux     bool     `json:"mux,omitempty"`
		} `json:"dokodemo,omitempty"`
		Http []struct {
			Tag     string   `json:"tag,omitempty"`
			Network []string `json:"network,omitempty"`
			Listen  string   `json:"listen,omitempty"`
			Mux     bool     `json:"mux,omitempty"`
		} `json:"http,omitempty"`
		Shadowsocks []struct {
			Tag     string   `json:"tag,omitempty"`
			Network []string `json:"network,omitempty"`
			Listen  string   `json:"listen,omitempty"`
			User    struct {
				Security string `json:"security,omitempty"`
				Password string `json:"password,omitempty"`
			} `json:"user,omitempty"`
			Tcp struct {
				Tls struct {
					ServerName  string `json:"serverName,omitempty"`
					Certificate string `json:"certificate,omitempty"`
					Key         string `json:"key,omitempty"`
				} `json:"tls,omitempty"`
			} `json:"tcp,omitempty"`
			Websocket struct {
				Path string `json:"path,omitempty"`
				Tls  struct {
					ServerName  string `json:"serverName,omitempty"`
					Certificate string `json:"certificate,omitempty"`
					Key         string `json:"key,omitempty"`
				} `json:"tls,omitempty"`
			} `json:"websocket,omitempty"`
			Mux bool `json:"mux,omitempty"`
		} `json:"shadowsocks,omitempty"`
		Socks []struct {
			Tag     string   `json:"tag,omitempty"`
			Network []string `json:"network,omitempty"`
			Listen  string   `json:"listen,omitempty"`
			Resp    string   `json:"resp,omitempty"`
			Mux     bool     `json:"mux,omitempty"`
		} `json:"socks,omitempty"`
		Tun []struct {
			Tag     string   `json:"tag,omitempty"`
			Network []string `json:"network,omitempty"`
		} `json:"tun,omitempty"`
		Vmess []struct {
			Tag     string   `json:"tag,omitempty"`
			Network []string `json:"network,omitempty"`
			Listen  string   `json:"listen,omitempty"`
			User    struct {
				UUID string `json:"uuid,omitempty"`
			} `json:"user,omitempty"`
			Tcp struct {
				Tls struct {
					ServerName  string `json:"serverName,omitempty"`
					Certificate string `json:"certificate,omitempty"`
					Key         string `json:"key,omitempty"`
				} `json:"tls,omitempty"`
			} `json:"tcp,omitempty"`
			Websocket struct {
				Path string `json:"path,omitempty"`
				Tls  struct {
					ServerName  string `json:"serverName,omitempty"`
					Certificate string `json:"certificate,omitempty"`
					Key         string `json:"key,omitempty"`
				} `json:"tls,omitempty"`
			} `json:"websocket,omitempty"`
			Mux bool `json:"mux,omitempty"`
		} `json:"vmess,omitempty"`
	} `json:"inbounds,omitempty"`
	Outbounds struct {
		Block []struct {
			Tag string `json:"tag,omitempty"`
		} `json:"block,omitempty"`
		Dns []struct {
			Tag string `json:"tag,omitempty"`
		} `json:"dns,omitempty"`
		Freedom []struct {
			Tag string `json:"tag,omitempty"`
		} `json:"freedom,omitempty"`
		Http []struct {
			Tag    string `json:"tag,omitempty"`
			Target string `json:"target,omitempty"`
			Mux    bool   `json:"mux,omitempty"`
		} `json:"http,omitempty"`
		Shadowsocks []struct {
			Tag    string `json:"tag,omitempty"`
			Target string `json:"target,omitempty"`
			User   struct {
				Security string `json:"security,omitempty"`
				Password string `json:"password,omitempty"`
			} `json:"user,omitempty"`
			Tcp struct {
				Tls struct {
					ServerName string `json:"serverName,omitempty"`
				} `json:"tls,omitempty"`
			} `json:"tcp,omitempty"`
			Websocket struct {
				Path string `json:"path,omitempty"`
				Tls  struct {
					ServerName string `json:"serverName,omitempty"`
				} `json:"tls,omitempty"`
			} `json:"websocket,omitempty"`
			Forward struct {
				Tag string `json:"tag,omitempty"`
				Tls struct {
					ServerName string `json:"serverName,omitempty"`
				} `json:"tls,omitempty"`
			} `json:"forward,omitempty"`
			Mux bool `json:"mux,omitempty"`
		} `json:"shadowsocks,omitempty"`
		Socks []struct {
			Tag    string `json:"tag,omitempty"`
			Target string `json:"target,omitempty"`
			Mux    bool   `json:"mux,omitempty"`
		} `json:"socks,omitempty"`
		Tor []struct {
			Tag     string `json:"tag,omitempty"`
			Forward struct {
				Tag string `json:"tag,omitempty"`
				Tls struct {
					ServerName string `json:"serverName,omitempty"`
				} `json:"tls,omitempty"`
			} `json:"forward,omitempty"`
			Mux bool `json:"mux,omitempty"`
		} `json:"tor,omitempty"`
		Trojan []struct {
			Tag    string `json:"tag,omitempty"`
			Target string `json:"target,omitempty"`
			User   struct {
				Password string `json:"password,omitempty"`
			} `json:"user,omitempty"`
			Tcp struct {
				Tls struct {
					ServerName string `json:"serverName,omitempty"`
				} `json:"tls,omitempty"`
			} `json:"tcp,omitempty"`
			Websocket struct {
				Path string `json:"path,omitempty"`
				Tls  struct {
					ServerName string `json:"serverName,omitempty"`
				} `json:"tls,omitempty"`
			} `json:"websocket,omitempty"`
			Forward struct {
				Tag string `json:"tag,omitempty"`
				Tls struct {
					ServerName string `json:"serverName,omitempty"`
				} `json:"tls,omitempty"`
			} `json:"forward,omitempty"`
			Mux bool `json:"mux,omitempty"`
		} `json:"trojan,omitempty"`
		Vmess []struct {
			Tag    string `json:"tag,omitempty"`
			Target string `json:"target,omitempty"`
			User   struct {
				Security string `json:"security,omitempty"`
				UUID     string `json:"uuid,omitempty"`
			} `json:"user,omitempty"`
			Tcp struct {
				Tls struct {
					ServerName string `json:"serverName,omitempty"`
				} `json:"tls,omitempty"`
			} `json:"tcp,omitempty"`
			Websocket struct {
				Path string `json:"path,omitempty"`
				Tls  struct {
					ServerName string `json:"serverName,omitempty"`
				} `json:"tls,omitempty"`
			} `json:"websocket,omitempty"`
			Forward struct {
				Tag string `json:"tag,omitempty"`
				Tls struct {
					ServerName string `json:"serverName,omitempty"`
				} `json:"tls,omitempty"`
			} `json:"forward,omitempty"`
			Mux bool `json:"mux,omitempty"`
		} `json:"vmess,omitempty"`
	} `json:"outbounds,omitempty"`
	Rules struct {
		Dns []struct {
			Condition []struct {
				Name   string   `json:"name,omitempty"`
				Length string   `json:"length,omitempty"`
				String []string `json:"string,omitempty"`
			} `json:"condition,omitempty"`
			OutboundTag string `json:"outboundTag,omitempty"`
		} `json:"dns,omitempty"`
		Outbound []struct {
			Condition []struct {
				Name   string   `json:"name,omitempty"`
				Length string   `json:"length,omitempty"`
				String []string `json:"string,omitempty"`
			} `json:"condition,omitempty"`
			OutboundTag string `json:"outboundTag,omitempty"`
		} `json:"outbound,omitempty"`
	} `json:"rules,omitempty"`
}

func unmarshal() (config, error) {
	b, err := ReadBytes(assets.ConfFileFileReader)
	if err != nil {
		return config{}, newError("failed to read file").WithError(err)
	}

	c := config{}
	if err := json.Unmarshal(b, &c); err != nil {
		return config{}, newError("failed to unmarshal bytes").WithError(err)
	}
	return c, nil
}

func ReadBytes(reader assets.ConfFileReaderFunc) ([]byte, error) {
	file, err := reader()
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = file.Close()
	}()

	return io.ReadAll(file)
}

//go:generate go run v2ray.com/core/common/errors/errorgen
