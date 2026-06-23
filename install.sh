#!/usr/bin/env bash
#
# bitrise-cli installer.
#
# Downloads the latest bitrise-cli release for your OS/architecture from GitHub
# Releases, verifies its checksum, and installs the binary. No Go toolchain
# required.
#
# Usage:
#   curl -fsSL https://app.bitrise.io/cli/install.sh | bash
#
# Environment overrides:
#   BITRISE_CLI_VERSION       Install a specific tag (e.g. v0.2.0). Default: latest release.
#   BITRISE_CLI_INSTALL_DIR   Install directory. Default: ~/.local/bin.
#
set -euo pipefail

REPO="bitrise-io/bitrise-cli"
BINARY="bitrise-cli"
INSTALL_DIR="${BITRISE_CLI_INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${BITRISE_CLI_VERSION:-}"

# This script produces no stdout — everything below is diagnostics/progress, so
# it all goes to stderr.
info() { printf '%s\n' "$*" >&2; }
die()  { printf 'error: %s\n' "$*" >&2; exit 1; }

require() {
  command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

require curl
require tar

# --- Detect OS -------------------------------------------------------------
os="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$os" in
  linux | darwin) ;;
  *) die "unsupported OS: $os — Windows users: download the .zip from https://github.com/$REPO/releases/latest" ;;
esac

# --- Detect architecture ---------------------------------------------------
arch="$(uname -m)"
case "$arch" in
  x86_64 | amd64) arch="amd64" ;;
  aarch64 | arm64) arch="arm64" ;;
  *) die "unsupported architecture: $arch — see https://github.com/$REPO/releases/latest" ;;
esac

# --- Resolve version -------------------------------------------------------
# When no version is pinned, follow the redirect from /releases/latest to
# /releases/tag/<tag> and read the tag from the final URL. This avoids the
# GitHub API's unauthenticated rate limit (60/hr per IP).
if [ -z "$VERSION" ]; then
  info "Resolving latest version…"
  VERSION="$(curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/$REPO/releases/latest" |
    sed 's#.*/tag/##' | tr -d '[:space:]')" ||
    die "could not reach GitHub to resolve the latest version; set BITRISE_CLI_VERSION to pin one"
  [ -n "$VERSION" ] || die "could not determine the latest version; set BITRISE_CLI_VERSION to pin one"
fi

# Asset filenames drop the leading 'v' (e.g. bitrise-cli_0.2.0_darwin_arm64.tar.gz).
asset="${BINARY}_${VERSION#v}_${os}_${arch}.tar.gz"
base_url="https://github.com/$REPO/releases/download/$VERSION"

info "Installing $BINARY $VERSION ($os/$arch)…"

# --- Download into a temp dir ----------------------------------------------
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

curl -fsSL "$base_url/$asset" -o "$tmp/$asset" ||
  die "download failed: $base_url/$asset (no prebuilt binary for $os/$arch in $VERSION?)"
curl -fsSL "$base_url/checksums.txt" -o "$tmp/checksums.txt" ||
  die "could not download checksums.txt for $VERSION"

# --- Verify checksum -------------------------------------------------------
expected="$(awk -v f="$asset" '$2 == f {print $1}' "$tmp/checksums.txt")"
[ -n "$expected" ] || die "no checksum entry for $asset in checksums.txt"

if command -v sha256sum >/dev/null 2>&1; then
  actual="$(sha256sum "$tmp/$asset" | awk '{print $1}')"
elif command -v shasum >/dev/null 2>&1; then
  actual="$(shasum -a 256 "$tmp/$asset" | awk '{print $1}')"
elif command -v openssl >/dev/null 2>&1; then
  actual="$(openssl dgst -sha256 "$tmp/$asset" | awk '{print $NF}')"
else
  die "no SHA-256 tool found (need sha256sum, shasum, or openssl)"
fi

[ "$expected" = "$actual" ] || die "checksum mismatch for $asset (expected $expected, got $actual)"
info "Checksum verified."

# --- Extract + install -----------------------------------------------------
tar -xzf "$tmp/$asset" -C "$tmp" "$BINARY" || die "failed to extract $BINARY from $asset"
mkdir -p "$INSTALL_DIR"
install -m 0755 "$tmp/$BINARY" "$INSTALL_DIR/$BINARY" ||
  die "failed to install to $INSTALL_DIR — set BITRISE_CLI_INSTALL_DIR to a writable dir, or re-run with sudo"

info "✓ Installed $BINARY $VERSION to $INSTALL_DIR/$BINARY"

# --- PATH check ------------------------------------------------------------
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *)
    info ""
    info "$INSTALL_DIR is not on your PATH. Add it, e.g.:"
    info "    export PATH=\"$INSTALL_DIR:\$PATH\""
    ;;
esac

info ""
info "Join our community:"
info "  https://join.slack.com/t/bitrise/shared_invite/zt-3ycxzv91u-hsP5Uw9wcrOMjNDiJChJfw"
info ""
info "Get started:  $BINARY auth login"
