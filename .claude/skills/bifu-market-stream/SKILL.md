---
name: bifu-market-stream
description: Stream public market data over WebSocket — tickers, depth/order books (no auth).
auth: none
---

# bifu-cli: public market data stream

Activate to stream live public market data. No authentication required. For
private trading events, use bifu-private-stream.

```bash
bifu-cli ws market --channels ticker.BTCUSDT          # symbol — auto-resolved
bifu-cli ws market --channels ticker.10000001         # numeric instrumentId
bifu-cli ws market --channels ticker.all              # every ticker
bifu-cli ws market --channels ticker.BTCUSDT,depth.SOLUSDT.15
```

Channels are `<type>.<instrumentId>[.<extra>]`. The `<instrumentId>` may be a
**numeric ID** or a **symbol name** — the CLI resolves names via `getMetaData`
and prints the mapping. Disambiguation: `/` → contract (`BTC/USDT`), `-` → spot
(`BTC-USDT`), no separator → contract first then spot (`BTCUSDT`). `ticker.all`
streams every ticker. The numeric ID is still accepted and needs no lookup.

## Endpoints

```bash
bifu-cli ws config show                # resolved market/private/pushgw/tradfi WS URLs
bifu-cli ws config set --market-url wss://quote.bifu.dev/api/v1/public/ws
```

## Notes
- Streaming runs until interrupted (Ctrl-C). `-o json` emits raw frames for parsing.
- WS URLs come from the active profile (`config init --env`).
