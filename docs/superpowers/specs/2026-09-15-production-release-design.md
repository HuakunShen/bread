# bread Production Release Design

**Date:** 2026-09-15

## Objective

Turn the local `bread` draft into a publishable Go CLI with repeatable tests,
cross-platform release artifacts, GitHub Pages documentation, curl/PowerShell
installers, self-upgrade, and a Homebrew tap path.

## Release topology

- `main` is the development branch.
- Pull requests and pushes run `.github/workflows/ci.yml` on Ubuntu, macOS, and
  Windows.
- Tags matching `v*.*.*` run `.github/workflows/release.yml`.
- GoReleaser builds `darwin`, `linux`, and `windows` for `amd64` and `arm64`,
  publishes archives and `checksums.txt` to GitHub Releases, and embeds the
  release version in the binary.
- `.github/workflows/pages.yml` deploys the checked-in `site/` directory with
  the GitHub Pages Actions artifact/deployment flow.
- `.github/workflows/homebrew.yml` publishes a generated formula to
  `HuakunShen/homebrew-tap` only when the repository owner enables it with a
  repository variable and a narrowly scoped token secret.

The local repository contains all workflow and documentation configuration. A
public GitHub repository and the tap are external resources and require an
explicit owner-side authorization before creation or publication.

## Upgrade contract

The user-facing command is `bread upgrade`; an installed Go module cannot add a
subcommand to the `go` executable, so `go upgrade` is not a realizable command
for this project. `bread upgrade` queries the latest public GitHub release,
selects the current OS/architecture archive, verifies its SHA-256 entry from
`checksums.txt`, extracts only the `bread` executable, preserves the installed
file mode, and replaces the current executable. Unix replacement is atomic;
Windows schedules a retrying `.cmd` helper so the locked parent executable can
exit first.

`bread upgrade --check` performs the release check without downloading or
changing the installed binary. Development builds report the latest release as
available so a binary built by `go install` can still self-upgrade.

## Installation channels

1. Developers with Go use
   `go install github.com/HuakunShen/bread/cmd/bread@latest`.
2. Unix users without Go use `install.sh` through `curl`; it supports macOS and
   Linux on amd64 and arm64.
3. Windows users without Go use `install.ps1`; it supports amd64 and arm64.
4. Homebrew users add the tap once and then use `brew install bread`. A bare
   `brew install bread` without tapping first requires a separate accepted
   formula in `homebrew/core`; this repository cannot self-merge into that
   official repository.

## Documentation and Skill

`README.md` is the English landing page, `README.zh-CN.md` is the Simplified
Chinese landing page, and `docs/zh-CN.md` is the Chinese operational guide.
The navigation skill remains at
`skills/efficient-codebase-navigation/SKILL.md`; both READMEs document the
`npx skills@latest add HuakunShen/bread` installation command.

## Security and operational constraints

- Release workflows use least-privilege `contents: write` only where release
  publication requires it; Pages uses `pages: write` and `id-token: write`.
- The tap workflow never uses the repository-scoped `GITHUB_TOKEN` to write a
  second repository; it requires `HOMEBREW_TAP_TOKEN` and an explicit enable
  variable.
- Installers verify the downloaded archive against the release checksum before
  installing it.
- The updater does not execute archive contents, follow archive symlinks, or
  accept a checksum from the downloaded archive itself.
- No secrets are committed. No installer uses `sudo` implicitly.

## Acceptance evidence

The local gate is `gofmt -l .`, `go test ./...`, `go test -race ./...`,
`go vet ./...`, GoReleaser snapshot validation, shell syntax validation, and
PowerShell parsing. After external setup, the remote gate is a successful CI
run, a tag-created Release containing all target archives and checksums, a
published Pages URL, and a tap formula that passes `brew audit` and installs
`bread` on supported macOS/Linux hosts.
