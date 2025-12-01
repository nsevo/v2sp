# v2sp

v2sp 是自研面板配套的多内核节点端，默认集成 xray / sing-box / hysteria2，开箱即用，单实例即可管理多节点。

## Highlights

- VLESS / VMess / Trojan / Shadowsocks / Hysteria2 / Anytls 一体化，可自由裁剪内核
- 设备/IP/速率限制、审计规则、DNS 分流等基础能力全部内建
- 支持自动申请 TLS/ACME 证书，支持 Reality、XTLS 等新特性
- 透明的 JSON 配置，方便和自建脚本或面板联动

## Install

```bash
wget -N https://raw.githubusercontent.com/nsevo/v2sp-script/master/install.sh && bash install.sh
```

## Build

```bash
GOEXPERIMENT=jsonv2 go build -v -o build_assets/v2sp \
  -tags "sing xray hysteria2 with_quic with_grpc with_utls with_wireguard with_acme with_gvisor" \
  -trimpath -ldflags "-X 'github.com/nsevo/v2sp/cmd.version=$version' -s -w -buildid="
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

v2sp 只需要一个 HTTP/HTTPS 入口即可。后端可以使用 PHP、Node.js、Go、Python 等任意语言实现，接口只需识别下表中 `action` 并返回 JSON。客户端会自动附带：

| Query 参数  | 说明                                                                 |
|-------------|----------------------------------------------------------------------|
| `action`    | `config` / `user` / `alivelist` / `push` / `alive`                   |
| `node_id`   | 节点 ID，整数                                                        |
| `node_type` | 节点类型（`vmess/vless/trojan/shadowsocks/hysteria2/anytls`）        |
| `token`     | 后端配置的 API Key，用于鉴权                                        |

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
- 返回 `{"users":[...]}` 数组，每个用户对象包含：`id`、`uuid`（或密码）、`device_limit`、`speed_limit`、`u`/`d`（已用流量）、`transfer_enable` 等面板侧可提供的信息。
- 可选支持 `304` 与 `ETag`，以减轻负载。
- 若没有可用用户，返回空数组即可。

**alivelist**
- 返回 `{"alive": { "26": 2, "27": 1 }}` 这类键值映射；数值代表该用户允许的同时在线 IP/设备数（通常来自面板记录）。
- 若暂未统计，可返回空对象 `{ "alive": {} }`。

**push**
- 请求体为 `userID: [uploadBytes, downloadBytes]` 的映射，单位为字节，API 需要负责累加到面板数据库。
- 成功时返回 `{ "message": "ok" }`，失败时可返回 `{ "message": "db error" }` 并使用 `4xx/5xx`。
- 服务器应保证幂等或合理处理重复上报。

**alive**
- 请求体为 `userID: ["ip1","ip2"...]` 的映射，代表当前在线 IP 列表；后端可用来记录实时在线设备。
- 建议在后台做去重/过期处理，响应同样返回 `{ "message": "ok" }`。

只要遵循以上 JSON 规范即可，语言和框架不限。

## Credits

项目基于社区多款核心演进，特别感谢：
[XTLS](https://github.com/XTLS/) · [V2Fly](https://github.com/v2fly) · [XrayR](https://github.com/XrayR/XrayR) · [SagerNet/sing-box](https://github.com/SagerNet/sing-box) · [V2bX 项目](https://github.com/wyx2685/V2bX)

Star 走势可在 [starchart](https://starchart.cc/nsevo/v2sp) 查看。
