---
name: bifu-contract
description: Contract/futures trading — orders, positions and account assets.
auth: required
---

# bifu-cli: contract (futures) trading

Activate for perpetual/futures orders, positions, and contract account info.
Requires a logged-in profile (see bifu-auth).

`--contract` is the numeric **contractId** (not "BTC-USDT-SWAP"). On dev,
`10000001` = BTC perpetual.

Direction model: position side `--side LONG|SHORT`, order side
`--order-side BUY|SELL`. Open long = LONG+BUY; close long = LONG+SELL `--reduce-only`;
open short = SHORT+SELL; close short = SHORT+BUY.

## Account / positions

```bash
bifu-cli contract account
bifu-cli contract position list [--contract 10000001]
```

## Create order

```bash
# Market open long 0.001
bifu-cli contract order create --contract 10000001 --side LONG --order-side BUY --size 0.001
# Limit open short
bifu-cli contract order create --contract 10000001 --side SHORT --order-side SELL --type LIMIT --price 95000 --size 0.001
# Market close long (reduce-only)
bifu-cli contract order create --contract 10000001 --side LONG --order-side SELL --size 0.001 --reduce-only
```

## Query / cancel

```bash
bifu-cli contract order get --order-id 7594...     # active orders only
bifu-cli contract order list [--contract 10000001]
bifu-cli contract order list --history --limit 20
bifu-cli contract order cancel --order-id 7594...
bifu-cli contract order cancel --all [--contract 10000001]   # destructive: prompts unless -y
```

## Notes
- No modify endpoint — to change price/size, cancel then re-create.
- Unrealized PnL is colored (green/red) in table output; `--json` for raw values.
