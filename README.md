# v2sp

A high-performance, production-ready multi-core proxy backend designed for self-hosted panels. Built with official Xray-core and Hysteria2, featuring comprehensive traffic management, intelligent connection handling, and seamless API integration.

## Overview

v2sp is a multi-node, multi-core management backend that bridges self-hosted panels with Xray-core and Hysteria2. It provides enterprise-grade features including granular user limits, intelligent connection pooling, automated certificate management, and real-time traffic accounting—all through a simple, language-agnostic JSON API.

### Key Features

**Multi-Core Architecture**
- Xray-core: VLESS, VMess, Trojan, Shadowsocks with XTLS/Reality support
- Hysteria2: High-performance UDP-based protocol (subprocess mode)
- Automatic core selection based on node protocol type
- Independent node operation - single node failure doesn't affect others

**Traffic & Access Control**
- Per-user speed limiting with dynamic rate adjustment
- Device/IP-based connection limits with configurable thresholds
- Intelligent connection pooling with automatic oldest-connection eviction (FIFO)
- Protocol and domain-based traffic filtering and auditing

**Fault Tolerance & Stability**
- Independent node startup - failed nodes are skipped, others continue
- API communication failures handled gracefully with automatic retry
- No Panic on transient errors - service remains stable
- Existing connections unaffected by API outages

**Auto-Configuration**
- Configuration files auto-generated if missing (route.json, outbound.json)
- Certificate directories auto-created
- Hysteria2 node configs dynamically generated
- Default ACL rules for private IP blocking

**Advanced Features**
- Automated TLS certificate provisioning via ACME (HTTP/DNS modes)
- Graceful configuration reloading without service interruption
- Comprehensive logging with structured output
- Supports 50+ DNS providers for certificate validation

## Quick Start

### Installation

```bash
wget -N https://raw.githubusercontent.com/nsevo/v2sp-script/master/install.sh && bash install.sh
```

The installation script will:
1. Download the latest v2sp binary
2. Download Hysteria2 binary (for hysteria2 nodes)
3. Generate default configuration files
4. Set up systemd service
5. Create necessary directories (/etc/v2sp, /etc/v2sp/cert, /etc/v2sp/hy2)

### Configuration Generator

After installation, generate node configuration:

```bash
v2sp config
# Or run the script directly:
bash /path/to/initconfig.sh
```

The generator will:
1. Fetch node info from your panel API
2. Auto-detect protocol type and TLS requirements
3. Configure certificate settings based on protocol
4. Optionally enable automatic certificate provisioning

### Basic Usage

```bash
# Start service
systemctl start v2sp

# Check status
systemctl status v2sp

# View logs
journalctl -u v2sp -f

# Reload configuration
systemctl reload v2sp
```

## Architecture

### Multi-Core Design

v2sp uses a selector pattern to manage multiple proxy cores:

```
┌─────────────────────────────────────────────────────────┐
│                    Panel API (JSON)                      │
└──────────────────────────┬──────────────────────────────┘
                           │
              ┌────────────▼────────────┐
              │    Node Controller      │
              │  (Per-Node Management)  │
              └────────────┬────────────┘
                           │
         ┌─────────────────┼─────────────────┐
         │                 │                 │
    ┌────▼────┐      ┌────▼────┐      ┌────▼────┐
    │ Limiter │      │ Counter │      │Selector │
    │         │      │         │      │         │
    └─────────┘      └─────────┘      └────┬────┘
                                           │
                      ┌────────────────────┼────────────────────┐
                      │                    │                    │
               ┌──────▼──────┐     ┌──────▼──────┐     ┌──────▼──────┐
               │  Xray Core  │     │ Hysteria2   │     │  (Future)   │
               │  (in-proc)  │     │ (subprocess)│     │             │
               └─────────────┘     └─────────────┘     └─────────────┘
```

### Supported Protocols

| Core | Protocol | TLS Requirement | Description |
|------|----------|-----------------|-------------|
| Xray | vless | Optional/Reality | VLESS with XTLS support |
| Xray | vmess | Optional | VMess with AEAD encryption |
| Xray | trojan | Required | Trojan protocol |
| Xray | shadowsocks | None | Shadowsocks 2022 support |
| Hysteria2 | hysteria2 | Required | UDP-based, high-performance |

### Fault Tolerance

**Node Startup**
```
Starting nodes...
  Node 209: ERROR - cert request failed (skipped)
  Node 210: OK - vless started
  Node 211: OK - hysteria2 started
Summary: 2 success, 1 failed, 3 total
```

- Failed nodes are logged and skipped
- Successful nodes continue to operate
- Service only fails if ALL nodes fail

**Runtime Errors**
```
API call failed: 500 Internal Server Error
  -> Log error, retry next interval
  -> Existing connections unaffected
  -> User list preserved from last successful fetch
```

## Features

### Traffic Management

**Speed Limiting**
- Per-user bandwidth control (Mbps)
- Node-level bandwidth caps
- Dynamic speed adjustment based on traffic patterns
- Token bucket implementation for smooth rate limiting

**Connection Management**
- Per-user concurrent connection limits (`conn_limit`)
- Automatic oldest-connection eviction (FIFO) when limit exceeded
- Separate tracking for TCP and UDP connections
- Real-time connection counting

**Device Limiting**
- IP-based device counting (`device_limit`)
- Configurable simultaneous device limits
- IPv4/IPv6 support with address normalization
- Grace period for transient IP changes

### Hysteria2 Port Hopping

Port hopping helps bypass ISP UDP throttling by switching between multiple ports.

**How It Works**
```
Client ──UDP──► [20000-50000] ──iptables─► [:20000] ──► Hy2 Server
```

1. Hysteria2 server listens on the first port of the range
2. v2sp automatically creates iptables rules to redirect entire range
3. Client connects to any port in range, randomly hopping

**Port Configuration**

| `server_port` Value | Mode | v2sp Action |
|---------------------|------|-------------|
| `443` (integer) | Single port | Listen on 443 only |
| `"20000-50000"` (string) | Port hopping | Listen on 20000, iptables redirect 20000-50000 |

**Database Field**: Uses existing `port_range` field (e.g., `"20000-50000"`)
- If `port_range` is empty: single port mode using `backend_port`
- If `port_range` is set: port hopping mode, value passed as `server_port`

**API Response Examples**

Single port (no hopping):
```json
{
  "node_type": "hysteria2",
  "server_port": 443
}
```

Port hopping (automatic iptables):
```json
{
  "node_type": "hysteria2",
  "server_port": "20000-50000"
}
```

**Automatic Management**
- Rules created on node startup with unique comment tags
- Stale rules cleaned up on v2sp restart
- Rules removed when node is deleted
- Both IPv4 and IPv6 supported

**Generated iptables Rules**
```bash
# IPv4
iptables -t nat -A PREROUTING -p udp --dport 20000:50000 -j REDIRECT --to-ports 20000 -m comment --comment "v2sp-hy2:node-tag"
# IPv6
ip6tables -t nat -A PREROUTING -p udp --dport 20000:50000 -j REDIRECT --to-ports 20000 -m comment --comment "v2sp-hy2:node-tag"
```

### Certificate Management

**Supported Modes**

| Mode | Description | Auto-Renewal |
|------|-------------|--------------|
| `none` | No TLS (Reality, plain) | N/A |
| `file` | Manual certificate files | No |
| `http` | ACME HTTP-01 challenge | Yes |
| `dns` | ACME DNS-01 challenge | Yes |
| `self` | Self-signed certificate | No |

**Auto-Configuration**
```bash
# initconfig.sh automatically detects TLS requirements:
Node 209: hysteria2 -> jp.example.com (TLS required, auto-cert)
Node 210: vless (Reality, no cert needed)
Node 211: shadowsocks (no TLS)
```

## API Specification

### Overview

v2sp communicates with panel backends through HTTP/HTTPS JSON API.

### Authentication

All requests include:

| Parameter | Type | Description |
|-----------|------|-------------|
| `action` | string | API action (config, user, push, alive) |
| `node_id` | int | Node identifier |
| `token` | string | API authentication key |

### Required Response Fields

**Node Configuration** (`action=config`)

The API MUST return `node_type` field:

| node_type | Core | Description |
|-----------|------|-------------|
| `vless` | Xray | VLESS protocol |
| `vmess` | Xray | VMess protocol |
| `trojan` | Xray | Trojan protocol |
| `shadowsocks` | Xray | Shadowsocks protocol |
| `hysteria2` | Hysteria2 | Hysteria2 protocol |

**Example Response**
```json
{
  "node_type": "vless",
  "server_port": 443,
  "server_name": "example.com",
  "tls": 1,
  "network": "tcp"
}
```

**User List** (`action=user`)

```json
{
  "users": [
    {
      "id": 1001,
      "uuid": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
      "speed_limit": 100,
      "device_limit": 3,
      "conn_limit": 50
    }
  ]
}
```

| Field | Type | Description |
|-------|------|-------------|
| `id` | int | Unique user ID |
| `uuid` | string | User UUID |
| `speed_limit` | int | Speed limit in Mbps (0 = unlimited) |
| `device_limit` | int | Max devices (0 = unlimited) |
| `conn_limit` | int | Max connections (0 = unlimited) |

## Configuration

### Example Configuration

```json
{
  "Log": {
    "Level": "error",
    "Output": ""
  },
  "Cores": [
    {
      "Type": "xray",
      "Log": {
        "Level": "error",
        "ErrorPath": "/etc/v2sp/error.log"
      },
      "OutboundConfigPath": "/etc/v2sp/custom_outbound.json",
      "RouteConfigPath": "/etc/v2sp/route.json"
    },
    {
      "Type": "hysteria2",
      "Log": {
        "Level": "error",
        "ErrorPath": "/etc/v2sp/hy2_error.log"
      },
      "BinaryPath": "/usr/local/bin/hysteria",
      "ConfigDir": "/etc/v2sp/hy2"
    }
  ],
  "Nodes": [
    {
      "ApiHost": "https://panel.example.com/api",
      "ApiKey": "your_api_key",
      "NodeID": 1,
      "Timeout": 30,
      "ListenIP": "0.0.0.0",
      "SendIP": "0.0.0.0",
      "CertConfig": {
        "CertMode": "http",
        "CertDomain": "node1.example.com",
        "CertFile": "/etc/v2sp/cert/node1.example.com.crt",
        "KeyFile": "/etc/v2sp/cert/node1.example.com.key"
      }
    }
  ]
}
```

### Configuration Reference

**Xray Core Options**

| Field | Type | Description |
|-------|------|-------------|
| `Type` | string | Must be "xray" |
| `OutboundConfigPath` | string | Path to outbound config (auto-created if missing) |
| `RouteConfigPath` | string | Path to route config (auto-created if missing) |
| `AssetPath` | string | Path to geoip.dat and geosite.dat |

**Hysteria2 Core Options**

| Field | Type | Description |
|-------|------|-------------|
| `Type` | string | Must be "hysteria2" |
| `BinaryPath` | string | Path to hysteria binary (default: /usr/local/bin/hysteria) |
| `ConfigDir` | string | Directory for node configs (default: /etc/v2sp/hy2) |

**Certificate Options**

| Field | Type | Description |
|-------|------|-------------|
| `CertMode` | string | none, file, http, dns, self |
| `CertDomain` | string | Domain for certificate |
| `CertFile` | string | Certificate file path |
| `KeyFile` | string | Private key file path |
| `Provider` | string | DNS provider (for dns mode) |
| `DNSEnv` | object | DNS provider credentials |

### Auto-Created Files

When v2sp starts, it automatically creates missing configuration files:

**route.json** (default routing rules)
```json
{
  "domainStrategy": "AsIs",
  "rules": [
    {"outboundTag": "block", "ip": ["geoip:private"]},
    {"outboundTag": "block", "ip": ["127.0.0.0/8", "10.0.0.0/8", ...]},
    {"outboundTag": "IPv4_out", "network": "udp,tcp"}
  ]
}
```

**custom_outbound.json** (default outbounds)
```json
[
  {"tag": "IPv4_out", "protocol": "freedom"},
  {"tag": "IPv6_out", "protocol": "freedom"},
  {"tag": "block", "protocol": "blackhole"}
]
```

**Hysteria2 node configs** (per-node YAML)
```yaml
listen: ":443"
tls:
  cert: /etc/v2sp/cert/domain.crt
  key: /etc/v2sp/cert/domain.key
auth:
  type: userpass
  userpass:
    uuid1: uuid1
    uuid2: uuid2
acl:
  inline:
    - reject(geoip:private)
    - direct(all)
```

## Building from Source

### Prerequisites

- Go 1.21 or later
- Git

### Build Commands

```bash
# Linux AMD64
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w" -o v2sp-linux-amd64 ./main.go

# Linux ARM64
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-s -w" -o v2sp-linux-arm64 ./main.go
```

## Troubleshooting

### Common Issues

**Single node failure crashes service**
- Update to v1.4.3+ which includes independent node operation
- Failed nodes are now skipped, others continue

**API 500 errors causing service restart**
- Update to v1.4.3+ which includes fault-tolerant API handling
- Transient errors are logged and retried automatically

**Certificate request fails**
- Ensure `CertFile` and `KeyFile` paths are specified (even for http mode)
- Create `/etc/v2sp/cert/` directory
- Verify port 80 is accessible (for http mode)

**Hysteria2 node not starting**
- Ensure `/usr/local/bin/hysteria` exists and is executable
- Check `/etc/v2sp/hy2/` directory is writable
- Verify TLS certificate is valid

### Debug Mode

```json
{
  "Log": {
    "Level": "debug"
  }
}
```

## Version History

### v1.4.4
- Added Hysteria2 port hopping support
- Automatic iptables rule management (create, cleanup, restart-safe)
- IPv4 and IPv6 port hopping support
- Stale rule cleanup on service start

### v1.4.3
- Independent node operation - single node failure doesn't affect others
- Removed Panic calls, replaced with graceful error handling
- API communication failures handled with automatic retry
- Improved stability for multi-node deployments

### v1.4.2
- Fixed Hysteria2 config filename sanitization
- Support special characters in node tags

### v1.4.1
- Fixed node type validation for hysteria2
- Allow empty Core field in node options

### v1.4.0
- Added Hysteria2 support (subprocess mode)
- Multi-core architecture with automatic core selection
- Auto-create missing configuration files
- Certificate path auto-configuration in initconfig.sh

## Credits

**Core Projects**
- [XTLS/Xray-core](https://github.com/XTLS/Xray-core) - High-performance proxy platform
- [apernet/hysteria](https://github.com/apernet/hysteria) - Hysteria2 protocol implementation

**Inspiration**
- [XrayR](https://github.com/XrayR-project/XrayR) - Xray backend reference
- [V2bX](https://github.com/wyx2685/V2bX) - Multi-protocol backend

**Infrastructure**
- [go-acme/lego](https://github.com/go-acme/lego) - ACME client library
- [spf13/cobra](https://github.com/spf13/cobra) - CLI framework

## License

This project is provided as-is for educational and self-hosting purposes.

## Links

- Repository: [github.com/nsevo/v2sp](https://github.com/nsevo/v2sp)
- Scripts: [github.com/nsevo/v2sp-script](https://github.com/nsevo/v2sp-script)
- Issues: [GitHub Issues](https://github.com/nsevo/v2sp/issues)
