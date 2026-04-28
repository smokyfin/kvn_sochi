# KVN Sochi architecture

## Strict traffic flow

Default:

```text
TUN → hev-socks5-tunnel → Arti → xray-core → Internet
```

`skip_arti=true`:

```text
TUN → hev-socks5-tunnel → xray-core → Internet
```

## Security boundaries

- Flutter UI never opens network listeners.
- Native services own VPN permission and TUN lifecycle.
- Go core stores generated config, IPC socket, and tokens inside the private app directory with `0700`/`0600` permissions.
- IPC is a Unix socket plus a random token generated per process start.
- SOCKS endpoints are loopback-only, random port, username/password protected.
- Disconnect cancels context, stops supervised processes in reverse order, closes DNS/TUN owners, then removes generated runtime files.

## Config parsing

The parser accepts the full JSON from `https://incss.ru/vless.conf` but only keeps:

- `bridge_rsa_id`
- `bridge_ed25519_id`
- `doh_server`
- `doh_server_ip`
- `outbounds`
- `skip_arti`

Everything else is ignored.

## Native dependencies

The scaffold expects real native binaries for:

- `xray-core` latest from git
- `arti`/`arti-client` >= 0.41
- `hev-socks5-tunnel`

The Go lifecycle/orchestration code is wired for these processes, but release packaging must add per-ABI binaries and app-store-compliant iOS embedding/signing.
