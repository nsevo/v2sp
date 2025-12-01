# v2sp

v2sp 是自研面板配套的 Xray 节点端，使用官方 Xray-core，开箱即用，单实例即可管理多节点。

## Highlights

- 基于官方 Xray-core v1.251201.0，支持 VLESS / VMess / Trojan / Shadowsocks / Hysteria 等协议
- 设备/IP/速率/连接数限制、审计规则、DNS 分流等基础能力全部内建
- 智能连接管理：连接数超限时自动替换最旧连接，提升用户体验
- 支持自动申请 TLS/ACME 证书，支持 Reality、XTLS 等新特性
- 透明的 JSON 配置，方便和自建脚本或面板联动

## Install

```bash
wget -N https://raw.githubusercontent.com/nsevo/v2sp-script/master/install.sh && bash install.sh
```

## Build

```bash
# 本地构建
go build -trimpath -ldflags "-s -w" -o v2sp main.go

# Linux AMD64 (生产环境)
GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o v2sp-linux-amd64 main.go

# Linux ARM64 (ARM 服务器)
GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "-s -w" -o v2sp-linux-arm64 main.go
```

## Release

```bash
./scripts/release.sh v1.0.2 "Short release notes"
```

该脚本会：
- 确认工作区干净并推送 main
- 打上 `v1.0.2` tag 并推送
- 如果安装了 GitHub CLI，则自动创建 Release（否则可手动在网页发布）

## 单接口对接说明

v2sp 只需要一个 HTTP/HTTPS 入口即可。后端可以使用 PHP、Node.js、Go、Python 等任意语言实现，**接口必须返回标准 JSON 格式**。客户端会自动附带：

| Query 参数  | 说明                                                                 |
|-------------|----------------------------------------------------------------------|
| `action`    | `config` / `user` / `alivelist` / `push` / `alive`                   |
| `node_id`   | 节点 ID，整数                                                        |
| `node_type` | 节点类型（`vmess/vless/trojan/shadowsocks/hysteria`）                |
| `token`     | 后端配置的 API Key，用于鉴权                                        |

### 重要：API 响应格式要求

**必须返回标准 JSON 格式：**
- Content-Type: `application/json`
- 使用标准 JSON 编码（UTF-8）
- 支持 ETag 缓存（可选，但推荐）
- 不支持 msgpack、protobuf 等其他格式

各 `action` 的期望行为如下（支持返回 `4xx/5xx` 表达错误，`message` 字段用于描述原因）：

| action      | method | 请求体示例                                   | 作用概要 |
|-------------|--------|----------------------------------------------|----------|
| `config`    | GET    | 无                                           | 下发节点完整配置 |
| `user`      | GET    | 无                                           | 返回可用用户列表 |
| `alivelist` | GET    | 无                                           | 返回用户在线设备计数 |
| `push`      | POST   | `{ "26":[uploadBytes,downloadBytes], ... }`  | 上报并累积流量 |
| `alive`     | POST   | `{ "26":["1.1.1.1","2.2.2.2"], ... }`        | 上报在线 IP 列表 |

**config**
- 必须返回 v2sp 能直接使用的完整 JSON（包含 `Log`、`Cores`、`Nodes`、`ApiHost` 等字段），建议与面板中的“节点详情”保持一致。
- 可以设置 `ETag` 响应头；v2sp 会发送 `If-None-Match`，当配置无变化时可返回 `304 Not Modified`。
- 若节点不存在或已禁用，应返回 `404` 或 `403`，并附带 `{ "message": "reason" }`。

**user**
- 返回 `{"users":[...]}` 数组，每个用户对象包含：
  - `id` (int): 用户 ID **[必需]**
  - `uuid` (string): 用户 UUID **[必需]**
  - `speed_limit` (int): 速率限制 (Mbps)，0 = 不限制 **[可选]**
  - `device_limit` (int): 设备数限制，0 = 不限制 **[可选]**
  - `conn_limit` (int): 连接数限制，0 = 不限制 **[可选，v1.2.0+]**
- 可选支持 `304` 与 `ETag`，以减轻负载。
- 若没有可用用户，返回空数组即可。

**示例响应：**
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

**alivelist**
- 返回 `{"alive": { "26": 2, "27": 1 }}` 这类键值映射；数值代表该用户允许的同时在线 IP/设备数（通常来自面板记录）。
- 若暂未统计，可返回空对象 `{ "alive": {} }`。

**示例响应：**
```json
{
  "alive": {
    "1001": 2,
    "1002": 1
  }
}
```

**push**
- 请求体为 `userID: [uploadBytes, downloadBytes]` 的映射，单位为字节，API 需要负责累加到面板数据库。
- 成功时返回 `{ "message": "ok" }`，失败时可返回 `{ "message": "db error" }` 并使用 `4xx/5xx`。
- 服务器应保证幂等或合理处理重复上报。

**示例请求：**
```json
{
  "1001": [1048576, 2097152],
  "1002": [524288, 1048576]
}
```

**alive**
- 请求体为 `userID: ["ip1","ip2"...]` 的映射，代表当前在线 IP 列表；后端可用来记录实时在线设备。
- 建议在后台做去重/过期处理，响应同样返回 `{ "message": "ok" }`。

**示例请求：**
```json
{
  "1001": ["1.2.3.4", "5.6.7.8"],
  "1002": ["9.10.11.12"]
}
```

### API 开发检查清单

实现面板 API 时，请确保：
- 所有响应使用 `Content-Type: application/json`
- 正确的 JSON 编码（使用 `json_encode`、`JSON.stringify` 等标准方法）
- HTTP 状态码正确（200 成功，304 未修改，4xx/5xx 错误）
- 错误时返回 `{ "message": "错误原因" }`
- 字段类型正确（id 是 int，uuid 是 string 等）
- 字段名使用下划线命名（`speed_limit`，不是 `speedLimit`）

只要遵循以上 JSON 规范即可，语言和框架不限。

## Credits

项目基于社区多款核心演进，特别感谢：
[XTLS](https://github.com/XTLS/) · [V2Fly](https://github.com/v2fly) · [XrayR](https://github.com/XrayR/XrayR) · [SagerNet/sing-box](https://github.com/SagerNet/sing-box) · [V2bX 项目](https://github.com/wyx2685/V2bX)

Star 走势可在 [starchart](https://starchart.cc/nsevo/v2sp) 查看。
