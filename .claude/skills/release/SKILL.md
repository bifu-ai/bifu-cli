---
name: release
description: How to develop, branch, and ship bifu-cli — feature branch → develop (auto rc prerelease) → main → tag (official release + npm + Homebrew + open-source mirror). Use when cutting a release, publishing a version, reviewing an rc, or promoting an rc to stable ("发版", "发布", "publish", "cut a release").
auth: none
---

# bifu-cli: branch & release flow

Maintainer-facing. This is how code reaches users. Source repo `decodeex/bifu-cli`
is **private**; downloadable binaries live in the **public** `decodeex/bifu-cli-releases`.
Default branch is `develop`.

## Branch model (every change goes through develop first)

```
feature branch ──PR merge──▶ develop ──(auto)──▶ rc prerelease  vX.Y.Z-rc.<run>
                                  │                  (review the rc build)
                                  └──PR merge──▶ main ──tag vX.Y.Z──▶ OFFICIAL release
```

1. **Branch off develop** for any change: `git switch -c feat/<thing> develop`.
2. **Merge into develop via PR** (branch merge — never commit straight to a release branch).
3. Pushing `develop` **auto-publishes an rc prerelease** (see below) — nothing to do by hand.
4. Review the rc binaries. When satisfied, **merge `develop` → `main` via PR**.
5. **Tag `vX.Y.Z` on `main`** to cut the official release.

Never tag on `develop`. Never commit to the `bifu-ai/bifu-cli` mirror — it's overwritten.

## The two pipelines

| Trigger | Workflow | Publishes |
|---|---|---|
| push to `develop` | `.github/workflows/prerelease.yml` | GitHub **Pre-release** `vX.Y.Z-rc.<run>` (6-platform binaries + checksums) to `bifu-cli-releases`. **No** npm / Homebrew / docs. |
| tag `vX.Y.Z` on `main` | `.github/workflows/release.yml` | GitHub **Release** + npm `@decodeex/bifu-cli` + Homebrew cask `decodeex/tap/bifu-cli` + README/skills sync to releases repo. |
| push to `main` | `.github/workflows/sync-upstream.yml` | Force-mirrors `main` → public `bifu-ai/bifu-cli`. |

- **rc version** is computed automatically: latest stable tag's next patch + `-rc.<run_number>`
  (e.g. latest `v1.1.8` → `v1.1.9-rc.1`). No manual tagging on develop.
- rc is marked GitHub *pre-release* (`release.prerelease: auto`), so `releases/latest`
  (what `install.sh` / curl resolve) keeps pointing at the **stable** version — an rc never
  reaches end users via curl/brew/npm.
- The Homebrew cask is skipped for rc via `homebrew_casks.skip_upload: "auto"`; npm + docs-sync
  jobs are gated on stable tags with `if: ${{ !contains(github.ref_name, '-') }}`.
- Version string is injected by ldflags into `bifu-cli/cmd.version`.

## Cut an official release

After `develop` → `main` is merged and you're on an up-to-date `main`:

```bash
git switch main && git pull
git tag v1.1.9            # bump from the latest stable tag; no -rc suffix
git push origin v1.1.9    # → release.yml: GitHub Release + npm + Homebrew + docs + mirror
```

Pick the version to match the rc you validated (rc `v1.1.9-rc.N` → ship `v1.1.9`).

## Verify

```bash
# rc after a develop push
gh run list --workflow=prerelease.yml --limit 1
gh release view "$(gh release list -R decodeex/bifu-cli-releases --json tagName,isPrerelease \
  --jq '[.[]|select(.isPrerelease)][0].tagName')" -R decodeex/bifu-cli-releases \
  --json tagName,isPrerelease,assets

# official release reached all channels
gh release list -R decodeex/bifu-cli-releases --limit 3          # vX.Y.Z = Latest
npm view @decodeex/bifu-cli version                              # = X.Y.Z
gh api repos/decodeex/homebrew-tap/contents/Casks/bifu-cli.rb --jq .content | base64 -d | grep version
curl -fsSL https://cli.bifu.dev/install.sh | bash               # installs X.Y.Z
```

Always run `goreleaser check` and `actionlint` after editing `.goreleaser.yaml` or any workflow:

```bash
GOTOOLCHAIN=auto go run github.com/goreleaser/goreleaser/v2@latest check
GOTOOLCHAIN=auto go run github.com/rhysd/actionlint/cmd/actionlint@latest
```

## Secrets / tokens (already configured)

- `HOMEBREW_TAP_GITHUB_TOKEN` — cross-repo PAT (owner decodeex) that GoReleaser uses as
  `GITHUB_TOKEN` to write the GitHub Release to `bifu-cli-releases` and push the cask to
  `homebrew-tap`. The repo-default `GITHUB_TOKEN` only reaches this private repo.
- `NPM_TOKEN` — npm publish (`bifu-admin`, @decodeex org).
- `UPSTREAM_SYNC_SSH_KEY` — ed25519 deploy key scoped to **only** `bifu-ai/bifu-cli`, for the mirror.

Do **not** store a personal broad-scope `gh` OAuth token as a CI secret; use scoped PATs / deploy keys.
