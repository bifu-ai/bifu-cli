# Device Login (`bifu-cli auth login --device`)

A `gh auth login`-style flow: the CLI opens a browser approval page, the user
(already logged in on the web) approves, and the CLI polls until it receives the
session cookie. No password is typed in the terminal.

This **reuses the backend's existing scan-to-login (QR) endpoints** — there are
no dedicated device endpoints. The CLI side is implemented in
[cmd/auth/login.go](../cmd/auth/login.go) (`runDeviceLogin`).

---

## Sequence

```
CLI                              Backend                    Browser (logged-in user)
 │ GET  /user/login/qr_code_get    │                              │
 │ ──────────────────────────────► │  issueId + url               │
 │ ◄──────────────────────────────                                │
 │ open {web_url}/x/{issueId} ────────────────────────────────────►│
 │                                 │      page: qr_code_scan       │
 │                                 │ ◄──────────────────────────── │
 │                                 │      page: qr_code_confirm    │
 │                                 │ ◄──────────────────────────── │ (is_confirm=1)
 │ POST /user/login/qr_code_check  │                              │
 │ ──────────────────────────────► │  (poll every 3s)             │
 │ ◄── issueStatus:"processing" ─                                  │
 │           ... repeat ...        │                              │
 │ ◄── issueStatus:"success" ───   │  cookieStr + user            │
 │ save cookie to profile          │                              │
```

The CLI opens `{profile.web_url}/x/{issueId}` (e.g. `https://bifu.dev/x/...`),
not the `url` returned by the backend — `qr_code_get` returns a hard-coded prod
URL, so the CLI rewrites the host using the profile's `web_url` to hit the right
environment.

---

## Endpoints used (already exist on `develop`)

### `GET /user/login/qr_code_get`

Response (envelope: `retCode == "0"` is OK):

```json
{ "retCode": "0", "result": { "url": "https://bifu.co/x/<issueId>", "issueId": "<issueId>" } }
```

### `POST /user/login/qr_code_check`

```json
{ "issueId": "<issueId>" }
```

Response — state is in `result.issueStatus`:

| `issueStatus` | CLI behaviour |
|---------------|---------------|
| `pending` / `processing` | keep polling |
| `success` | save `result.cookieStr` (JSON `http.Cookie`, the CLI extracts `.Value`) + `result.user.userId`, stop |
| `refused` | error: rejected in browser |
| `expired` | error: code expired |

Success example:

```json
{
  "retCode": "0",
  "result": {
    "issueStatus": "success",
    "cookieStr": "{\"Name\":\"user_auth_name\",\"Value\":\"<session-cookie>\"}",
    "user": { "userId": "109150807" }
  }
}
```

---

## Browser approval page — `/x/{issueId}` (frontend)

A logged-in user lands on `/x/{issueId}` and approves. The page drives the
backend's two-step confirm:

1. `POST /user/login/qr_code_scan` `{ "issueId": "<issueId>" }` → moves the issue
   to `processing`.
2. `POST /user/login/qr_code_confirm` `{ "issueId": "<issueId>", "isConfirm": "1" }`
   (requires the user's session) → moves it to `success:{userId}`.

`is_confirm = "0"` rejects (→ `refused`). The issue TTL is short (60s, reset on
each step), so the page should call scan immediately on load.
