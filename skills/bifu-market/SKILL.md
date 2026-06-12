---
name: bifu-market
description: Real-time streaming over WebSocket — public market data, private trading events, and forex push quotes.
auth: partial
---

# bifu-cli: WebSocket streaming

Activate to stream live data. Public market data needs no auth; private trading
events and some forex streams require a logged-in profile (see bifu-auth).

## Public market data (no auth)

```bash
bifu-cli ws market --channels ticker.BTCUSDT
bifu-cli ws market --channels ticker.BTCUSDT,depth.BTCUSDT
```

## Private trading events (auth)

```bash
bifu-cli ws private          # contract private stream
bifu-cli ws private --spot   # spot private stream
```

## Forex push quotes

```bash
bifu-cli ws pushgw           # MT5 push gateway quotes
```

## Endpoints

```bash
bifu-cli ws config show      # show resolved market/private/pushgw/tradfi WS URLs
bifu-cli ws config set --market-url wss://quote.bifu.dev/api/v1/public/ws
```

## Notes
- Streaming runs until interrupted (Ctrl-C). Use `-o json` for raw frames.
- WS URLs come from the active profile (set by `config init --env`).
