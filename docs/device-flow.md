# Device Login Flow (`bifu-cli auth login --device`)

OAuth 2.0 Device Authorization Grant ([RFC 8628](https://datatracker.ietf.org/doc/html/rfc8628)),
the same model as `gh auth login`. The CLI shows a one-time code, opens the
browser to a verification page, and polls until the user approves. No password
is typed in the terminal.

The CLI side is **already implemented** ([cmd/auth/login.go](../cmd/auth/login.go),
`runDeviceLogin`). It is ready the moment the backend ships the two endpoints
below. Headers sent on both requests: `Content-Type: application/json`,
`terminalType`, `locale`, `appVersion` (same as `/user/login`).

---

## Sequence

```
CLI                         Backend                       Browser (user)
 │  POST /user/device_code    │                                 │
 │ ─────────────────────────► │                                 │
 │  ◄───────────────────────  │  deviceCode + userCode          │
 │                            │                                 │
 │  show userCode, open verificationUriComplete ───────────────►│
 │                            │       user logs in & approves   │
 │                            │ ◄────────────────────────────── │
 │  POST /user/device_token   │  (poll every `interval`s)       │
 │ ─────────────────────────► │                                 │
 │  ◄── status:"pending" ───  │                                 │
 │           ... repeat ...   │                                 │
 │  ◄── status:"success" ───  │  cookieStr + user               │
 │  save cookie to profile    │                                 │
```

---

## Endpoint 1 — issue codes

`POST /user/device_code`

**Request**

```json
{ "terminalType": "API" }
```

**Response** (envelope identical to `/user/login`: `retCode` `"0"` = OK)

```json
{
  "retCode": "0",
  "retMsg": "",
  "result": {
    "deviceCode": "long-opaque-secret-bound-to-this-attempt",
    "userCode": "ABCD-1234",
    "verificationUri": "https://bifu.dev/device",
    "verificationUriComplete": "https://bifu.dev/device?code=ABCD-1234",
    "expiresIn": 600,
    "interval": 5
  }
}
```

| Field | Meaning |
|-------|---------|
| `deviceCode` | Opaque secret the CLI polls with. Not shown to the user. |
| `userCode` | Short human code the user confirms in the browser (e.g. `ABCD-1234`). |
| `verificationUri` | Page where the user enters/approves the code. |
| `verificationUriComplete` | Same page with `userCode` prefilled (the CLI opens this). |
| `expiresIn` | Seconds until `deviceCode` expires (CLI stops polling after this). |
| `interval` | Minimum seconds between polls. |

---

## Endpoint 2 — poll for approval

`POST /user/device_token`

**Request**

```json
{ "deviceCode": "long-opaque-secret-bound-to-this-attempt" }
```

**Response** — always `retCode: "0"`; the state lives in `result.status`:

| `result.status` | Meaning | CLI behaviour |
|-----------------|---------|---------------|
| `pending` | User has not approved yet | keep polling |
| `slow_down` | Polling too fast | increases interval by 5s, keeps polling |
| `success` | Approved | save cookie, stop |
| `denied` | User rejected | error out |
| `expired` | `deviceCode` expired | error out |

Pending example:

```json
{ "retCode": "0", "result": { "status": "pending" } }
```

Success example (`cookieStr` is the **same JSON-serialised `http.Cookie`** that
`/user/login_check` already returns — the CLI extracts `.Value`):

```json
{
  "retCode": "0",
  "result": {
    "status": "success",
    "cookieStr": "{\"Name\":\"user_auth_name\",\"Value\":\"<session-cookie>\"}",
    "user": { "userId": "109150807" }
  }
}
```

A nonzero `retCode` is treated as a hard error (message shown to the user), so
keep transient device states inside `result.status`, not in `retCode`.

---

## Browser side (`verificationUri` page)

A logged-in user lands on `verificationUri`, sees the `userCode` (prefilled from
`verificationUriComplete`), and clicks **Approve**. That approval is what flips
the next `device_token` poll to `success`. If the user is not logged in, the
page should send them through normal login first, then back to approval.

## Notes

- `deviceCode` must be single-use and bound to the issuing client; expire it
  after `expiresIn`.
- The dev environment may keep the fixed verification-code convention; the
  device page can auto-approve in dev to make CLI testing trivial.
- No CLI change is needed when this ships — `bifu-cli auth login --device`
  already speaks this contract.
