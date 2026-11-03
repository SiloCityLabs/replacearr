# OpenVPN Proxy Plugin

External `network_proxy` plugin that advertises a local SOCKS5 URL (`socks5://127.0.0.1:1080` by default) and optionally starts OpenVPN when a config path and binary are available.

## Settings

| Field | Default | Notes |
| --- | --- | --- |
| `configPath` | _(empty)_ | Path to an `.ovpn` file. Empty = advertise SOCKS only. |
| `socksListen` | `127.0.0.1:1080` | Local listen address returned by `ProxyURL()`. |
| `openvpnBinary` | `openvpn` | Binary name/path. Startup is best-effort. |

## Capabilities

OpenVPN needs the ability to create TUN/TAP devices. In containers grant:

```bash
--cap-add=NET_ADMIN --device /dev/net/tun
```

Without `CAP_NET_ADMIN`, configure a sidecar that already exposes a SOCKS port and set `socksListen` / leave `configPath` empty. The built-in stub `builtin-openvpn-proxy` exposes the same `ProxyURL` from settings without launching OpenVPN (useful in tests).

## Build

```bash
CGO_ENABLED=0 go build -o bin/plugins/openvpn-proxy ./examples/plugins/openvpn-proxy
```
