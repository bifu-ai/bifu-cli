---
name: bifu-orion
description: Orion signal subscription — read subscription pricing, current signals, signal history, and subscription status.
auth: partial
---

# bifu-cli: orion signals (read-only)

Activate for the orion trade-signal subscription product. `price` is public;
`signal` / `signal-history` show details only with an active subscription;
`subscription` needs login (see bifu-auth). Not a generic market-quote feed —
orion provides curated buy/sell signal calls (entry / stop / targets / trend).

```bash
bifu-cli orion price                          # subscription pricing tiers (public)
bifu-cli orion signal                         # current signal + active buy/sell calls (needs subscription)
bifu-cli orion signal-history --days 30       # past calls; pagination is a DAY window
bifu-cli orion signal-history --days 90 --page 2   # the 90-day window before the most recent
bifu-cli orion subscription                   # current subscription status / validity (needs login)
```

## Notes
- A call is `type` (buy/sell), `entry`, `sl` (stop loss), `pt1`/`pt2` (targets),
  `trend`, `product`.
- `signal-history` paginates by DAY window: `--days` = look-back days, `--page`
  = which window (1 = most recent). Empty result = no calls in that window → use
  a larger `--days`. `total` is the all-time signal count.
- `signal` needs an active subscription (otherwise it reports "subscribe to view").
- `-o json` for machine-readable output.
