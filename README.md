# bifu-cli

BifuFX 交易平台命令行工具，支持现货、合约、外汇(MT5)交易，以及 WebSocket 实时行情订阅。

设计灵感来自 [Solana CLI](https://docs.solanalabs.com/cli)，采用多 Profile 配置管理，可在 custom/dev/staging/prod 环境之间快速切换。

---

## 安装

```bash
# 从源码编译
git clone <repo>
cd bifu-cli
make build          # 输出 bin/bifu-cli

# 或安装到 $GOPATH/bin
make install
```

**环境要求**: Go 1.24+

---

## 快速开始

```bash
# 1. 初始化 dev 环境配置
bifu-cli config init --env dev

# 2. 登录并自动保存 Cookie（推荐）
bifu-cli --profile dev auth login
# 按提示输入邮箱密码和验证码，Cookie 自动写入 profile

# 3. 查看当前配置
bifu-cli config get

# 4. 现货查询余额
bifu-cli spot balance

# 5. 外汇市价买入
bifu-cli forex order create --login-id 90390034 --symbol EURUSD --type buy --volume 0.01
```

---

## 全局 Flag

所有命令均支持以下全局选项：

| Flag | 简写 | 默认值 | 说明 |
|------|------|--------|------|
| `--profile` | `-p` | `default` | 使用的配置 Profile |
| `--output` | `-o` | `table` | 输出格式：`table` / `json` / `plain` |
| `--verbose` | `-v` | `false` | 开启调试输出 |

---

## config — 配置管理

配置文件路径：`~/.bifu-cli/config.yaml`，支持多 Profile，类似 AWS CLI。

### 初始化 Profile

```bash
# 使用 dev 预设（默认）
bifu-cli config init --env dev

# 使用 staging 预设
bifu-cli config init --env staging

# 创建自定义 profile 并使用 prod 预设
bifu-cli config init --profile myprod --env prod
```

**环境预设地址**

| 环境 | Base URL | Market WS | Private WS (contract) | Private WS (spot) |
|------|----------|-----------|-----------------------|-------------------|
| `dev` | `https://fxapi.bifu.dev` | `wss://quote.bifu.dev` | `wss://contract.bifu.dev` | `wss://spot.bifu.dev` |
| `staging` | `https://fxapi.staging.bifu.co` | `wss://quote.staging.bifu.co` | `wss://contract.staging.bifu.co` | `wss://spot.staging.bifu.co` |
| `prod` | `https://fxapi.bifu.co` | `wss://quote.bifu.co` | `wss://contract.bifu.live` | `wss://spot.bifu.live` |

### 修改配置

> 现货 / 合约 / 支付 / 外汇所有认证接口统一使用 `bifu-cli auth login` 获取的会话 Cookie，无需单独配置 API Key。

```bash
# 设置 Cookie 认证（现货/合约/支付/外汇通用，推荐用 auth login 自动写入）
bifu-cli config set --auth-cookie "user_auth_name=eyJhbGc..."

# 设置外汇 HTTP 地址（可选）
bifu-cli config set --forex-http https://fxapi.bifu.dev

# 修改 Base URL
bifu-cli config set --base-url https://fxapi.bifu.dev

# 指定特定 profile 修改
bifu-cli config set --profile staging --auth-cookie "..."
```

### Profile 管理

```bash
# 列出所有 Profile
bifu-cli config list

# 查看当前 Profile 详情
bifu-cli config get

# 切换到其他 Profile
bifu-cli config use staging

# 删除 Profile
bifu-cli config delete myenv
```

---

## auth — 认证管理

外汇（forex）和支付（payment）接口使用 `user_auth_name` Cookie 认证。

### auth login — 邮箱密码登录（推荐）

通过账号密码 + 邮件验证码完成登录，Cookie 自动写入 profile，无需手动复制。

```bash
# 交互式登录（密码不显示在屏幕）
bifu-cli --profile dev auth login

# 预填用户名，密码仍隐藏输入
bifu-cli --profile dev auth login --username user@example.com

# 完全非交互式（CI 场景）
bifu-cli --profile dev auth login --username user@example.com --password 'MyPass'
```

**登录流程：**
1. 输入邮箱和密码（密码不回显）
2. 服务端发送验证码到邮箱
3. 输入收到的验证码
4. Cookie 自动保存到 profile 的 `auth_cookie` 字段（有效期 30 天）

> **注意**：Dev 环境验证码固定为 `123456`。

### auth cookie — Cookie 工具

> 适用于已知 Cookie 加密 key 的环境（dev / custom）。生产环境推荐使用 `auth login`。

#### 生成并保存 Cookie（custom 环境）

```bash
# 生成 cookie 并保存到当前激活 profile（env 自动从 profile 名推断）
bifu-cli auth cookie set 620640738

# 指定 env
bifu-cli auth cookie set 620640738 --env dev
bifu-cli auth cookie set 620640738 --env staging

# 针对特定 profile 操作
bifu-cli --profile staging auth cookie set 620640738 --env staging
```

#### 仅生成（不保存）

```bash
bifu-cli auth cookie encode 620640738 --env dev
```

#### 解码 Cookie

```bash
bifu-cli auth cookie decode "yHjCFUQ2jFBQ..."
# 输出:
#   uid : 620640738
#   env : dev
#   raw : 620640738=dev=C8DXTLEX=1770620640
```

---

## spot — 现货交易

### 查询余额

```bash
bifu-cli spot balance

# JSON 输出
bifu-cli spot balance -o json
```

> `--symbol` 传的是**数值 symbolId**（不是 `BTCUSDT` 这种名称）。常用 dev 现货 symbolId：
> `90000001` = BTC-USDT、`90000002` = ETH-USDT、`90000004` = SOL-USDT、`90000010` = DOGE-USDT。
> 完整列表见 `GET /api/v1/public/meta/getMetaData` 的 `symbolList`。

### 下单

```bash
# 市价买入 0.0001 BTC（symbolId 90000001）
bifu-cli spot order create --symbol 90000001 --side BUY --size 0.0001

# 限价卖出
bifu-cli spot order create --symbol 90000001 --side SELL --type LIMIT --price 100000 --size 0.001

# 指定 TIF
bifu-cli spot order create --symbol 90000002 --side BUY --size 0.1 --tif IMMEDIATE_OR_CANCEL
```

| Flag | 说明 | 默认值 |
|------|------|--------|
| `--symbol` / `-s` | 数值 symbolId（必填，见 getMetaData） | — |
| `--side` | BUY / SELL（必填） | — |
| `--size` | 数量，按 base 币计（必填） | — |
| `--type` | MARKET / LIMIT / STOP_LIMIT | `MARKET` |
| `--price` | 限价价格 | `0` |
| `--tif` | 时效：GOOD_TIL_CANCEL / IMMEDIATE_OR_CANCEL / FILL_OR_KILL | `GOOD_TIL_CANCEL` |
| `--client-id` | 自定义订单 ID | — |

### 查询订单

```bash
# 查询单个挂单（仅返回活动单；已成交/已撤的用 --history 查）
bifu-cli spot order get --order-id 759472786740609900

# 查看所有挂单
bifu-cli spot order list

# 指定 symbolId
bifu-cli spot order list --symbol 90000001

# 查看历史订单
bifu-cli spot order list --history --limit 20
```

### 撤单

```bash
# 撤销指定订单
bifu-cli spot order cancel --order-id 759472786740609900

# 撤销所有挂单
bifu-cli spot order cancel --all

# 撤销指定 symbolId 的所有挂单
bifu-cli spot order cancel --all --symbol 90000001
```

---

## contract — 合约交易

### 查询账户

```bash
bifu-cli contract account
```

### 查看持仓

```bash
bifu-cli contract position list

# 按合约过滤
bifu-cli contract position --contract 10000001
```

### 下单

> `--contract` 传的是**数值 contractId**（不是 `BTC-USDT-SWAP`）。dev 上 `10000001` = BTC 永续。
> 仓位方向用 `--side LONG|SHORT`，下单方向用 `--order-side BUY|SELL`：开多=LONG+BUY，平多=LONG+SELL（加 `--reduce-only`），开空=SHORT+SELL，平空=SHORT+BUY。

```bash
# 市价开多 0.001
bifu-cli contract order create --contract 10000001 --side LONG --order-side BUY --size 0.001

# 限价开空
bifu-cli contract order create --contract 10000001 --side SHORT --order-side SELL --type LIMIT --price 95000 --size 0.001

# 市价平多（reduce-only）
bifu-cli contract order create --contract 10000001 --side LONG --order-side SELL --size 0.001 --reduce-only
```

### 查询订单

```bash
bifu-cli contract order get --order-id 759471776194363801   # 仅活动单
bifu-cli contract order list
bifu-cli contract order list --contract 10000001
bifu-cli contract order list --history --limit 20
```

### 撤单

```bash
# 撤销指定订单
bifu-cli contract order cancel --order-id 759471776194363801

# 撤销所有挂单
bifu-cli contract order cancel --all

# 撤销指定合约的所有挂单
bifu-cli contract order cancel --all --contract 10000001
```

> 后端不支持改单（无 modify 接口）；如需改价/改量，撤单后重新下单。

---

## payment — 资金管理

### 查询余额

```bash
# 查询储蓄账户余额
bifu-cli payment balance

# 查询所有账户总余额
bifu-cli payment balance --total

# 指定货币
bifu-cli payment balance --currency USD
bifu-cli payment balance --total --currency USDT
```

### 查看外汇账户列表

```bash
bifu-cli payment forex-accounts
```

### 统一划转

`payment unified-transfer` 支持任意两种账户类型之间的资金划转，通过 `POST /payment/v2/transfer` 实现。

| 账户类型 | 说明 | 所需参数 |
|---------|------|---------|
| `SAVING` | 法币储蓄/钱包账户 | `--currency` |
| `FOREX` | MT5 外汇账户 | `--currency` |
| `FUNDING` | 加密资金账户 | `--coin-id` |
| `SPOT` | 加密现货账户 | `--coin-id` |
| `CONTRACT` | 加密合约账户 | `--coin-id` |
| `EARN` | 理财账户 | `--coin-id` 或 `--currency` |

```bash
# 储蓄 → 外汇（出金到 MT5）
bifu-cli payment unified-transfer --from SAVING --to FOREX --amount 1000 --currency USD

# 外汇 → 储蓄（从 MT5 提款）
bifu-cli payment unified-transfer --from FOREX --to SAVING --amount 500 --currency USD

# 储蓄 → 现货
bifu-cli payment unified-transfer --from SAVING --to SPOT --amount 100 --currency USD

# 储蓄 → 合约
bifu-cli payment unified-transfer --from SAVING --to CONTRACT --amount 100 --currency USD

# 资金账户 → 现货（coin-id 见下方说明）
bifu-cli payment unified-transfer --from FUNDING --to SPOT --amount 10 --coin-id 2

# 现货 → 合约
bifu-cli payment unified-transfer --from SPOT --to CONTRACT --amount 10 --coin-id 2

# 合约 → 资金账户
bifu-cli payment unified-transfer --from CONTRACT --to FUNDING --amount 10 --coin-id 2
```

> **coin-id 说明**：coin-id 是数值币种 ID，因环境而异，查 `getMetaData`。dev 环境 **`2` = USDT、`1` = BTC**。从法币 SAVING 划入加密账户时用 `--currency USD` 即可（自动换成 USDT 入账到对应 coin）。

| Flag | 说明 | 示例 |
|------|------|------|
| `--from` | 转出账户类型（必填） | `SAVING` |
| `--to` | 转入账户类型（必填） | `FOREX` |
| `--amount` | 划转金额（必填） | `100` |
| `--currency` | 法币代码（法币类账户必填） | `USD` |
| `--coin-id` | 加密币数值 ID（加密类账户必填，dev: 2=USDT） | `2` |
| `--comment` | 备注（可选） | `recharge` |

---

## forex — 外汇(MT5 / TradFi)交易

> 外汇接口通过 Cookie 认证，需先执行 `bifu-cli auth login` 登录或手动设置 Cookie。

> **MT5 与 TradFi(Fortex) 双平台**：后端按账户的 `mt_type` 自动路由（`2`=MT5，`3`=TradFi/Fortex）。
> 所有 `forex order` 命令对两种平台通用——只要传对应账户的 `--login-id` 即可，无需切换命令。
> 用 `bifu-cli payment forex-accounts` 的 **PLATFORM** 列区分账户平台。

### 创建账户

```bash
# 创建 TradFi(Fortex) 账户（mt_type=3）。需用户在 tradfi 白名单内——
# CLI 会自动为当前用户设置 tradfi-whitelist 属性（POST /user/set_user_attribute），无需手动操作。
bifu-cli forex account create --platform tradfi --type live --currency USD --leverage 100 --password 'Pass123!'
# 加 --no-whitelist 可跳过自动白名单

# 创建 MT5 demo 账户
bifu-cli forex account create --platform mt5 --type demo --currency USD --leverage 100 --password 'Pass123!'
```

> 创建成功会回显 **Login**（下单用）和 **Account ID**（内部 id，划转入金用）。

### 账户充值/划转

```bash
# 储蓄 → TradFi 外汇账户入金（已验证）
#   --to-account-id 传创建时回显的 Account ID（内部 id，非 login）
#   --from-account-id 传 USD 储蓄账户 id（见 payment balance 的 id）
#   --mt-type 3 = TradFi（MT5 用 2 或省略）
bifu-cli payment unified-transfer --from SAVING --to FOREX --amount 100 --currency USD \
  --from-account-id <savingAccountId> --to-account-id <forexAccountId> --mt-type 3
```

> 注意：saving→forex 划转要求 **live** 账户且币种匹配（demo 账户后端单独处理）。

### 订单类型

> MT5 用小写类型名（buy/sell/buyLimit…）。TradFi 也可直接用 `--order-type Market|Limit|Stop|StopLimit` + `--side Buy|Sell`（不传则由 `--type` 自动推导）。

| 类型 | 说明 | 成交条件 |

| 类型 | 说明 | 成交条件 |
|------|------|----------|
| `buy` | 市价买入 | 立即成交 |
| `sell` | 市价卖出 | 立即成交 |
| `buyLimit` | 买入限价单 | 价格 ≤ 指定价格时成交 |
| `sellLimit` | 卖出限价单 | 价格 ≥ 指定价格时成交 |
| `buyStop` | 买入止损单 | 价格 ≥ 指定价格时成交 |
| `sellStop` | 卖出止损单 | 价格 ≤ 指定价格时成交 |

### 下单

```bash
# 市价买入（设置止损止盈）
bifu-cli forex order create \
  --login-id 90390034 \
  --symbol EURUSD \
  --type buy \
  --volume 0.01 \
  --sl 1.0200 \
  --tp 1.0800

# 买入限价单（带过期时间）
bifu-cli forex order create \
  --login-id 90390034 \
  --symbol EURUSD \
  --type buyLimit \
  --volume 0.01 \
  --price 1.0500 \
  --expiration "2026-12-31T18:00:00Z"

# TradFi(Fortex) 账户下单（login-id 为 mt_type=3 账户，自动路由到 TradFi）
bifu-cli forex order create --login-id 800000175 --symbol EURUSD --type buy --volume 0.01
# 或用 TradFi 原生字段
bifu-cli forex order create --login-id 800000175 --symbol EURUSD --order-type Market --side Buy --lots 0.01
```

### 修改订单

```bash
# 修改止损止盈
bifu-cli forex order modify \
  --login-id 90390034 \
  --order-id 16424091 \
  --sl 1.0300 \
  --tp 1.0900

# 修改挂单价格
bifu-cli forex order modify \
  --login-id 90390034 \
  --order-id 16424091 \
  --price 1.0600
```

### 平仓 / 取消

```bash
# 全部平仓
bifu-cli forex order close --login-id 90390034 --order-id 16424091

# 部分平仓（指定手数）
bifu-cli forex order close --login-id 90390034 --order-id 16424091 --volume 0.005

# 取消挂单（未成交委托）
bifu-cli forex order cancel --login-id 90390034 --order-id 16424091
```

### 批量操作

```bash
# 批量平仓
bifu-cli forex order batch-close \
  --login-id 90390034 \
  --order-ids "16424091,16424092,16424093"

# 批量取消挂单
bifu-cli forex order batch-cancel \
  --login-id 90390034 \
  --order-ids "16424091,16424092,16424093"
```

### 历史订单

```bash
# 查询历史
bifu-cli forex order history \
  --login-id 90390034 \
  --from 2026-01-01 \
  --to 2026-12-31

# 分页
bifu-cli forex order history \
  --login-id 90390034 \
  --from 2026-01-01 \
  --to 2026-12-31 \
  --page-size 50 \
  --page-num 0
```

---

## ws — WebSocket 实时订阅

### 配置 WebSocket 地址

```bash
# 查看当前 WS 配置
bifu-cli ws config show

# 同时修改 Market WS base URL 和路径（自动合并为完整 URL）
bifu-cli ws config set --market-url wss://quote.bifu.dev --ws-market /api/v1/public/ws

# 仅修改 Market WS base URL（路径保持不变）
bifu-cli ws config set --market-url wss://quote.bifu.dev

# 修改 Private WS（直接设置完整 URL）
bifu-cli ws config set --private-url wss://contract.bifu.dev/api/v1/private/contract/ws

# 修改 Spot Private WS
bifu-cli ws config set --ws-private-spot wss://spot.bifu.dev/api/v1/private/spot/ws

# 修改 Pushgw WS
bifu-cli ws config set --pushgw-ws wss://fxapi.bifu.dev --pushgw-path /pushgw/ws
```

### 订阅市场行情

Channel 格式使用 contractId（数字 ID），不是 symbol 名称。

```bash
# 订阅所有合约 ticker
bifu-cli ws market --channels ticker.all

# 订阅单个合约（10000001 = BTC-USDT-SWAP）
bifu-cli ws market --channels ticker.10000001

# 多个 channels
bifu-cli ws market --channels ticker.10000001,depth.10000001.15

# 美化 JSON 输出
bifu-cli ws market --channels ticker.all --pretty
```

### 订阅私有事件（订单/持仓推送）

```bash
# 合约私有流（默认）
# 端点: wss://contract.bifu.dev/api/v1/private/contract/ws
bifu-cli ws private
bifu-cli ws private --pretty

# 现货私有流
# 端点: wss://spot.bifu.dev/api/v1/private/spot/ws
bifu-cli ws private --spot
bifu-cli ws private --spot --pretty
```

### Push 网关

```bash
bifu-cli ws pushgw --pretty
```

> 按 `Ctrl+C` 断开连接。

---

## 多环境使用示例

```bash
# 创建多个环境 profile
bifu-cli config init --profile dev --env dev
bifu-cli config init --profile staging --env staging
bifu-cli config init --profile prod --env prod

# 各 profile 登录（自动获取并保存 Cookie）
bifu-cli --profile dev auth login
bifu-cli --profile staging auth login
bifu-cli --profile prod auth login

# 使用指定 profile 执行命令（不切换默认值）
bifu-cli -p dev spot balance
bifu-cli -p prod forex order history --login-id 12345 --from 2026-01-01 --to 2026-05-01

# 切换默认 profile
bifu-cli config use dev
```

---

## 输出格式

```bash
# 表格（默认）
bifu-cli spot balance

# JSON
bifu-cli spot balance -o json

# Plain（每行 key=value）
bifu-cli spot balance -o plain
```

---

## Makefile 命令

```bash
make build      # 编译到 bin/bifu-cli
make install    # 安装到 $GOPATH/bin
make tidy       # go mod tidy
make clean      # 清理编译产物
make test       # 运行单元测试
make lint       # 静态分析（需安装 staticcheck）
```
