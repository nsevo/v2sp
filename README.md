# v2sp

A high-performance, production-ready Xray backend designed for self-hosted proxy panels. Built with official Xray-core, featuring comprehensive traffic management, intelligent connection handling, and seamless API integration.

## Overview

v2sp is a multi-node management backend that bridges self-hosted panels with Xray-core. It provides enterprise-grade features including granular user limits, intelligent connection pooling, automated certificate management, and real-time traffic accounting—all through a simple, language-agnostic JSON API.

### Key Features

**Core Capabilities**
- Based on official Xray-core v1.251201.0 with full protocol support (VLESS, VMess, Trojan, Shadowsocks)
- Multi-node management in a single process with independent configurations
- Real-time traffic accounting with configurable reporting intervals
- Automated TLS certificate provisioning via ACME with multiple DNS provider support

**Traffic & Access Control**
- Per-user speed limiting with dynamic rate adjustment
- Device/IP-based connection limits with configurable thresholds
- Intelligent connection pooling with automatic oldest-connection eviction
- Protocol and domain-based traffic filtering and auditing

**Advanced Features**
- XTLS and Reality protocol support for enhanced performance and security
- DNS-based traffic routing with custom rule sets
- Graceful configuration reloading without service interruption
- Comprehensive logging with structured output and log rotation

## Quick Start

### Installation

```bash
wget -N https://raw.githubusercontent.com/nsevo/v2sp-script/master/install.sh
bash install.sh
```

The installation script will:
1. Download the latest v2sp binary
2. Generate default configuration files
3. Set up systemd service
4. Configure log rotation

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

### Design Philosophy

v2sp follows a composition-over-inheritance design pattern, integrating tightly with Xray-core while maintaining clean separation of concerns:

```
┌─────────────────────────────────────────────┐
│           Panel API (HTTP/HTTPS)            │
└──────────────────┬──────────────────────────┘
                   │ JSON
         ┌─────────▼─────────┐
         │   API Client      │
         │  (Auto Retry)     │
         └─────────┬─────────┘
                   │
         ┌─────────▼─────────┐
         │   Controller      │
         │ (Node Manager)    │
         └─────────┬─────────┘
                   │
    ┌──────────────┼──────────────┐
    │              │              │
┌───▼───┐    ┌────▼────┐    ┌───▼───┐
│Limiter│    │ Counter │    │ Core  │
│       │    │         │    │(Xray) │
└───────┘    └─────────┘    └───┬───┘
                                 │
                    ┌────────────┼────────────┐
                    │            │            │
              ┌─────▼─────┐ ┌───▼────┐ ┌────▼────┐
              │Dispatcher │ │Inbound │ │Outbound │
              │ (Custom)  │ │        │ │         │
              └───────────┘ └────────┘ └─────────┘
```

### Components

**API Client**: Handles panel communication with automatic retry, ETag caching, and connection pooling.

**Controller**: Manages node lifecycle, coordinates user updates, and schedules periodic tasks (traffic reporting, online user tracking).

**Limiter**: Enforces per-user rate limits, device limits, and connection limits using token bucket algorithms and concurrent-safe maps.

**Counter**: Tracks upload/download bytes per user with atomic operations for accurate accounting.

**Custom Dispatcher**: Extends Xray's dispatcher to inject traffic accounting, rate limiting, and connection management into the data path without modifying core Xray code.

## Features

### Traffic Management

**Speed Limiting**
- Per-user bandwidth control (Mbps)
- Node-level bandwidth caps
- Dynamic speed adjustment based on traffic patterns
- Token bucket implementation for smooth rate limiting

**Connection Management**
- Per-user concurrent connection limits
- Automatic oldest-connection eviction when limit exceeded
- Separate tracking for TCP and UDP connections
- Connection creation time tracking for FIFO eviction

**Device Limiting**
- IP-based device counting
- Configurable simultaneous device limits
- IPv4/IPv6 support with IPv6 address normalization
- Grace period for transient IP changes

### Protocol Support

**Supported Protocols**
- VLESS (with XTLS support)
- VMess (with AEAD encryption)
- Trojan (standard and XTLS variants)
- Shadowsocks (including 2022 edition)

**Transport Methods**
- TCP (with HTTP obfuscation)
- WebSocket (with custom headers)
- HTTP/2 (gRPC mode)
- QUIC (experimental)
- mKCP (with various header types)
- SplitHTTP (for restrictive networks)

### Security & Privacy

**Certificate Management**
- Automatic ACME certificate provisioning
- Support for 50+ DNS providers
- Certificate auto-renewal
- Self-signed certificate fallback

**Privacy Features**
- Reality protocol for TLS fingerprint randomization
- No persistent user data storage
- Optional traffic obfuscation
- Configurable logging levels

## API Specification

### Overview

v2sp communicates with panel backends through a single HTTP/HTTPS endpoint. The API is language-agnostic and follows RESTful principles with JSON payloads.

### Authentication

All requests include the following query parameters:

| Parameter   | Type   | Description                                    |
|-------------|--------|------------------------------------------------|
| `action`    | string | API action (config, user, push, alive, etc.)   |
| `node_id`   | int    | Node identifier                                |
| `node_type` | string | Protocol type (vless, vmess, trojan, etc.)     |
| `token`     | string | API key for authentication                     |

**Example Request**
```
GET /api/v2sp?action=user&node_id=1&node_type=vless&token=your_api_key
```

### Response Format

**Success Response** (HTTP 200)
```json
{
  "users": [...]
}
```

**Not Modified** (HTTP 304)
No body, indicates cached data is still valid.

**Error Response** (HTTP 4xx/5xx)
```json
{
  "message": "Descriptive error message"
}
```

### Endpoints

#### GET /api?action=config

Retrieves node configuration including protocol settings, TLS certificates, and routing rules.

**Request Headers**
```
If-None-Match: "config-etag-value"
```

**Response** (HTTP 200)
```json
{
  "Log": {
    "Level": "info",
    "Output": "/var/log/v2sp/access.log"
  },
  "Cores": [
    {
      "Type": "xray",
      "AssetPath": "/etc/v2sp/",
      "DnsConfigPath": "/etc/v2sp/dns.json",
      "RouteConfigPath": "/etc/v2sp/route.json"
    }
  ],
  "Nodes": [
    {
      "ApiHost": "https://panel.example.com",
      "ApiKey": "your_api_key",
      "NodeID": 1,
      "NodeType": "vless",
      "Timeout": 30,
      "ListenIP": "0.0.0.0",
      "SendIP": "0.0.0.0",
      "EnableXTLS": true,
      "CertConfig": {
        "CertMode": "dns",
        "CertDomain": "node1.example.com",
        "Provider": "cloudflare",
        "Email": "admin@example.com"
      }
    }
  ]
}
```

**Response Headers**
```
Content-Type: application/json
ETag: "config-etag-value"
```

**Caching**: Implement ETag support to minimize unnecessary configuration reloads.

#### GET /api?action=user

Retrieves active user list with associated limits.

**Request Headers**
```
If-None-Match: "users-etag-value"
```

**Response** (HTTP 200)
```json
{
  "users": [
    {
      "id": 1001,
      "uuid": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
      "speed_limit": 100,
      "device_limit": 3,
      "conn_limit": 50
    },
    {
      "id": 1002,
      "uuid": "yyyyyyyy-yyyy-yyyy-yyyy-yyyyyyyyyyyy",
      "speed_limit": 0,
      "device_limit": 0,
      "conn_limit": 0
    }
  ]
}
```

**Field Specifications**

| Field         | Type   | Required | Description                                  |
|---------------|--------|----------|----------------------------------------------|
| `id`          | int    | Yes      | Unique user identifier                       |
| `uuid`        | string | Yes      | User UUID (RFC 4122 format)                  |
| `speed_limit` | int    | No       | Bandwidth limit in Mbps (0 = unlimited)      |
| `device_limit`| int    | No       | Maximum concurrent devices (0 = unlimited)   |
| `conn_limit`  | int    | No       | Maximum concurrent connections (0 = unlimited)|

**Notes**:
- Users with limits set to 0 or omitted fields will have no restrictions
- User list changes are detected automatically and applied without restart
- When limits change, existing connections remain active but new connections use updated limits

#### GET /api?action=alivelist

Retrieves current online device counts for device limit enforcement.

**Response** (HTTP 200)
```json
{
  "alive": {
    "1001": 2,
    "1002": 1,
    "1003": 0
  }
}
```

**Field Specifications**

| Field   | Type   | Description                                           |
|---------|--------|-------------------------------------------------------|
| `alive` | object | Map of user IDs to current online device count        |

**Usage**: v2sp compares reported device counts against `device_limit` to enforce restrictions.

#### POST /api?action=push

Reports user traffic consumption for billing and quota enforcement.

**Request Body**
```json
{
  "1001": [104857600, 209715200],
  "1002": [52428800, 104857600]
}
```

**Format**: `{ "user_id": [upload_bytes, download_bytes] }`

**Response** (HTTP 200)
```json
{
  "message": "ok"
}
```

**Implementation Notes**:
- Traffic is reported periodically (default: 60 seconds)
- Implement idempotency to handle duplicate reports
- Use database transactions to ensure atomic updates
- Values are in bytes and represent cumulative traffic since last report

#### POST /api?action=alive

Reports currently online user IPs for real-time monitoring and device tracking.

**Request Body**
```json
{
  "1001": ["1.2.3.4", "5.6.7.8"],
  "1002": ["9.10.11.12"]
}
```

**Format**: `{ "user_id": ["ip1", "ip2", ...] }`

**Response** (HTTP 200)
```json
{
  "message": "ok"
}
```

**Implementation Notes**:
- Reported every 60 seconds by default
- IPs are deduplicated on backend
- Implement expiry mechanism (recommend 5-minute TTL)
- Supports both IPv4 and IPv6 addresses

### Error Handling

**Common Error Codes**

| Status | Meaning                | Action                                        |
|--------|------------------------|-----------------------------------------------|
| 400    | Bad Request            | Check request format and parameters           |
| 401    | Unauthorized           | Verify API token                              |
| 403    | Forbidden              | Node may be disabled                          |
| 404    | Not Found              | Node ID doesn't exist                         |
| 500    | Internal Server Error  | Check panel logs                              |
| 502    | Bad Gateway            | Panel backend is down                         |
| 503    | Service Unavailable    | Panel is under maintenance                    |

**Error Response Format**
```json
{
  "message": "Detailed error description",
  "code": "ERROR_CODE",
  "details": {}
}
```

### API Implementation Checklist

When implementing panel API endpoints, ensure:

**Response Format**
- [ ] Content-Type header set to `application/json`
- [ ] Character encoding is UTF-8
- [ ] JSON is properly formatted (use `json_encode()`, `JSON.stringify()`, etc.)
- [ ] Field names use snake_case (not camelCase)

**HTTP Semantics**
- [ ] Correct status codes (200, 304, 4xx, 5xx)
- [ ] ETag support for caching (recommended)
- [ ] If-None-Match request header handling
- [ ] Proper error messages in response body

**Data Validation**
- [ ] Type safety (id as int, uuid as string, etc.)
- [ ] UUID format validation (RFC 4122)
- [ ] Limit values are non-negative integers
- [ ] Required fields are always present

**Security**
- [ ] API token validation on every request
- [ ] Rate limiting to prevent abuse
- [ ] SQL injection prevention
- [ ] Input sanitization

**Performance**
- [ ] Database query optimization
- [ ] Connection pooling
- [ ] Response caching where appropriate
- [ ] Efficient JSON encoding

## Configuration

### Core Configuration

The main configuration file (`config.json`) defines logging, core settings, and node configurations.

**Example Configuration**
```json
{
  "Log": {
    "Level": "info",
    "Output": "/var/log/v2sp/v2sp.log"
  },
  "Cores": [
    {
      "Type": "xray",
      "Log": {
        "Level": "warning"
      },
      "AssetPath": "/etc/v2sp/",
      "DnsConfigPath": "/etc/v2sp/dns.json",
      "RouteConfigPath": "/etc/v2sp/route.json"
    }
  ],
  "Nodes": [
    {
      "Core": "xray",
      "ApiHost": "https://panel.example.com",
      "ApiKey": "your_secure_api_key",
      "NodeID": 1,
      "NodeType": "vless",
      "Timeout": 30,
      "ListenIP": "0.0.0.0",
      "SendIP": "0.0.0.0",
      "DeviceOnlineMinTraffic": 200,
      "EnableXTLS": true,
      "EnableVless": true,
      "CertConfig": {
        "CertMode": "dns",
        "CertDomain": "node1.example.com",
        "Provider": "cloudflare",
        "Email": "admin@example.com",
        "DNSEnv": {
          "CF_DNS_API_TOKEN": "your_cloudflare_token"
        }
      },
      "LimitConfig": {
        "EnableDynamicSpeedLimit": false,
        "SpeedLimit": 0,
        "DeviceLimit": 0,
        "ConnLimit": 0
      }
    }
  ]
}
```

### Configuration Reference

**Log Options**

| Field    | Type   | Default | Description                                |
|----------|--------|---------|------------------------------------------|
| `Level`  | string | `info`  | Log level: debug, info, warning, error   |
| `Output` | string | stdout  | Log file path or empty for stdout        |

**Node Options**

| Field                      | Type    | Required | Description                                    |
|----------------------------|---------|----------|------------------------------------------------|
| `Core`                     | string  | No       | Core type, defaults to "xray"                  |
| `ApiHost`                  | string  | Yes      | Panel API base URL                             |
| `ApiKey`                   | string  | Yes      | API authentication token                       |
| `NodeID`                   | int     | Yes      | Unique node identifier                         |
| `NodeType`                 | string  | Yes      | Protocol: vless, vmess, trojan, shadowsocks    |
| `Timeout`                  | int     | No       | API request timeout in seconds (default: 30)   |
| `ListenIP`                 | string  | No       | Listening address (default: 0.0.0.0)           |
| `SendIP`                   | string  | No       | Outbound source IP (default: 0.0.0.0)          |
| `DeviceOnlineMinTraffic`   | int     | No       | Minimum traffic (KB) to count as online        |
| `EnableXTLS`               | bool    | No       | Enable XTLS support                            |
| `EnableVless`              | bool    | No       | Enable VLESS protocol                          |

**Certificate Configuration**

| Field        | Type   | Description                                        |
|--------------|--------|----------------------------------------------------|
| `CertMode`   | string | Certificate mode: none, file, http, dns, self      |
| `CertDomain` | string | Domain name for certificate                        |
| `CertFile`   | string | Path to certificate file (file mode)               |
| `KeyFile`    | string | Path to private key file (file mode)               |
| `Provider`   | string | DNS provider name (dns mode)                       |
| `Email`      | string | ACME account email                                 |
| `DNSEnv`     | object | DNS provider credentials                           |

**Limit Configuration**

| Field                      | Type | Description                                     |
|----------------------------|------|-------------------------------------------------|
| `EnableDynamicSpeedLimit`  | bool | Enable dynamic speed adjustment                 |
| `SpeedLimit`               | int  | Node-level speed limit in Mbps (0 = unlimited)  |
| `DeviceLimit`              | int  | Node-level device limit (0 = unlimited)         |
| `ConnLimit`                | int  | Node-level connection limit (0 = unlimited)     |

### DNS Configuration

Custom DNS rules can be defined in `dns.json`:

```json
{
  "servers": [
    {
      "address": "223.5.5.5",
      "domains": ["geosite:cn"]
    },
    {
      "address": "8.8.8.8",
      "domains": ["geosite:geolocation-!cn"]
    }
  ]
}
```

### Routing Configuration

Traffic routing rules can be defined in `route.json`:

```json
{
  "domainStrategy": "IPIfNonMatch",
  "rules": [
    {
      "type": "field",
      "domain": ["geosite:cn"],
      "outboundTag": "direct"
    },
    {
      "type": "field",
      "ip": ["geoip:cn", "geoip:private"],
      "outboundTag": "direct"
    }
  ]
}
```

## Building from Source

### Prerequisites

- Go 1.21 or later
- Git
- Make (optional)

### Build Commands

**Local Build**
```bash
go build -trimpath -ldflags "-s -w" -o v2sp
```

**Cross-Compilation**

Linux AMD64 (most common for servers):
```bash
GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w -buildid=" -o v2sp-linux-amd64
```

Linux ARM64 (ARM-based servers):
```bash
GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "-s -w -buildid=" -o v2sp-linux-arm64
```

**Build Flags Explained**

- `-trimpath`: Remove file system paths from binary for reproducible builds
- `-ldflags "-s -w"`: Strip debugging information to reduce binary size
- `-buildid=""`: Remove build ID for reproducible builds

### Release Process

Create a new release using the provided script:

```bash
./scripts/release.sh v1.0.0 "Release description"
```

The script performs the following:
1. Validates working directory is clean
2. Pushes latest changes to main branch
3. Creates and pushes Git tag
4. Triggers GitHub Actions workflow for automated builds
5. Optionally creates GitHub Release (requires `gh` CLI)

## Deployment

### Systemd Service

v2sp includes a systemd service file for production deployments:

```ini
[Unit]
Description=v2sp Multi-Node Backend
After=network.target

[Service]
Type=simple
User=nobody
ExecStart=/usr/local/bin/v2sp run -c /etc/v2sp/config.json
ExecReload=/bin/kill -HUP $MAINPID
Restart=on-failure
RestartSec=10s
LimitNOFILE=1000000

[Install]
WantedBy=multi-user.target
```

**Service Management**
```bash
systemctl enable v2sp
systemctl start v2sp
systemctl status v2sp
systemctl reload v2sp  # Reload configuration without downtime
```

### Docker Deployment

**Dockerfile**
```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /build
COPY . .
RUN go build -trimpath -ldflags "-s -w" -o v2sp

FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
COPY --from=builder /build/v2sp /usr/local/bin/
ENTRYPOINT ["/usr/local/bin/v2sp"]
CMD ["run", "-c", "/etc/v2sp/config.json"]
```

**Docker Compose**
```yaml
version: '3.8'
services:
  v2sp:
    image: ghcr.io/nsevo/v2sp:latest
    container_name: v2sp
    restart: unless-stopped
    volumes:
      - ./config:/etc/v2sp
      - ./logs:/var/log/v2sp
    network_mode: host
    cap_add:
      - NET_ADMIN
```

### Production Considerations

**Security**
- Run as non-privileged user
- Use firewall rules to restrict access
- Keep API keys secure and rotate regularly
- Enable TLS for panel API communication
- Regularly update to latest stable version

**Performance**
- Allocate sufficient file descriptors (`LimitNOFILE`)
- Monitor memory usage and set appropriate limits
- Use SSD storage for high-traffic nodes
- Consider using CDN for certificate provisioning
- Enable kernel TCP optimizations

**Monitoring**
- Set up log aggregation (e.g., ELK, Loki)
- Monitor system metrics (CPU, memory, network)
- Track traffic anomalies
- Set up alerting for service failures
- Regular backup of configuration files

## Development

### Project Structure

```
v2sp/
├── api/            # Panel API client implementation
├── cmd/            # CLI commands and entry point
├── common/         # Shared utilities (crypto, task, format)
├── conf/           # Configuration structures and validation
├── core/           # Xray core integration
│   └── xray/       # Xray-specific implementation
│       └── app/    # Custom dispatcher and components
├── limiter/        # Rate limiting and connection management
├── node/           # Node controller and lifecycle management
└── example/        # Example configuration files
```

### Code Architecture

**Dependency Flow**
```
main.go
  ↓
cmd/
  ↓
node/Controller
  ├→ api/Client (panel communication)
  ├→ core/Xray (protocol handling)
  ├→ limiter/Limiter (rate/device/connection limits)
  └→ common/Counter (traffic accounting)
```

### Contributing

Contributions are welcome. Please follow these guidelines:

**Code Style**
- Follow Go standard formatting (`gofmt`)
- Use meaningful variable and function names
- Add comments for exported functions
- Keep functions focused and testable

**Pull Request Process**
1. Fork the repository
2. Create a feature branch
3. Make your changes with clear commit messages
4. Ensure all tests pass
5. Update documentation if needed
6. Submit pull request with detailed description

**Testing**
```bash
# Run unit tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests with race detector
go test -race ./...
```

## Troubleshooting

### Common Issues

**Problem**: Connection refused when connecting to panel API

**Solution**: 
- Verify `ApiHost` is correct and accessible
- Check firewall rules
- Ensure panel API is running
- Verify API token is valid

**Problem**: Users not showing up in node

**Solution**:
- Check API response format matches specification
- Verify user list endpoint returns correct JSON
- Check v2sp logs for API errors
- Ensure user UUIDs are valid RFC 4122 format

**Problem**: Speed limit not working

**Solution**:
- Verify `speed_limit` values are in Mbps (not Bps)
- Check if node-level speed limit overrides user limits
- Ensure rate limiting is enabled in configuration
- Monitor system resources (CPU/memory)

**Problem**: Certificate renewal fails

**Solution**:
- Verify DNS provider credentials are correct
- Check domain ownership
- Ensure port 80/443 are accessible (http mode)
- Review ACME account status

### Debug Mode

Enable detailed logging for troubleshooting:

```json
{
  "Log": {
    "Level": "debug",
    "Output": "/var/log/v2sp/debug.log"
  }
}
```

## Credits

v2sp is built upon the excellent work of the following projects:

**Core Projects**
- [XTLS/Xray-core](https://github.com/XTLS/Xray-core) - High-performance proxy platform
- [v2fly/v2ray-core](https://github.com/v2fly/v2ray-core) - Platform for building proxies

**Inspiration & Reference**
- [XrayR](https://github.com/XrayR-project/XrayR) - Xray backend for V2board and other panels
- [V2bX](https://github.com/wyx2685/V2bX) - Multi-protocol backend implementation
- [SagerNet/sing-box](https://github.com/SagerNet/sing-box) - Universal proxy platform

**Infrastructure**
- [go-acme/lego](https://github.com/go-acme/lego) - ACME client library
- [spf13/cobra](https://github.com/spf13/cobra) - CLI framework
- [sirupsen/logrus](https://github.com/sirupsen/logrus) - Structured logging

Special thanks to all contributors and users who have provided feedback, bug reports, and feature suggestions.

## License

This project is provided as-is for educational and self-hosting purposes. Please review and comply with local regulations regarding proxy services.

## Links

- Documentation: [GitHub Wiki](https://github.com/nsevo/v2sp/wiki)
- Issue Tracker: [GitHub Issues](https://github.com/nsevo/v2sp/issues)
- Star History: [Star Chart](https://starchart.cc/nsevo/v2sp)
