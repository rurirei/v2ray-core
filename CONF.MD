## Conf

you could see this all at common/settings/conf/conf.go

### client

```json
{
  "dns": {
    "hosts":[
      {
        "domain": "dns.google",
        "ips6": [
          "2001:4860:4860::8888"
        ],
        "ips4": [
          "8.8.8.8"
        ]
      }
    ],
    "fake":[
      {
        "tag": "fakedns",
        "cidr6": "fc00::/18",
        "cidr4": "198.18.0.0/15"
      }
    ],
    "doh":[
      {
        "tag": "dohdns",
        "url": "https://dns.google/dns-query"
      }
    ]
  },
  "inbounds": {
    "http": [
      {
        "tag": "http",
        "network": [
          "tcp"
        ],
        "listen": "127.0.0.1:1080"
      }
    ],
    "socks": [
      {
        "tag": "socks",
        "network": [
          "tcp",
          "udp"
        ],
        "listen": "127.0.0.1:1080"
      }
    ],
    "tun": [
      {
        "tag": "tun",
        "network": [
          "tcp",
          "udp"
        ]
      }
    ]
  },
  "outbounds": {
    "block": [
      {
        "tag": "block"
      }
    ],
    "dns": [
      {
        "tag": "dns"
      }
    ],
    "freedom": [
      {
        "tag": "freedom"
      }
    ],
    "http": [
      {
        "tag": "http",
        "target": "1.2.3.4:0",
        "mux": false/true
      }
    ],
    "shadowsocks": [
      {
        "tag": "shadowsocks",
        "target": "1.2.3.4:0",
        "user": {
          "security": "aes_128_gcm/aes_256_gcm/..",
          "password": "password"
        },
        "tcp": {
          "tls": {
            "serverName": "domain"
          }
        },
        "mux": false/true
      }
    ],
    "socks": [
      {
        "tag": "socks",
        "target": "1.2.3.4:0",
        "mux": false/true
      }
    ],
    "trojan": [
      {
        "tag": "trojan",
        "target": "1.2.3.4:0",
        "user": {
          "password": "password"
        },
        "tcp": {
          "tls": {
            "serverName": "domain"
          }
        },
        "mux": false/true
      }
    ],
    "vmess": [
      {
        "tag": "vmess",
        "target": "1.2.3.4:0",
        "user": {
          "security": "auto/..",
          "uuid": "uuid"
        },
        "tcp": {
          "tls": {
            "serverName": "domain"
          }
        },
        "mux": false/true
      }
    ]
  },
  "rules": {
    "dns": [
      {
        "condition": [
          {
            "name": "domains",
            "length": "full/sub/regex",
            "string": [
              "dns.google/google/goo.*"
            ]
          },
          {
            "name": "inboundTag",
            "length": "full/sub/regex",
            "string": [
              "tun-in/tun/tun.*"
            ]
          }
        ],
        "outboundTag": "fake/doh/.."
      }
    ],
    "outbound": [
      {
        "condition": [
          {
            "name": "srcNetwork/dstNetwork",
            "length": "full",
            "string": [
              "tcp/udp"
            ]
          },
          {
            "name": "srcIP/dstIP",
            "length": "full",
            "string": [
              "1.2.3.4",
              "geoip:private/.."
            ]
          },
          {
            "name": "dstDomain",
            "length": "full/sub/regex",
            "string": [
              "dns.google/google/goo.*"
            ]
          },
          {
            "name": "srcPort/dstPort",
            "length": "full",
            "string": [
              "53"
            ]
          },
          {
            "name": "inboundTag",
            "length": "full/sub/regex",
            "string": [
              "tun-in/tun/tun.*"
            ]
          }
        ],
        "outboundTag": "http/socks/.."
      }
    ]
  }
}
```

### server

```json
{
  "inbounds": {
    "shadowsocks": [
      {
        "tag": "shadowsocks",
        "network": [
          "tcp",
          "udp"
        ],
        "listen": "127.0.0.1:1080",
        "user": {
          "security": "aes_128_gcm/aes_256_gcm/..",
          "password": "password"
        },
        "tcp": {
          "tls": {
            "serverName": "domain",
            "certificate": "certificate",
            "key": "key"
          }
        },
        "mux": false/true
      }
    ],
    "vmess": [
      {
        "tag": "vmess",
        "network": [
          "tcp",
          "udp"
        ],
        "listen": "127.0.0.1:1080",
        "user": {
          "uuid": "uuid"
        },
        "tcp": {
          "tls": {
            "serverName": "domain",
            "certificate": "certificate",
            "key": "key"
          }
        },
        "mux": false/true
      }
    ]
  },
  "outbounds": {
    "freedom": [
      {
        "tag": "freedom"
      }
    ]
  },
  "rules": {
    "outbounds": [
      {
        "condition": [
          {
            "name": "srcNetwork",
            "length": "full",
            "string": [
              "tcp",
              "udp"
            ]
          }
        ],
        "outboundTag": "freddom"
      }
    ]
  }
}
```
