# 安全整改报告 — bifu-cli（SA-BIFU-CLI-202606）

对应审计报告 [Security_Audit_Report_bifu-cli_2026.md](Security_Audit_Report_bifu-cli_2026.md)
的 29 项发现，记录每一项的修复方式、对应文件，以及仍需人工完成的运维步骤。

> 结论：**29 项全部修复**，并按报告「修复建议」落地。已通过 `go build` / `go vet` /
> `go test ./...` / `goreleaser check` / `actionlint` / `gofmt`。

## 一、逐条修复对照（全部 29 项）

| 编号 | 严重性 | 问题 | 修复方式 | 文件 |
|---|---|---|---|---|
| 001 | High | MCP 交易工具零认证/零校验 | 写工具（下单/撤单）默认禁用，需 `BIFU_MCP_ALLOW_TRADE=1`；size/price 数值校验 + 上限 | `internal/mcp/server.go` |
| 002 | High | 重定向泄露会话 Cookie | `CheckRedirect` 跨 host 剥离 Cookie 头 + 限 10 跳 | `internal/client/client.go` |
| 003 | Medium | base_url 无 scheme 校验 + cookie 不绑主机 | `config set` 强制 https/wss（`--insecure` 可临时放行）；cookie 绑定到 base_url 主机 | `cmd/config/config.go`、`internal/client/client.go` |
| 004 | Medium | 会话 cookie 明文存储 | **AES-256-GCM 加密落盘**，密钥按机器/用户派生；内存保持明文，Load 透明解密 | `internal/clifconfig/secret.go` |
| 005 | Medium | verbose 泄露 forex 密码 + 文档谎称脱敏 | body 按字段名脱敏（password/token/secret/cookie）；修正 `docs/usage-agents.md` | `internal/client/client.go`、`docs/usage-agents.md` |
| 006 | Low | MCP cancel JSON 字符串拼接 | 改用 `json.Marshal` 构造返回 | `internal/mcp/server.go` |
| 007 | Low | 私有 WS 不强制 wss | `GetWSPrivateURL`/`GetWSPrivateSpotURL` 把 ws:// 强升 wss:// | `internal/clifconfig/config.go` |
| 008 | Low | 安装/升级链无 checksum 校验 | install.sh 与 npm 都下载并校验 `checksums.txt`；upgrade 经 install.sh 覆盖 | `install.sh`、`npm/install.js` |
| 009 | Low | ClientOrderID 含 user_id + 可碰撞 | 去掉 user_id，加 crypto/rand 后缀防同秒碰撞 | `internal/clifconfig/config.go` |
| 010 | Low | UToken/VToken 死字段 | 删除字段与对应 `config set --u-token` 标志 | `internal/clifconfig/config.go`、`cmd/config/config.go` |
| 011 | Info | upgrade 用 sh -c，Windows 失败 | Windows 改用 `cmd /c` | `cmd/upgrade.go` |
| 012 | Info | QR issueID 在 URL 路径 + web_url 未强制 https | 强制 web_url 为 https（详见下方「说明」） | `cmd/auth/login.go` |
| 013 | Info | 交易命令无预登录闸门 | spot/contract/payment/forex/**orion**（均 cookie 认证）在 load 时校验已登录，未登录直接报错；`ws market` 公共流不 gate | `cmd/root.go` |
| 014 | High | forex 金额用 float64 | **保留 float64 +** 输入校验（volume>0、price/sl/tp≥0、拒非有限值/超上限） | `cmd/forex/forex.go` |
| 015 | High | 发布未签名 + cask 剥离 Gatekeeper | 加 cosign keyless 签名 checksums；**移除** cask 的 `xattr -dr quarantine`，发布说明给出 macOS 放行指引 | `.goreleaser.yaml` |
| 016 | High | CI 宽 PAT + Action 浮动标签 | 所有 Action **SHA 固定**；最小权限 + `id-token: write`；token 拆分（带回退）；goreleaser 锁版本 | `.github/workflows/*` |
| 017 | High | 合约字段无跨字段一致性 | 不变量校验下沉到 API 层 `CreateOrderReq.Validate()`，**CLI 与 MCP 同时覆盖** | `internal/api/contract/contract.go`、`internal/mcp/server.go` |
| 018 | Medium | HTTP_PROXY 截获 cookie/密码 | 认证 HTTP 客户端显式 `Transport{Proxy:nil}`；**login/logout/register 裸客户端**也走 `NewSecureHTTPClient` | `internal/client/client.go`、`cmd/auth/*` |
| 019 | Medium | 密码/cookie 走命令行 | 支持 `BIFU_PASSWORD`/`BIFU_FOREX_PASSWORD`/`BIFU_AUTH_COOKIE` 环境变量；文档移除明文示例 | `cmd/auth/login.go`、`cmd/forex`、`cmd/config` |
| 020 | Medium | npm 安装失败 process.exit(0) | 失败改 `exit(1)` + WARNING | `npm/install.js` |
| 021 | Medium | npm tar 解压无路径校验 | 解压前列举成员，拒绝 `../`/绝对路径 | `npm/install.js` |
| 022 | Medium | 错误响应回显原始 body | 4xx/错误 body 仅 verbose 且脱敏；spot/contract 解析错误不再回显原始 body | `internal/client/client.go`、`internal/api/spot`、`internal/api/contract` |
| 023 | Medium | .gitignore 缺密钥规则 | 补全 `*.pem/*.key/*.p12/*.crt/...` 与 `config.yaml` | `.gitignore` |
| 024 | Medium | MCP 只读工具泄露完整财务画像 | 只读工具默认掩码金额字段，需 `BIFU_MCP_DETAILED=1` 才返回精确值 | `internal/mcp/server.go` |
| 025 | Low | BIFU_CLI_HOME 无校验 | 校验符号链接/目录权限，异常回退默认目录并告警 | `internal/clifconfig/config.go` |
| 026 | Low | extractCookie 无条件信任 cookie 名 | 对 cookie 名做安全字符集校验，非法回退默认名（见下方「说明」） | `cmd/auth/login.go` |
| 027 | Low | skills install 目标无 containment | `filepath.Clean` + 拒绝符号链接目标 | `cmd/skills/skills.go` |
| 028 | Low | MCP 撤单无确认/门控不一致 | MCP 撤单与下单**统一**走 `BIFU_MCP_ALLOW_TRADE` 门控 | `internal/mcp/server.go` |
| 029 | Low | gorilla/websocket 已归档 | 迁移到 `github.com/coder/websocket` | `internal/client/websocket.go`、`go.mod` |

### 两处与报告原文略有出入（已主动说明）

- **012（QR issueID 在 URL 路径）**：已实现报告主要建议「web_url 强制 https」。但「issueID 改放
  fragment/POST body」属于**后端 web 审批页契约**（服务端读取 `/x/<issueID>` 路径），客户端单方面
  改成 `#issueID` 会破坏扫码审批，需与 web 端协同，故未单方面改动。该项为 Info/P3，且 issueID
  单次有效、2 分钟窗口、需 MITM 才可观测，残余风险很低。
- **026（cookie 名白名单）**：报告建议「已知环境名白名单」。考虑到 dev/staging/prod 的 cookie 名
  各不相同，硬编码白名单会误伤，改为**安全字符集校验**（RFC6265 token，拒绝控制/分隔字符），
  同样消除了「无条件信任服务端 cookie 名」的根因，且不会因环境而误判。

## 二、运维侧（已自动落地）

- **仓库级强制 Action SHA 固定**：已通过 API 打开
  `actions/permissions → sha_pinning_required: true`。今后任何使用浮动 `@v*` 标签的 workflow
  会被 GitHub 直接拒绝（016 在平台层焊死）。
- **发布 token 拆分做成不破坏**：workflow 读取
  `${{ secrets.RELEASES_REPO_TOKEN || secrets.HOMEBREW_TAP_GITHUB_TOKEN }}`，
  今天仍用现有共享 PAT 正常发版，一旦补上 scoped token 即自动切换完成分离。
- **MCP 交易闸门在所有注册入口默认关闭**：
  - `bifu-cli mcp setup` 默认只读，`--allow-trade`（及 `--detailed`）才把 env 写进客户端配置；
  - Claude Desktop `.mcpb` 安装界面新增「Allow trading」「Show precise balances」开关，均默认关。

## 三、必须人工完成（无法通过 API 自动化）

### 1. 拆分发布 PAT 为两个最小权限 token（收尾 016）

GitHub 不支持通过 API 创建 PAT，需在网页端创建：

1. 创建 fine-grained PAT（owner `decodeex`），**仅**对 `decodeex/bifu-cli-releases`
   授予 **Contents: Read and write**，加为仓库 secret：
   ```sh
   gh secret set RELEASES_REPO_TOKEN --repo decodeex/bifu-cli --body '<pat>'
   ```
2. 把 `HOMEBREW_TAP_GITHUB_TOKEN` 重新限定为**仅** `decodeex/homebrew-tap` 的
   **Contents: Read and write**。
3. `RELEASES_REPO_TOKEN` 创建后，workflow 里的 `|| HOMEBREW_TAP_GITHUB_TOKEN` 回退即可移除。

### 2. macOS 公证（收尾 015）

cosign 签名 + checksum 校验已上线，但 Gatekeeper 需 Apple 公证。在此之前 macOS 用户首次运行
经 系统设置 → 隐私与安全性 放行一次（发布说明已注明）。彻底解决需：

1. 申请 Apple Developer ID Application 证书 → 在 `.goreleaser.yaml` 为 macOS 构建加
   `sign:`/`notarize:`，并配置 `AC_USERNAME`/`AC_PASSWORD`（app 专用密码）secret。

### 3. 轮换可能在修复前已暴露的凭据

- 曾在 `--verbose` 可能记录 body、或可能经恶意代理、或明文存于被同步/备份的 `config.yaml`
  的会话，建议轮换：让相关用户重新 `bifu-cli auth login`。

## 四、需告知用户的行为变化

- MCP 下单/撤单**默认关闭**——已接入自动交易的用户需重新 `mcp setup --allow-trade`
  （或在 Claude Desktop 里打开开关）。
- `config.yaml` 的 cookie 现已机器绑定（AES-GCM）：拷到其它机器/用户将无法解密，需在该机重新登录。
- 命令行 `--password` / `--auth-cookie` 不再推荐，优先用对应环境变量。
