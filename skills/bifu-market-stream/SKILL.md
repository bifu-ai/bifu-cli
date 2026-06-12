---
name: bifu-market-stream
description: Stream public market data over WebSocket — tickers, depth/order books (no auth).
auth: none
---

# bifu-cli: public market data stream

Activate to stream live public market data. No authentication required. For
private trading events, use bifu-private-stream.

```bash
bifu-cli ws market --channels ticker.BTCUSDT
bifu-cli ws market --channels ticker.BTCUSDT,depth.BTCUSDT
```

Channels are `<type>.<symbol>` (e.g. `ticker.BTCUSDT`, `depth.BTCUSDT`).

## Endpoints

```bash
bifu-cli ws config show                # resolved market/private/pushgw/tradfi WS URLs
bifu-cli ws config set --market-url wss://quote.bifu.dev/api/v1/public/ws
```

## Notes
- Streaming runs until interrupted (Ctrl-C). `-o json` emits raw frames for parsing.
- WS URLs come from the active profile (`config init --env`).
