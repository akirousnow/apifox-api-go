# apifox-api-go release guide

Production distribution for the native `apifox-api` Go CLI, plus the npm compatibility bridge for one stable migration cycle.

## Toolchain pin

- **Release builds:** Go **1.26.5** (`GOTOOLCHAIN=go1.26.5` or `go1.26.5` binary).
- **Module minimum** in `go.mod` may remain lower (currently 1.25.x) so local development stays flexible.
- Never embed Auth Keys or secrets into ldflags, SBOM, provenance, or CI logs.

## Six release targets

| GOOS    | GOARCH | Artifact path (under `dist/release/`)   |
|---------|--------|------------------------------------------|
| linux   | amd64  | `linux-amd64/apifox-api`                 |
| linux   | arm64  | `linux-arm64/apifox-api`                 |
| darwin  | amd64  | `darwin-amd64/apifox-api`                |
| darwin  | arm64  | `darwin-arm64/apifox-api`                |
| windows | amd64  | `windows-amd64/apifox-api.exe`           |
| windows | arm64  | `windows-arm64/apifox-api.exe`           |

Build flags (frozen in `scripts/release.sh`):

- `CGO_ENABLED=0`
- `-trimpath`
- `-ldflags "-s -w -X .../buildinfo.Version=... -X .../buildinfo.Commit=..."`

## Build a release locally

```bash
# run from repository root
export GOTOOLCHAIN=go1.26.5   # pin when available
export VERSION=0.1.0
export COMMIT="$(git rev-parse --short HEAD)"
./scripts/release.sh
```

Outputs:

- Six platform binaries under `dist/release/<os>-<arch>/`
- `dist/release/checksums.txt` (SHA-256 of every binary)
- `dist/release/sbom.spdx.json` (SPDX-2.3 style package list)
- `dist/release/provenance.json` (build materials / toolchain pin / commit)

Smoke (no Node/Bun):

```bash
./scripts/smoke.sh dist/release/linux-amd64/apifox-api
# or after release.sh on Linux: automatic --version / version smoke
```

## Version surface

Both of these print the same shape:

```text
apifox-api <semver> (commit <sha>)
```

- `apifox-api --version`
- `apifox-api version`

Link-time injection:

```bash
go build -ldflags "-X github.com/akirousnow/apifox-api-go/internal/buildinfo.Version=0.1.0 -X github.com/akirousnow/apifox-api-go/internal/buildinfo.Commit=$(git rev-parse --short HEAD)" -o apifox-api .
```

Defaults when not injected: `dev` / `unknown`.

## `go install` path

Until the module is published under a public module path matching your hosting (replace with the real import path when published):

```bash
# From a clone (local path / replace module path when published)
# run from repository root
go install -ldflags "-X github.com/akirousnow/apifox-api-go/internal/buildinfo.Version=0.1.0 -X github.com/akirousnow/apifox-api-go/internal/buildinfo.Commit=$(git rev-parse --short HEAD)" .

# After the module is published (example):
# GOTOOLCHAIN=go1.26.5 go install github.com/akirousnow/apifox-api-go@v0.1.0
```

The resulting `apifox-api` binary is a pure native executable: **no Node or Bun runtime**.

## Prerelease → stable cutover

| Stage        | Version example | npm package behavior                                      | Native binary                          |
|--------------|-----------------|-----------------------------------------------------------|----------------------------------------|
| prerelease   | `0.1.0-rc.1`    | Keep shipping TS `dist/index.js` as today (`0.0.x` line) | Publish Go artifacts for early adopters |
| first stable | `0.1.0`         | **Still** ship npm/npx TS entry for **one full stable cycle** | Primary recommended install path        |
| next cycle   | `0.2.0+`        | Optional: switch npm bin to download native binary wrapper | Default                                |

Do **not** remove the npm package in the first stable Go cutover cycle.

## npm / npx compatibility bridge (one stable cycle)

Existing users continue to work with:

```bash
npm install -g apifox-api
npx apifox-api <command>
```

- Package name remains `apifox-api`.
- Current `package.json` `bin` points at the TypeScript/Node build (`dist/index.js`).
- This is intentional for at least **one stable release cycle** after the Go binary is production-ready.
- See also root `NPM_COMPAT.md` for rollback steps.

### TypeScript rollback

If a Go-related change ships and npm users need the previous TS CLI:

1. Install a known-good TS version: `npm install -g apifox-api@0.0.20` (or the last TS-only tag).
2. Or pin in package.json: `"apifox-api": "0.0.20"`.
3. Confirm with `apifox-api version` / `npx apifox-api version` that the Node entry runs.
4. Registry binding (`~/.apifox-api.json`) and cache layout under `.cache/apifox-api/` stay shared with the dual-baseline contract (v0.1 + v0.0.20).

## GitHub Actions auto-release

Workflow file: [`.github/workflows/release.yml`](./.github/workflows/release.yml).

### What it does

On trigger it will:

1. Install Go **1.26.5**
2. Run `./scripts/release.sh` (six platforms + checksums + SBOM + provenance)
3. Run `./scripts/smoke.sh dist/release/linux-amd64/apifox-api`
4. Pack archives under `dist/assets/`:
   - `apifox-api_<version>_<os>_<arch>.tar.gz` (unix)
   - `apifox-api_<version>_windows_<arch>.zip`
   - `checksums.txt` / `checksums-archives.txt` / `sbom.spdx.json` / `provenance.json`
5. Upload workflow artifacts
6. When a real version is resolved: create/update a **GitHub Release** (`v<version>`) and attach assets

Uses only the default `GITHUB_TOKEN` (`permissions: contents: write`).  
Never print `APIFOX_AUTH_KEY` or other secrets in release logs.

### Triggers

| Trigger | Publish GitHub Release? |
|---------|-------------------------|
| Push to `release` branch **with** root `VERSION` file | Yes (`v` + file content) |
| Push tag `v0.1.0` / `v0.1.0-rc.1` | Yes |
| Push to `release` **without** `VERSION` / tag | Build + artifact only (no Release) |
| Actions → Run workflow (`workflow_dispatch`) + version input | Yes |

Versions containing `-` (e.g. `0.1.0-rc.1`) are marked **prerelease**.

### Recommended flow: push `release` branch

```bash
# 1) On main (or your work branch), prepare the cut
git checkout main
git pull

# 2) Create/update the release branch
git checkout -B release
# or: git checkout release && git merge main

# 3) Set the version that should be published (no leading v)
echo '0.1.0' > VERSION
git add VERSION
git commit -m "release: v0.1.0"

# 4) Push — Actions builds and publishes GitHub Release v0.1.0
git push -u origin release
```

After the workflow finishes, open:

https://github.com/akirousnow/apifox-api-go/releases

### Alternative: tag-driven release

```bash
git checkout release   # or main
git tag v0.1.0
git push origin v0.1.0
```

### Manual re-run

GitHub → Actions → **Release** → **Run workflow** → optional `version` input (e.g. `0.1.0`).

## Verify downloads

```bash
cd dist/release
sha256sum -c checksums.txt   # or shasum -a 256 -c checksums.txt
```

Inspect `sbom.spdx.json` and `provenance.json` for version, commit, and toolchain pin.
