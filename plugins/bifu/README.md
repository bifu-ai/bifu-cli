# bifu — Claude Code & Codex plugin

Brings the **BifuFX** trading platform into AI agents: read balances / positions /
orders and place or cancel spot & contract orders, plus skills for auth, config,
forex (MT5/TradFi), payments, WebSocket streams and orion signals.

This one plugin ships **both** a Claude Code manifest (`.claude-plugin/plugin.json`)
and a Codex manifest (`.codex-plugin/plugin.json`), sharing the same `skills/` and
MCP server.

## Prerequisite — install the CLI

The plugin invokes the `bifu-cli` binary (it is **not** bundled). Install it, then
create and log into the environment profiles you want (profile names must match
`dev` / `staging` / `prod`):

```bash
curl -fsSL https://cli.bifu.dev/install.sh | bash   # or: brew install decodeex/tap/bifu-cli

bifu-cli config init --profile dev     --env dev     && bifu-cli --profile dev     auth login
bifu-cli config init --profile staging --env staging && bifu-cli --profile staging auth login
bifu-cli config init --profile prod    --env prod    && bifu-cli --profile prod    auth login
```

You only need the environments you'll actually use — an env you don't log into
just returns `unauthorized` for its tools (harmless).

## Install the plugin

```bash
# Claude Code
/plugin marketplace add bifu-ai/bifu-cli
/plugin install bifu@bifu

# Codex
codex plugin marketplace add https://github.com/bifu-ai/bifu-cli
codex plugin add bifu@bifu
```

## What you get

- **Three MCP servers — one per environment**, each pinned to its profile:
  - `bifu-dev` → `bifu-cli --profile dev mcp serve` (test funds)
  - `bifu-staging` → `--profile staging`
  - `bifu-prod` → `--profile prod` (**real money**)

  Each exposes the same 11 tools (get/list balances, positions, orders;
  create/cancel spot & contract orders). The agent picks the environment by
  server name (e.g. Claude Code calls `mcp__bifu-dev__get_payment_balance`).
- **10 skills** — `bifu-auth`, `bifu-config`, `bifu-spot`, `bifu-contract`,
  `bifu-forex-trade`, `bifu-forex-account`, `bifu-payment`, `bifu-market-stream`,
  `bifu-private-stream`, `bifu-orion`.

## Environment / safety

All three environments are exposed so you can switch without reconfiguring — tell
the agent which to use, or just don't log into the ones you don't want. **`bifu-prod`
places real orders on the live account**, so be explicit when you use it. To narrow
the set, delete unwanted servers from the manifest (`.claude-plugin/plugin.json`
and `.mcp.json`).

> The skills under `skills/` are generated from the canonical `skills/` at the repo
> root — run `make plugins-sync` after changing a skill; do not hand-edit them here.
