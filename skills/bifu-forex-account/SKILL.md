---
name: bifu-forex-account
description: Create and list forex trading accounts (MT5 or TradFi/Fortex).
auth: required
---

# bifu-cli: forex account management

Activate to create a new MT5/TradFi forex trading account or list existing ones
(and their login ids). Requires a logged-in profile (bifu-auth). To place orders
on an account, use bifu-forex-trade.

## Create

```bash
# TradFi/Fortex (mt_type=3), defaults to live; user auto-enrolled into tradfi whitelist
bifu-cli forex account create --platform tradfi --currency USD --leverage 100 --password 'Pass123!'
# MT5 (mt_type=2), demo
bifu-cli forex account create --platform mt5 --type demo --currency USD --leverage 100 --password 'Pass123!'
```

Flags: `--platform mt5|tradfi`, `--type live|demo`, `--currency`, `--leverage`,
`--password`, `--no-whitelist` (skip the tradfi auto-enroll).

## List

```bash
bifu-cli payment forex-accounts        # all forex accounts + login ids
```

## Notes
- TradFi accounts require the user to be in the tradfi whitelist (auto-enrolled
  unless `--no-whitelist`).
- The returned login id is what bifu-forex-trade uses as `--login-id`.
