---
name: bifu-auth
description: Sign in to bifu-cli — scan-to-login (QR) or email/password — and manage the session.
auth: none
---

# bifu-cli: login & session

Activate when the user needs to sign in before any authenticated command, or when
a command returns 401 (session expired). For profiles/environments see the
bifu-config skill.

All authenticated endpoints (spot/contract/forex/payment + private WS) use the
session cookie obtained here, saved into the active profile (valid ~30 days).

## Log in

```bash
# Email/password + email verification code (dev code is 123456)
bifu-cli --profile dev auth login
bifu-cli --profile dev auth login --username user@example.com --password 'pw'   # non-interactive (CI)

# Scan-to-login (like `gh auth login`): prints a QR; scan with the logged-in Bifu app
bifu-cli --profile dev auth login --device
```

Verify with any authenticated command, e.g. `bifu-cli spot balance`.

## Notes
- A 401 from any command = session expired → run `auth login` again.
- Pick the target profile with `-p/--profile <name>` (see bifu-config).
- `auth cookie encode/decode/set` exist but are offline-debug only; backend now
  validates the session server-side, so locally-generated cookies won't authenticate.
