#!/usr/bin/env bash
set -euo pipefail

repo="${BREAD_RELEASE_REPOSITORY:-HuakunShen/bread}"
install_dir="${BREAD_INSTALL_DIR:-${HOME}/.local/bin}"

case "$(uname -s)" in
  Darwin) os="darwin" ;;
  Linux) os="linux" ;;
  *)
    printf 'bread installer: unsupported operating system: %s\n' "$(uname -s)" >&2
    exit 1
    ;;
esac

case "$(uname -m)" in
  x86_64|amd64) arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *)
    printf 'bread installer: unsupported architecture: %s\n' "$(uname -m)" >&2
    exit 1
    ;;
esac

if [ -n "${BREAD_VERSION:-}" ]; then
  tag="$BREAD_VERSION"
else
  tag="$(curl -fsSL \
    -H 'Accept: application/vnd.github+json' \
    "https://api.github.com/repos/${repo}/releases/latest" |
    sed -n 's/.*\"tag_name\"[[:space:]]*:[[:space:]]*\"\([^\"]*\)\".*/\1/p' |
    head -n 1)"
fi

if [ -z "$tag" ]; then
  printf 'bread installer: could not determine the latest release\n' >&2
  exit 1
fi

version="${tag#v}"
asset="bread_${version}_${os}_${arch}.tar.gz"
base_url="https://github.com/${repo}/releases/download/${tag}"
temporary_directory="$(mktemp -d "${TMPDIR:-/tmp}/bread-install.XXXXXX")"
trap 'rm -rf "$temporary_directory"' EXIT

curl -fsSL -o "$temporary_directory/$asset" "$base_url/$asset"
curl -fsSL -o "$temporary_directory/checksums.txt" "$base_url/checksums.txt"

expected="$(awk -v name="$asset" '
  {
    file = $2
    sub(/^\*/, "", file)
    if (file == name) {
      print $1
      exit
    }
  }
' "$temporary_directory/checksums.txt")"
if [ -z "$expected" ]; then
  printf 'bread installer: checksum entry not found for %s\n' "$asset" >&2
  exit 1
fi

if command -v sha256sum >/dev/null 2>&1; then
  actual="$(sha256sum "$temporary_directory/$asset" | awk '{print $1}')"
else
  actual="$(shasum -a 256 "$temporary_directory/$asset" | awk '{print $1}')"
fi
if [ "$actual" != "$expected" ]; then
  printf 'bread installer: checksum verification failed\n' >&2
  exit 1
fi

mkdir -p "$install_dir"
tar -xzf "$temporary_directory/$asset" -C "$temporary_directory"
install -m 0755 "$temporary_directory/bread" "$install_dir/bread"

printf 'Installed bread %s to %s/bread\n' "$version" "$install_dir"
case ":${PATH}:" in
  *:"$install_dir":*) ;;
  *) printf 'Add %s to PATH to run bread.\n' "$install_dir" ;;
esac
printf 'Install the agent skill with: %s/bread skill --add\n' "$install_dir"
