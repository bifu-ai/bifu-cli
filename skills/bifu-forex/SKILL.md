---
name: bifu-forex
description: Forex/MT5/TradFi trading — market & pending orders, positions, history, account creation.
auth: required
---

# bifu-cli: forex (MT5 / TradFi) trading

Activate for MT5/Fortex(TradFi) forex orders, positions, history, and forex
account management. Requires a logged-in profile (see bifu-auth).

Order types: `buy`, `sell` (market); `buyLimit`, `sellLimit`, `buyStop`,
`sellStop` (pending). `--login-id` is the MT5/TradFi trading account login.

## Account

```bash
# Create a forex account (mt5 default live; tradfi requires whitelist, auto-enrolled)
bifu-cli forex account create --platform tradfi --currency USD --leverage 100 --password 'Pass123!'
bifu-cli forex account create --platform mt5 --type demo --currency USD --leverage 100 --password 'Pass123!'
bifu-cli payment forex-accounts            # list forex accounts (see bifu-payment)
```

## Orders

```bash
# Market buy 0.01 lots EURUSD
bifu-cli forex order create --login-id 90390034 --symbol EURUSD --type buy --volume 0.01
# Pending order with SL/TP
bifu-cli forex order create --login-id 90390034 --symbol EURUSD --type buyLimit --price 1.05 --volume 0.01 --sl 1.03 --tp 1.09
# Modify / close / cancel
bifu-cli forex order modify  --login-id 90390034 --order-id 12345 --sl 1.03 --tp 1.09
bifu-cli forex order close   --login-id 90390034 --order-id 12345
bifu-cli forex order cancel  --login-id 90390034 --order-id 12345
# History
bifu-cli forex order history --login-id 90390034 --from 2026-01-01 --to 2026-12-31
```

## Positions

```bash
bifu-cli forex positions --login-id 90390034
```

## Notes
- Forex endpoints go through the payment service; the same session cookie applies.
- TradFi(Fortex) accounts need the user to be in the tradfi whitelist (auto-enrolled
  unless `--no-whitelist`).
