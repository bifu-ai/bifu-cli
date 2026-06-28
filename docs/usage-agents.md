# bifu-cli 使用说明(给 AI 编程代理)

面向 **Claude Desktop / Codex / Cursor / VS Code / Claude Code** 等 AI 代理的 bifu-cli
接入与使用说明(`mcp setup --client` 一键注册这些客户端)。
bifu-cli 是 BifuFX 交易平台命令行工具:现货、合约、外汇(MT5/TradFi)、支付、
WebSocket 实时行情、orion 信号订阅,并内置 **MCP server** 与 **agent skills**,
让 AI 代理可以直接读余额/持仓/订单、下单/撤单。

> 跨平台:macOS / Windows / Linux(各客户端配置路径自动按系统选择)。
> 命令与配置风格对齐主流交易所 CLI;多 Profile 管理(custom/dev/staging/prod)。

---

## 1. 安装

```bash
# 一键脚本(自动识别 OS/架构)
curl -fsSL https://cli.bifu.dev/install.sh | bash

# Homebrew(推荐:与 node 版本无关,常驻 PATH)
brew install decodeex/tap/bifu-cli

# npm(注意:用 nvm 的话全局命令随 node 版本切换,终端常驻建议用 brew/curl)
npm i -g @decodeex/bifu-cli
```

验证:`bifu-cli version`

升级:`bifu-cli version --check`(查是否有新版)、`bifu-cli upgrade`(按安装方式自动升级,`-y` 免确认)

---

## 2. 给 AI 代理接入(Skills + MCP,或一键插件)

- **Skills**:`SKILL.md` 指南,告诉代理「何时用、怎么用 bifu-cli」(适合让代理用命令行)。
- **MCP**:`bifu-cli mcp serve` 把交易能力暴露成 MCP 工具(适合让代理直接调用工具)。
- **插件(Plugins)**:把 skills + MCP 打包成一个插件,Claude Code / Codex 一条命令装好(见 §2.5)。

三者可叠加用:MCP 提供工具,skills 提供使用说明,插件把两者一键装好。

### 2.1 Claude Code

```bash
# 安装技能到项目(或加 --global 到 ~/.claude/skills)
bifu-cli skills install --client claude

# 注册 MCP server 到 Claude Code(底层走 `claude mcp add`,写入 ~/.claude.json)
bifu-cli --profile dev mcp setup --client claude
```

- 技能落到 `.claude/skills/<name>/SKILL.md`,Claude Code 自动识别。
- MCP 用官方 `claude mcp add bifu -- bifu-cli --profile dev mcp serve` 注册;默认 **local
  作用域(仅当前项目)**。要**所有项目可用**:`claude mcp add -s user bifu -- bifu-cli --profile dev mcp serve`。
- 验证:`claude mcp list`(应显示 `bifu … ✓ Connected`),或在 Claude Code 里 `/mcp`。

> **Claude Desktop(桌面 App,不是 Claude Code)**走另一份配置,用 `--client claude-desktop`:
> `bifu-cli --profile dev mcp setup --client claude-desktop`,重启 App 生效。Claude Desktop 官方只有
> macOS / Windows 版,写入 `claude_desktop_config.json`:macOS `~/Library/Application Support/Claude/`、
> Windows `%APPDATA%\Claude\`(其余系统回退到 `~/.config/Claude/`)。
>
> **关于 `--profile`**:非必填。带上(如 `--profile dev`)会把启动命令钉成
> `mcp serve --profile dev`,**钉死该环境**,与后续 `config use` 无关;**省略**则注册
> `mcp serve`,运行时**跟随当前活动 profile**。MCP 是给代理真实下单用的,**建议显式钉死
> 环境**,避免哪天切到 prod 后代理误在真账户下单。

### 2.2 Codex(OpenAI Codex CLI)

Codex **支持 MCP**(stdio server)。三种方式,任选其一:

**方式 A — bifu-cli 一键(推荐)**

```bash
bifu-cli --profile dev mcp setup --client codex
```

自动调用 `codex mcp add bifu`。codex CLI 不在 PATH 时,macOS 上还会自动到桌面版
**Codex.app**(`/Applications/Codex.app/Contents/Resources/codex`)里找内置二进制;
Windows / Linux 请确保 `codex` 在 PATH(npm 安装默认即在)。都找不到时退化成打印
可粘贴的 TOML 片段。

**方式 B — codex 原生命令**

```bash
codex mcp add bifu -- bifu-cli --profile dev mcp serve
```

**方式 C — 改 `~/.codex/config.toml`**

```toml
[mcp_servers.bifu]
command = "bifu-cli"
args = ["--profile", "dev", "mcp", "serve"]
```

> 注意:profile 走 `--profile`(bifu-cli 不读环境变量选 profile)。
> 验证:`codex mcp list`(或 `codex mcp get bifu`)应看到 `bifu`、`enabled`、`transport: stdio`。
> `Auth` 列显示 `Unsupported` 是正常的 —— stdio server 没有 OAuth 登录流程,不影响调用。

Codex 走「项目说明 + 命令行」的话:把技能装到目录,在 `AGENTS.md` 里引用,
或直接让 Codex 跑 `bifu-cli <命令>`:

```bash
bifu-cli skills install ./docs/bifu-skills   # 生成 SKILL.md,供 Codex 阅读/引用
```

### 2.3 Cursor / VS Code

```bash
bifu-cli skills install --client cursor   # → .cursor/rules/<name>.mdc
bifu-cli mcp setup --client cursor        # 或 --client vscode
```

### 2.4 HTTP 传输(Streamable HTTP)

除默认的 stdio(由客户端拉起进程)外,`mcp serve` 也支持 **Streamable HTTP**
传输 —— 适合远程/共享部署,或不便拉起子进程的客户端。

```bash
bifu-cli --profile dev mcp serve --http 127.0.0.1:8080        # 监听 http://127.0.0.1:8080/mcp
bifu-cli --profile dev mcp serve --http :8080 --path /mcp     # 自定义路径
bifu-cli --profile dev mcp serve --http 127.0.0.1:8080 --stateless  # 无状态模式(serverless 友好)
```

把这个 URL 注册到客户端:

```bash
# Claude Code
claude mcp add --transport http bifu http://127.0.0.1:8080/mcp
# Cursor / VS Code:在其 mcp.json 用 { "url": "http://127.0.0.1:8080/mcp" } 形式
```

> **安全**:HTTP 传输按**当前 profile 的登录会话**执行,且**没有逐请求鉴权** ——
> 务必绑定 `127.0.0.1`(本机),除非在可信网络;对外暴露需自行在前面加鉴权网关。

### 2.5 一键插件(Claude Code / Codex / Claude Desktop)

仓库 `plugins/bifu/` 把 **10 个 skills + MCP server** 打包成插件,同一份同时是
Claude Code 插件(`.claude-plugin/plugin.json`)和 Codex 插件(`.codex-plugin/plugin.json`),
内含三个按环境钉死的 MCP server:**`bifu-dev` / `bifu-staging` / `bifu-prod`**(装一次,
三环境都在;agent 按 server 名选,`bifu-prod` 是真金账户)。仓库根的
`.claude-plugin/marketplace.json` 与 `.agents/plugins/marketplace.json` 让公开镜像
`bifu-ai/bifu-cli` 直接当 marketplace。

> 前置:插件不内置二进制,先装 `bifu-cli`(curl/brew/npm)并为要用的环境 `config init` + `auth login`。
> 插件运行时调 **PATH 上的 `bifu-cli`**,记得保持它是最新版(`npm i -g @decodeex/bifu-cli@latest` /
> `brew upgrade --cask bifu-cli`),否则跑的是旧二进制。

```bash
# Claude Code(CLI / IDE 扩展)
/plugin marketplace add bifu-ai/bifu-cli
/plugin install bifu@bifu

# Codex —— CLI 与桌面版 Codex.app 共享 ~/.codex,装一次两端都生效
codex plugin marketplace add https://github.com/bifu-ai/bifu-cli
codex plugin add bifu@bifu
# 重启 Codex(CLI/App)生效。验证:
codex plugin list      # → bifu@bifu  installed, enabled  1.1.x
codex mcp list         # → bifu-dev / bifu-staging / bifu-prod  enabled
```

> `codex` 不在 PATH 时(只装了 Codex.app),用包内二进制:
> `/Applications/Codex.app/Contents/Resources/codex plugin add bifu@bifu`。

**Claude Desktop(桌面 App)不走上面这套插件系统**(它没有 `/plugin`),用下面两种方式之一。
前置同样要先装 `bifu-cli` 并对要用的环境 `auth login`(登录态存在 `~/.bifu-cli/config.yaml`)。

**方式 A — MCP 配置(推荐,已验证)**:一条命令写入 `claude_desktop_config.json`,**Cmd+Q 完全退出再重开** Claude Desktop 生效。

```bash
bifu-cli mcp setup --client claude-desktop --profiles dev,staging,prod
```

注册 `bifu-dev` / `bifu-staging` / `bifu-prod` 三个 server(按名选环境;只要单个就去掉 `--profiles`、加
`--profile dev`,server 名为 `bifu`)。写的是 bifu-cli 的**绝对路径**(GUI 不吃 PATH)。`--profiles` 对所有客户端通用(claude / codex / cursor / vscode / claude-desktop)。

**方式 B — `.mcpb` 扩展(若你的 Claude Desktop 版本支持扩展安装)**:从
[releases](https://github.com/decodeex/bifu-cli-releases/releases/latest) 下载对应平台的
`bifu_<os>_<arch>.mcpb`(自包含,已内置二进制),在 Claude Desktop 的扩展安装入口选中它,装时填环境。
本地自建:`make mcpb`。⚠ 运行二进制虽已内置,但仍需上面 `auth login` 生成的会话;若你的版本没有扩展安装入口或装不上,用方式 A。

### 可用技能(10 个)

`bifu-auth`(登录)、`bifu-config`(配置/Profile)、`bifu-spot`、`bifu-contract`、
`bifu-forex-trade`、`bifu-forex-account`、`bifu-payment`、`bifu-market-stream`、
`bifu-private-stream`、`bifu-orion`。

```bash
bifu-cli skills list           # 列出
bifu-cli skills show bifu-spot # 查看某个
```

---

## 3. 登录(认证)

所有认证命令共用 `auth login` 拿到的会话 cookie。先配置环境再登录。

```bash
bifu-cli config init --profile dev --env dev   # 创建并初始化 dev profile
bifu-cli config use dev                         # 设为当前 profile(否则不带 --profile 默认走 default)
```

### 扫码登录(推荐)

```bash
bifu-cli --profile prod auth login --device
```

终端打印二维码 + 链接 → 用**已登录的 Bifu App** 扫码/打开链接批准 → cookie 自动落盘。
**全程不输密码、不粘贴。**

### 邮箱密码登录

```bash
# 交互式
bifu-cli --profile dev auth login
# 非交互(CI),dev 验证码固定 123456
echo 123456 | bifu-cli --profile dev auth login --username you@example.com --password 'pw'
```

> 任何命令返回 **401** = 会话过期 → 重新 `auth login`。

### 注册 / 登出

```bash
# 注册:邮箱+密码 → 邮箱验证码 → 激活即登录(dev 验证码固定 123456)
bifu-cli --profile dev auth register --email you@example.com
echo 123456 | bifu-cli --profile dev auth register --email you@example.com --password 'Pw123!@#'

# 登出:服务端失效会话 + 清除本地 profile 的 cookie
bifu-cli --profile dev auth logout
```

> 注册成功后会话直接落盘(等于已登录),并把活跃 profile 切到该 profile。

---

## 4. 常用命令

> 所有命令都可加 `-p/--profile <env>` 指定环境、`-o json`/`--json` 输出 JSON、
> `-v` 调试。`--symbol`(现货)、`--contract`(合约)既可填**符号名**(`BTCUSDT`、
> `BTC/USDT`、`BTC-USDT`)也可填**数值 id**,符号名会经 `getMetaData` 自动解析并打印映射。
> 消歧:`/`→合约、`-`→现货、无分隔→优先合约。

### 4.1 余额 / 账户(只读)

```bash
bifu-cli payment balance                 # 法币储蓄余额(各币种 余额/可用/冻结)
bifu-cli payment balance --currency USD  # 只看某币种
bifu-cli payment balance --total         # 总资产(储蓄/外汇/跟单/体验金/RWA 分项 + 合计 USD)
bifu-cli payment forex-accounts          # 外汇账户列表(login/平台/余额/净值/可用保证金/杠杆)
bifu-cli spot balance                    # 现货各币种余额
bifu-cli contract account                # 合约账户(权益/可用/已用/未实现盈亏)
```

输出样例(`payment balance --total`):

```text
Total Balance
  Total (USD)  180651.4894
  Saving       108296.9594 (59.95%)
  Forex        69849.34 (38.67%)
  CopyTrade    2465.19 (1.36%)
  TrialBonus   0 (0%)
  RWA          0 (0%)
```

### 4.2 现货交易(spot)

```bash
# 下单:--symbol/--side/--size 必填(--symbol 可用符号名或数值 id)
bifu-cli spot order create --symbol BTCUSDT --side BUY --size 0.0001                  # 市价(符号名)
bifu-cli spot order create --symbol 90000001 --side SELL --type LIMIT --price 100000 --size 0.001
#   --type   MARKET | LIMIT | STOP_LIMIT      (默认 MARKET)
#   --price  限价价格 (默认 0)                  --tif GOOD_TIL_CANCEL|IMMEDIATE_OR_CANCEL|FILL_OR_KILL
#   --client-id 自定义订单 ID

# 查询
bifu-cli spot order get --order-id <id>         # 单个(仅活动单)
bifu-cli spot order list                        # 活动挂单
bifu-cli spot order list --symbol BTCUSDT        # 按符号(或 symbolId)过滤
bifu-cli spot order list --history --limit 20   # 历史订单

# 撤单
bifu-cli spot order cancel --order-id <id>
bifu-cli spot order cancel --all                # 撤全部(二次确认;-y 跳过)
bifu-cli spot order cancel --all --symbol BTCUSDT

# 余额
bifu-cli spot balance
```

> 常用 symbolId(dev/prod 同号):`90000001`=BTC-USDT、`90000002`=ETH-USDT、`90000004`=SOL-USDT。
> 直接用符号名(`BTCUSDT`)即可,无需记数字 id。

### 4.3 合约交易(contract)

方向:仓位 `--side LONG|SHORT` + 下单 `--order-side BUY|SELL`。开多=LONG+BUY,
平多=LONG+SELL `--reduce-only`,开空=SHORT+SELL,平空=SHORT+BUY。

```bash
# --contract 可用符号名(BTCUSDT)或数值 contractId,符号名自动解析并打印映射
bifu-cli contract order create --contract BTCUSDT --side LONG --order-side BUY --size 0.001       # 市价开多(符号名)
bifu-cli contract order create --contract 10000001 --side SHORT --order-side SELL --type LIMIT --price 95000 --size 0.001
bifu-cli contract order create --contract BTCUSDT --side LONG --order-side SELL --size 0.001 --reduce-only  # 平多
#   --margin-mode SHARED|ISOLATED  --tif ...  --trigger-price/--trigger-type(条件单)

bifu-cli contract order get --order-id <id>
bifu-cli contract order list [--contract BTCUSDT] [--history --limit 20]
bifu-cli contract order cancel --order-id <id>     # 或 --all [--contract BTCUSDT]
bifu-cli contract position list [--contract BTCUSDT]
bifu-cli contract account
```

> 后端无改单接口:改价/改量请撤单后重下。常用 contractId(dev/prod 同号):`10000001`=BTC 永续(BTC/USDT)。

### 4.4 外汇(forex,MT5 / TradFi)

订单类型:`buy`/`sell`(市价);`buyLimit`/`sellLimit`/`buyStop`/`sellStop`(挂单)。
下单**按 `--login-id` 对应账户的平台自动路由**(MT5 或 TradFi/Fortex),命令一致,
平台/login 见 `bifu-cli payment forex-accounts`。

> **交易时段**:传统外汇/金属(`EURUSD`、`XAUUSD` 等)周末休市,市价单会被拒
> (MT5 返回 `[10017] 开单维护`、TradFi 返回 `[500] Quote Not Available`)。
> **TradFi 的加密 CFD(如 `BTCUSDT`)7×24 可交易,周末也能真成交。**

```bash
# ── 通用:下单(--login-id 必填,--symbol 必填) ──
bifu-cli forex order create --login-id 90390034 --symbol EURUSD --type buy --volume 0.01
bifu-cli forex order create --login-id 90390034 --symbol EURUSD --type buyLimit --price 1.05 --volume 0.01 --sl 1.03 --tp 1.09

bifu-cli forex order modify --login-id 90390034 --order-id <id> --sl 1.03 --tp 1.09
bifu-cli forex order close  --login-id 90390034 --order-id <id>
bifu-cli forex order cancel --login-id 90390034 --order-id <id>   # 挂单/pending
bifu-cli forex order history --login-id 90390034 --from 2026-01-01 --to 2026-12-31
bifu-cli forex positions --login-id 90390034     # 持仓 + 未触发挂单

# ── TradFi(Fortex):加密 7×24,周末可用 ──
#   TradFi 账户的 login 见 forex-accounts(PLATFORM=TradFi)。加密符号用 BTCUSDT。
bifu-cli forex order create --login-id 800000177 --symbol BTCUSDT --type buy --volume 0.01   # 市价开多(真成交)
bifu-cli forex positions   --login-id 800000177                                              # 看持仓(STATE=ORDER_STATE_FILLED)
bifu-cli forex order close --login-id 800000177 --order-id <ticket>                          # 平仓
#   TradFi 实时报价见 §4.6 的 `ws pushgw --tradfi --market-watch`(可发现可交易符号)

# ── 开户(--password 必填;--platform mt5|tradfi,--type live|demo,--leverage,--currency) ──
bifu-cli forex account create --platform tradfi --currency USD --leverage 100 --password 'Pass123!'
bifu-cli payment forex-accounts          # 列出账户 + login id + 平台
```

### 4.5 orion 信号订阅

```bash
bifu-cli orion price                          # 订阅定价(公开)
bifu-cli orion signal                         # 当前信号 + 活跃 buy/sell 计划(需订阅)
bifu-cli orion signal-history --days 30       # 历史信号(--days 回溯天数,--page 第几个窗口)
bifu-cli orion signal-history --days 90 --page 2
bifu-cli orion subscription                   # 当前订阅状态/有效期
```

> `signal-history` 每条含 `type`(buy/sell)、`entry`、`sl`、`pt1`、`pt2`。
> `--days` 是天数窗口:结果空就调大 `--days`。

### 4.6 WebSocket 实时(`ws`,Ctrl-C 结束)

```bash
bifu-cli ws market --channels ticker.BTCUSDT             # 公共行情(无需登录;符号自动解析为数字 ID)
bifu-cli ws market --channels ticker.10000001            # 也可直接用数字 instrumentId
bifu-cli ws market --channels ticker.all                 # 全部 ticker
bifu-cli ws market --channels ticker.BTCUSDT,depth.SOLUSDT.15
# 符号消歧:BTC/USDT→合约, BTC-USDT→现货, BTCUSDT(无分隔)→优先合约

# 外汇推送网关(pushgw):默认 MT5,加 --tradfi 走 TradFi(Fortex)
bifu-cli ws pushgw --market-watch                        # MT5 全品种行情
bifu-cli ws pushgw --tradfi --market-watch               # TradFi 全品种实时报价(含 BTCUSDT,周末有报价)
bifu-cli ws pushgw --tradfi --symbols BTCUSDT,EURUSD     # TradFi 指定品种 tick
bifu-cli ws pushgw --tradfi --login-ids 800000177        # TradFi 账户订单/持仓事件
bifu-cli ws config show                                  # 查看各 WS 端点 URL(含 TradFi WS)
```

> `ws private` / `ws private --spot`(私有交易事件)当前需服务端鉴权握手,暂不可用。

---

## 5. 给代理的实用提示

| 场景 | 用法 |
|------|------|
| 机器可读输出 | `--json`(= `-o json`),便于解析 |
| 跳过危险操作确认 | `-y` / `--yes`(撤销全部挂单等) |
| 指定环境/账户 | `-p/--profile dev\|staging\|prod` |
| 调试请求 | `-v`(Cookie/Authorization 自动脱敏) |

- 下单类是**真实交易**;`order cancel --all`、大额操作请确认后再加 `-y`。
- `--symbol`/`--contract` 可填**符号名**(`BTCUSDT` 等)或**数值 id**;符号名经
  `getMetaData` 自动解析。完整列表见 `GET /api/v1/public/meta/getMetaData`。
- 私有交易 WS(`ws private`)当前需服务端鉴权握手,暂不可用;公共行情 `ws market` 可用。

---

## 6. 安全

- 会话 cookie 存于 `~/.bifu-cli/config.yaml`(0600),**不会回显到终端**,`-v` 日志自动脱敏。
- 无任何本地生成 cookie 的命令;会话只来自后端(`auth login` / `auth register` 激活);`auth logout` 清除。
