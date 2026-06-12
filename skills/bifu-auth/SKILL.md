---
name: bifu-auth
description: Authenticate and configure bifu-cli — profiles, environments, login (scan-to-login or email/password).
auth: none
---

# bifu-cli: auth & config

Activate when the user needs to set up bifu-cli, switch environments, or sign in
before any authenticated trading/account command.

All authenticated endpoints (spot/contract/forex/payment + private WS) use the
session cookie obtained by `auth login`. Get that cookie first, then other skills
work.

## Configure a profile

```bash
bifu-cli config init --env dev        # presets for dev | staging | prod | custom
bifu-cli config use dev               # switch active profile
bifu-cli config get                   # show active profile (cookie masked)
bifu-cli config list                  # list profiles
bifu-cli config set --base-url https://fxapi.bifu.dev   # override a field
```

Profiles live in `~/.bifu-cli/config.yaml`. Use `-p/--profile <name>` to target a
specific profile per command.

## Log in

```bash
# Email/password + email verification code (dev code is 123456)
bifu-cli --profile dev auth login
bifu-cli --profile dev auth login --username user@example.com --password 'pw'  # CI

# Scan-to-login (like `gh auth login`): prints a QR; scan with the logged-in Bifu app
bifu-cli --profile dev auth login --device
```

On success the session cookie is saved to the profile (valid ~30 days). Verify
with any authenticated command, e.g. `bifu-cli spot balance`.

## Notes
- Global flags: `-p/--profile`, `-o/--output table|json|plain`, `--json`, `-v/--verbose`, `-y/--yes`.
- A 401 from any command means the session expired → run `auth login` again.
