# Production Release Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `bread` releasable and installable on macOS, Linux, and Windows
for amd64 and arm64, with CI, Pages, Releases, self-upgrade, Homebrew tap
automation, and English/Simplified Chinese documentation.

**Architecture:** Keep update protocol and archive verification in a
standard-library `internal/update` package. Keep release mechanics in
GoReleaser and GitHub Actions, with Pages as a static checked-in site. Keep
Homebrew publication optional and cross-repository-token protected so a missing
tap secret cannot publish elsewhere.

**Tech Stack:** Go standard library, GoReleaser v2, GitHub Actions, GitHub
Pages Actions, POSIX shell, PowerShell, and Homebrew Ruby formula generation.

**Spec:** `docs/superpowers/specs/2026-09-15-production-release-design.md`

## Global Constraints

- Build targets are `darwin`, `linux`, and `windows` on `amd64` and `arm64`.
- `bread upgrade` is the supported self-upgrade command; `go upgrade` cannot be added to the Go tool.
- Downloads are accepted only after SHA-256 verification against release `checksums.txt`.
- No third-party runtime dependencies are added to the Go module.
- Cross-repository Homebrew writes require `HOMEBREW_TAP_TOKEN` and `HOMEBREW_TAP_ENABLED=true`.
- External GitHub repository creation/publication is separate from local file changes and requires owner authorization.

### Task 1: Version identity and updater core

**Files:**
- Modify: `go.mod`
- Modify: `cmd/bread/main.go`
- Create: `internal/version/version.go`
- Create: `internal/update/update.go`
- Create: `internal/update/update_test.go`

- [ ] **Step 1: Move the module to the verified GitHub import path and add version identity.**

  Change `go.mod` to `module github.com/HuakunShen/bread`, update internal imports,
  and replace the CLI's hard-coded version with `internal/version.Value`.

- [ ] **Step 2: Write updater tests against an `httptest.Server`.**

  Cover release comparison, target asset selection, missing assets, checksum
  matching, invalid checksums, tar.gz/zip extraction, path traversal rejection,
  and development-version availability.

- [ ] **Step 3: Implement verified release lookup and replacement.**

  Query the public latest-release endpoint, download into a temporary directory,
  verify SHA-256, extract only the expected executable, preserve its mode, and
  replace the installed executable safely on Unix and Windows.

- [ ] **Step 4: Add `bread upgrade` and `bread upgrade --check`.**

  Dispatch the subcommand before normal file-spec parsing, print status to
  stdout, write failures to stderr, and return a non-zero status on failure.

- [ ] **Step 5: Run updater tests and race/vet checks.**

  Run `go test ./internal/update -v`, `go test -race ./...`, and `go vet ./...`.

### Task 2: Reproducible cross-platform release configuration

**Files:**
- Create: `.goreleaser.yaml`
- Create: `LICENSE`
- Create: `.github/workflows/ci.yml`
- Create: `.github/workflows/release.yml`

- [ ] **Step 1: Configure GoReleaser.**

  Build with `CGO_ENABLED=0`, `-trimpath`, version ldflags, Darwin/Linux/Windows
  amd64/arm64 archives, Windows zip overrides, checksums, changelog filters,
  and a snapshot-safe generated Homebrew formula.

- [ ] **Step 2: Add least-privilege CI.**

  Run format, vet, race tests, ordinary tests, and builds in a matrix of Ubuntu,
  macOS, and Windows using current checkout/setup-Go actions.

- [ ] **Step 3: Add tag-driven Release automation.**

  Use full Git history, GoReleaser's official action, `contents: write`, and
  `v*.*.*` tag filtering. Upload the generated formula as a Release asset when
  present so the optional tap job can consume the exact file.

### Task 3: GitHub Pages and installation channels

**Files:**
- Create: `.github/workflows/pages.yml`
- Create: `site/index.html`
- Create: `site/zh-CN.html`
- Create: `install.sh`
- Create: `install.ps1`

- [ ] **Step 1: Add static Pages content and workflow.**

  Deploy `site/` through configure-pages, upload-pages-artifact, and deploy-pages
  with the `github-pages` environment and required permissions.

- [ ] **Step 2: Implement the Unix installer.**

  Resolve the latest tag, map Darwin/Linux and amd64/arm64, download the matching
  archive and checksums, verify before install, and install into
  `${BREAD_INSTALL_DIR:-$HOME/.local/bin}` without sudo.

- [ ] **Step 3: Implement the Windows installer.**

  Map AMD64/ARM64, download zip/checksums, verify with `Get-FileHash`, and install
  to `${env:BREAD_INSTALL_DIR:-$env:LOCALAPPDATA\\bread\\bin}`.

### Task 4: Homebrew tap automation and documentation

**Files:**
- Create: `.github/workflows/homebrew.yml`
- Create: `scripts/publish-homebrew-formula.sh`
- Modify: `.goreleaser.yaml`
- Modify: `README.md`
- Create: `README.zh-CN.md`
- Create: `docs/zh-CN.md`

- [ ] **Step 1: Add the optional tap publisher.**

  Trigger on published releases, skip unless `vars.HOMEBREW_TAP_ENABLED` is
  `true`, download the release-generated formula, and update
  `HuakunShen/homebrew-tap` through the Contents API using only
  `HOMEBREW_TAP_TOKEN`.

- [ ] **Step 2: Explain Homebrew's two paths.**

  Document `brew tap HuakunShen/tap && brew install bread` as the immediately
  available path and `brew install bread` without a tap as the later
  homebrew/core pull-request outcome. Include audit and secret setup steps.

- [ ] **Step 3: Write complete Simplified Chinese documentation.**

  Translate installation, supported platforms, CLI usage, upgrade behavior,
  Skill installation, release model, and troubleshooting into Chinese rather
  than merely linking to the English README.

### Task 5: Local verification and external handoff

- [ ] **Step 1: Run the full local gate.**

  Run `gofmt -w`, `go test ./...`, `go test -race ./...`, `go vet ./...`,
  cross-build checks for all six targets, installer syntax checks, and
  GoReleaser snapshot validation.

- [ ] **Step 2: Inspect the full diff and clean state.**

  Verify no secrets, placeholders, generated build directories, or stale local
  binaries are tracked; confirm the repo is independent of Xross/CrossCopy.

- [ ] **Step 3: State external setup blockers accurately.**

  If the owner has not authorized creation of public GitHub resources, report
  the exact one-time commands and the local artifacts already prepared. Do not
  claim a remote Release, Pages URL, or Homebrew install until observed.
