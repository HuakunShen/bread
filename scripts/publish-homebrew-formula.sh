#!/usr/bin/env bash
set -euo pipefail

: "${GH_TOKEN:?HOMEBREW_TAP_TOKEN must be available as GH_TOKEN}"
: "${TAP_REPOSITORY:?TAP_REPOSITORY is required}"
: "${RELEASE_REPOSITORY:?RELEASE_REPOSITORY is required}"
: "${RELEASE_TAG:?RELEASE_TAG is required}"

download_dir="${RUNNER_TEMP:-${TMPDIR:-/tmp}}/bread-homebrew-formula"
mkdir -p "$download_dir"
trap 'rm -rf "$download_dir"' EXIT

formula="$download_dir/bread.rb"
release_url="https://github.com/${RELEASE_REPOSITORY}/releases/download/${RELEASE_TAG}/bread.rb"
curl -fsSL --retry 3 --retry-delay 1 --output "$formula" "$release_url"
test -f "$formula"
content="$(base64 < "$formula" | tr -d '\n')"
path="repos/$TAP_REPOSITORY/contents/Formula/bread.rb"
message="bread ${RELEASE_TAG#v}"
sha="$(gh api "$path" --jq .sha 2>/dev/null || true)"

args=(
  --method PUT
  "$path"
  -f "message=$message"
  -f "content=$content"
  -f "branch=main"
)
if [ -n "$sha" ]; then
  args+=(-f "sha=$sha")
fi

gh api "${args[@]}" >/dev/null
echo "Published bread.rb for $RELEASE_TAG to $TAP_REPOSITORY."
