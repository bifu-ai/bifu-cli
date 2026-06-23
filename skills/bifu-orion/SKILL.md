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
bifu-cli orion price                              # subscription pricing tiers (public)
bifu-cli orion signal                             # current signal + active buy/sell calls (needs subscription)
bifu-cli orion signal-history --page 1 --size 20  # past signals (details need a subscription)
bifu-cli orion subscription                       # current subscription status / validity (needs login)
```

## Notes
- A `SignalPolicy`/history item is a call: `type` (buy/sell), `entry`, `sl`
  (stop loss), `pt1`/`pt2` (targets), `trend`, `product`.
- No active subscription → `signal` / `signal-history` return a friendly
  "subscription required" message (not an error).
- `-o json` for machine-readable output.
