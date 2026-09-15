#!/usr/bin/env bash
set -euo pipefail

root="$(mktemp -d)"
trap 'rm -rf "$root"' EXIT
mkdir -p "$root/bin" "$root/tmp"

printf "%s\n" "#!/usr/bin/env bash" 'trace="$GH_TRACE"' 'printf "%s\n" "$*" >> "$trace"' 'if [ "$1" = "release" ]; then exit 42; fi' 'if [ "$1" != "api" ]; then exit 43; fi' 'shift' 'if [ "$1" = "--method" ]; then test "$2" = "PUT"; exit 0; fi' 'exit 1' > "$root/bin/gh"

printf "%s\n" "#!/usr/bin/env bash" 'set -euo pipefail' 'out=""' 'while [ "$#" -gt 0 ]; do' '  if [ "$1" = "-o" ] || [ "$1" = "--output" ]; then out="$2"; shift 2; else shift; fi' 'done' 'test -n "$out"' 'printf "%s\n" "class Bread < Formula" "end" > "$out"' > "$root/bin/curl"

chmod +x "$root/bin/gh" "$root/bin/curl"

GH_TOKEN=tap-only-token TAP_REPOSITORY=HuakunShen/homebrew-tap RELEASE_REPOSITORY=HuakunShen/bread RELEASE_TAG=v0.2.3 RUNNER_TEMP="$root/tmp" GH_TRACE="$root/gh.trace" PATH="$root/bin:$PATH" ./scripts/publish-homebrew-formula.sh
grep -q -- "--method PUT" "$root/gh.trace"
if grep -q -- "release download" "$root/gh.trace"; then
  echo "unexpected gh release download call" >&2
  exit 1
fi
