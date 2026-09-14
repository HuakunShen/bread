#!/usr/bin/env bash
set -euo pipefail

version="${VERSION:?VERSION is required, without the leading v}"
repository="${GITHUB_REPOSITORY:-HuakunShen/bread}"
dist_dir="${DIST_DIR:-dist}"
checksum_file="${dist_dir}/checksums.txt"
output="${dist_dir}/bread.rb"

test -f "$checksum_file"

checksum_for() {
  local asset="$1"
  awk -v name="$asset" '
    {
      file = $2
      sub(/^\*/, "", file)
      if (file == name) {
        print $1
        exit
      }
    }
  ' "$checksum_file"
}

darwin_amd64="$(checksum_for "bread_${version}_darwin_amd64.tar.gz")"
darwin_arm64="$(checksum_for "bread_${version}_darwin_arm64.tar.gz")"
linux_amd64="$(checksum_for "bread_${version}_linux_amd64.tar.gz")"
linux_arm64="$(checksum_for "bread_${version}_linux_arm64.tar.gz")"

for checksum in "$darwin_amd64" "$darwin_arm64" "$linux_amd64" "$linux_arm64"; do
  test "${#checksum}" -eq 64
done

cat > "$output" <<EOF
# typed: false
# frozen_string_literal: true

class Bread < Formula
  desc "Bounded batch file reader for coding agents"
  homepage "https://github.com/$repository"
  version "$version"
  license "MIT"

  if OS.mac?
    if Hardware::CPU.arm?
      url "https://github.com/$repository/releases/download/v$version/bread_${version}_darwin_arm64.tar.gz"
      sha256 "$darwin_arm64"
    elsif Hardware::CPU.intel?
      url "https://github.com/$repository/releases/download/v$version/bread_${version}_darwin_amd64.tar.gz"
      sha256 "$darwin_amd64"
    else
      raise "Unsupported macOS architecture"
    end
  elsif OS.linux?
    if Hardware::CPU.arm? && Hardware::CPU.is_64_bit?
      url "https://github.com/$repository/releases/download/v$version/bread_${version}_linux_arm64.tar.gz"
      sha256 "$linux_arm64"
    elsif Hardware::CPU.intel? && Hardware::CPU.is_64_bit?
      url "https://github.com/$repository/releases/download/v$version/bread_${version}_linux_amd64.tar.gz"
      sha256 "$linux_amd64"
    else
      raise "Unsupported Linux architecture"
    end
  else
    raise "Unsupported operating system"
  end

  def install
    bin.install "bread"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/bread --version")
  end
end
EOF

printf 'Generated %s\n' "$output"
