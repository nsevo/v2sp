#!/usr/bin/env bash
#
# v2sp Install Script (V2RS)
#
# Usage (on server):
#   bash install.sh install
#   bash install.sh update
#   bash install.sh uninstall
#
# After install, use v2sp built-in commands (no v2spctl):
#   v2sp server -c /etc/v2sp/config.json
#   v2sp config init
#   v2sp config validate
#   v2sp config show
#
set -euo pipefail

# -----------------------------
# Config (override via env)
# -----------------------------
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
CONFIG_DIR="${CONFIG_DIR:-/etc/v2sp}"
SERVICE_NAME="${SERVICE_NAME:-v2sp}"
BINARY_NAME="${BINARY_NAME:-v2sp}"

# Default download URL (linux/amd64). Override with V2SP_DOWNLOAD_URL or --url.
DOWNLOAD_BASE_URL="${DOWNLOAD_BASE_URL:-https://resources.valtrogen.com/core}"
V2SP_DOWNLOAD_URL="${V2SP_DOWNLOAD_URL:-}"

# -----------------------------
# UI helpers
# -----------------------------
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${BLUE}[INFO]${NC} $*"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $*"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*"; }

die() { log_error "$*"; exit 1; }

need_root() {
  if [[ "${EUID:-0}" -ne 0 ]]; then
    die "This script requires root privileges. Run: sudo bash install.sh <install|update|uninstall>"
  fi
}

have_cmd() { command -v "$1" >/dev/null 2>&1; }

ensure_deps() {
  if ! have_cmd curl && ! have_cmd wget; then
    die "Please install curl or wget first"
  fi
  if ! have_cmd systemctl; then
    die "systemctl not found (systemd required)"
  fi
}

download_to() {
  local url="$1"
  local out="$2"
  local tmp
  tmp="$(mktemp)"

  if have_cmd curl; then
    curl -fsSL -o "$tmp" "$url"
  else
    wget -qO "$tmp" "$url"
  fi

  mv "$tmp" "$out"
}

resolve_download_url() {
  if [[ -n "${V2SP_DOWNLOAD_URL}" ]]; then
    echo "${V2SP_DOWNLOAD_URL}"
    return
  fi

  # We only support Linux amd64 (Debian/Ubuntu) in this environment.
  echo "${DOWNLOAD_BASE_URL}/v2sp-linux-amd64"
}

install_binary() {
  local url="$1"
  local out="${INSTALL_DIR}/${BINARY_NAME}"

  log_info "Downloading v2sp (linux/amd64) from: ${url}"
  mkdir -p "${INSTALL_DIR}"
  download_to "${url}" "${out}"
  chmod +x "${out}"

  log_success "Binary installed: ${out}"
  "${out}" version 2>/dev/null || true
}

create_config_if_missing() {
  mkdir -p "${CONFIG_DIR}"

  if [[ ! -f "${CONFIG_DIR}/config.json" ]]; then
    cat > "${CONFIG_DIR}/config.json" <<'EOF'
{
  "Log": {
    "Level": "info",
    "Output": ""
  },
  "Cores": [
    {
      "Type": "xray"
    }
  ],
  "Nodes": [
    {
      "ApiHost": "https://sub.example.com/api",
      "ApiKey": "your_api_token",
      "NodeID": 1
    }
  ]
}
EOF
    chmod 600 "${CONFIG_DIR}/config.json" 2>/dev/null || true
    log_warn "Created sample config: ${CONFIG_DIR}/config.json (edit before production)"
  else
    log_info "Config exists, skipping: ${CONFIG_DIR}/config.json"
  fi

  if [[ ! -f "${CONFIG_DIR}/dns.json" ]]; then
    cat > "${CONFIG_DIR}/dns.json" <<'EOF'
{ "servers": ["1.1.1.1", "8.8.8.8", "localhost"], "tag": "dns_inbound" }
EOF
    chmod 644 "${CONFIG_DIR}/dns.json" 2>/dev/null || true
  fi

  if [[ ! -f "${CONFIG_DIR}/route.json" ]]; then
    cat > "${CONFIG_DIR}/route.json" <<'EOF'
{
  "domainStrategy": "IPOnDemand",
  "rules": [
    { "type": "field", "outboundTag": "block", "ip": ["geoip:private"] },
    { "type": "field", "outboundTag": "IPv4_out", "network": "udp,tcp" }
  ]
}
EOF
    chmod 644 "${CONFIG_DIR}/route.json" 2>/dev/null || true
  fi

  if [[ ! -f "${CONFIG_DIR}/custom_outbound.json" ]]; then
    cat > "${CONFIG_DIR}/custom_outbound.json" <<'EOF'
[
  { "tag": "IPv4_out", "protocol": "freedom", "settings": {} },
  { "tag": "IPv6_out", "protocol": "freedom", "settings": { "domainStrategy": "UseIPv6" } },
  { "protocol": "blackhole", "tag": "block" }
]
EOF
    chmod 644 "${CONFIG_DIR}/custom_outbound.json" 2>/dev/null || true
  fi

  # Optional environment file for systemd (PPROF_ADDR / XRAY_DNS_PATH / SING_DNS_PATH, etc.)
  if [[ ! -f "${CONFIG_DIR}/v2sp.env" ]]; then
    cat > "${CONFIG_DIR}/v2sp.env" <<'EOF'
# Optional environment overrides for v2sp.service
# PPROF_ADDR=127.0.0.1:6060
# XRAY_DNS_PATH=/etc/v2sp/dns.json
# SING_DNS_PATH=/etc/v2sp/sing-dns.json
EOF
    chmod 644 "${CONFIG_DIR}/v2sp.env" 2>/dev/null || true
  fi

  log_success "Config directory ready: ${CONFIG_DIR}"
}

write_systemd_unit() {
  local unit="/etc/systemd/system/${SERVICE_NAME}.service"

  log_info "Writing systemd unit: ${unit}"
  cat > "${unit}" <<EOF
[Unit]
Description=v2sp V2RS Node
After=network.target network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
Group=root
EnvironmentFile=-${CONFIG_DIR}/v2sp.env
ExecStart=${INSTALL_DIR}/${BINARY_NAME} server -c ${CONFIG_DIR}/config.json --watch
Restart=always
RestartSec=3
LimitNOFILE=1048576

StandardOutput=journal
StandardError=journal
SyslogIdentifier=${SERVICE_NAME}

# Basic hardening (keep writable config dir)
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=${CONFIG_DIR}
PrivateTmp=true

[Install]
WantedBy=multi-user.target
EOF

  systemctl daemon-reload
  log_success "systemd unit installed"
}

start_service() {
  log_info "Enabling and starting: ${SERVICE_NAME}"
  systemctl enable "${SERVICE_NAME}" >/dev/null 2>&1 || true
  systemctl restart "${SERVICE_NAME}"
  sleep 1
  if systemctl is-active --quiet "${SERVICE_NAME}"; then
    log_success "Service is running"
  else
    log_warn "Service failed to start. Check: journalctl -u ${SERVICE_NAME} -n 50 --no-pager"
  fi
}

uninstall_all() {
  log_warn "Uninstalling v2sp..."
  systemctl stop "${SERVICE_NAME}" 2>/dev/null || true
  systemctl disable "${SERVICE_NAME}" 2>/dev/null || true
  rm -f "/etc/systemd/system/${SERVICE_NAME}.service"
  rm -f "${INSTALL_DIR}/${BINARY_NAME}"
  systemctl daemon-reload
  log_success "Uninstalled binary and systemd unit"
  log_warn "Config preserved at: ${CONFIG_DIR}"
}

usage() {
  cat <<EOF
v2sp installer

Usage:
  bash install.sh install [--url <download_url>]
  bash install.sh update  [--url <download_url>]
  bash install.sh uninstall

Notes:
  - Default config: ${CONFIG_DIR}/config.json
  - Service: systemctl status ${SERVICE_NAME}
  - Logs: journalctl -u ${SERVICE_NAME} -f
  - Use v2sp commands directly: v2sp server / v2sp config ...
EOF
}

main() {
  need_root
  ensure_deps

  local cmd="${1:-}"
  shift || true

  local url_arg=""
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --url)
        url_arg="${2:-}"
        shift 2
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        die "Unknown argument: $1"
        ;;
    esac
  done

  if [[ -n "$url_arg" ]]; then
    V2SP_DOWNLOAD_URL="$url_arg"
  fi
  local url
  url="$(resolve_download_url)"

  case "$cmd" in
    install|"")
      install_binary "$url"
      create_config_if_missing
      write_systemd_unit
      start_service
      ;;
    update)
      systemctl stop "${SERVICE_NAME}" 2>/dev/null || true
      install_binary "$url"
      systemctl restart "${SERVICE_NAME}" 2>/dev/null || true
      log_success "Updated and restarted: ${SERVICE_NAME}"
      ;;
    uninstall)
      uninstall_all
      ;;
    *)
      usage
      exit 1
      ;;
  esac

  log_info "Common commands:"
  echo "  - Edit config: ${EDITOR:-vi} ${CONFIG_DIR}/config.json"
  echo "  - Validate:    ${INSTALL_DIR}/${BINARY_NAME} config validate"
  echo "  - Show:        ${INSTALL_DIR}/${BINARY_NAME} config show"
  echo "  - Wizard:      ${INSTALL_DIR}/${BINARY_NAME} config init"
  echo "  - Logs:        ${INSTALL_DIR}/${BINARY_NAME} log"
  echo "  - Restart:     ${INSTALL_DIR}/${BINARY_NAME} restart"
  echo "  - Update:      ${INSTALL_DIR}/${BINARY_NAME} update"
  echo "  - Reality key: ${INSTALL_DIR}/${BINARY_NAME} reality"
}

main "$@"


