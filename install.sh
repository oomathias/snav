#!/usr/bin/env bash

set -euo pipefail

REPO="${SNAV_REPO:-oomathias/snav}"
BIN_NAME="${SNAV_BIN_NAME:-snav}"
VERSION="${SNAV_VERSION:-latest}"
INSTALL_DIR="${SNAV_INSTALL_DIR:-/usr/local/bin}"

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    printf "missing required command: %s\n" "$1" >&2
    exit 1
  fi
}

detect_os() {
  case "$(uname -s)" in
    Linux*) echo "linux" ;;
    Darwin*) echo "darwin" ;;
    MINGW*|MSYS*|CYGWIN*|Windows_NT) echo "windows" ;;
    *)
      printf "unsupported operating system: %s\n" "$(uname -s)" >&2
      exit 1
      ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo "amd64" ;;
    arm64|aarch64) echo "arm64" ;;
    *)
      printf "unsupported architecture: %s\n" "$(uname -m)" >&2
      exit 1
      ;;
  esac
}

resolve_version() {
  if [ "$VERSION" != "latest" ]; then
    printf "%s" "$VERSION"
    return
  fi

  need_cmd curl
  api_url="https://api.github.com/repos/${REPO}/releases/latest"
  tag="$({ curl -fsSL "$api_url" || true; } | tr -d '\n' | sed -n 's/.*"tag_name":"\([^"]*\)".*/\1/p')"
  if [ -z "$tag" ]; then
    printf "failed to resolve latest release for %s\n" "$REPO" >&2
    exit 1
  fi
  printf "%s" "$tag"
}

download() {
  local url="$1"
  local out="$2"
  curl -fL --retry 3 --retry-delay 1 "$url" -o "$out"
}

verify_checksum() {
  local archive="$1"
  local checksum_file="$2"

  if [ ! -f "$checksum_file" ]; then
    return
  fi

  local expected
  expected="$(awk -v name="$(basename "$archive")" '$2==name {print $1}' "$checksum_file" | head -n1)"
  if [ -z "$expected" ]; then
    printf "warning: no checksum found for %s\n" "$(basename "$archive")" >&2
    return
  fi

  local actual
  if command -v shasum >/dev/null 2>&1; then
    actual="$(shasum -a 256 "$archive" | awk '{print $1}')"
  elif command -v sha256sum >/dev/null 2>&1; then
    actual="$(sha256sum "$archive" | awk '{print $1}')"
  else
    printf "warning: cannot verify checksum (shasum/sha256sum not found)\n" >&2
    return
  fi

  if [ "$expected" != "$actual" ]; then
    printf "checksum mismatch for %s\n" "$(basename "$archive")" >&2
    exit 1
  fi
}

install_binary() {
  local src="$1"
  local dst="${INSTALL_DIR}/${BIN_NAME}"

  if [ "$OS" = "windows" ]; then
    dst="${INSTALL_DIR}/${BIN_NAME}.exe"
  fi

  if [ -w "$INSTALL_DIR" ]; then
    install -m 0755 "$src" "$dst"
  elif command -v sudo >/dev/null 2>&1; then
    sudo install -m 0755 "$src" "$dst"
  else
    printf "no permission to write %s (and sudo not available)\n" "$INSTALL_DIR" >&2
    exit 1
  fi

  clear_macos_quarantine "$dst"

  printf "installed %s to %s\n" "$BIN_NAME" "$dst"
}

clear_macos_quarantine() {
  local path="$1"

  if [ "$OS" != "darwin" ]; then
    return
  fi
  if ! command -v xattr >/dev/null 2>&1; then
    printf "warning: xattr not found, cannot clear quarantine attribute\n" >&2
    return
  fi
  if ! xattr -p com.apple.quarantine "$path" >/dev/null 2>&1; then
    return
  fi

  if xattr -d com.apple.quarantine "$path" >/dev/null 2>&1; then
    printf "removed quarantine attribute from %s\n" "$path"
    return
  fi

  if command -v sudo >/dev/null 2>&1 && sudo xattr -d com.apple.quarantine "$path" >/dev/null 2>&1; then
    printf "removed quarantine attribute from %s\n" "$path"
    return
  fi

  printf "warning: failed to remove quarantine attribute from %s\n" "$path" >&2
}

need_cmd curl
need_cmd tar
need_cmd install

OS="$(detect_os)"
ARCH="$(detect_arch)"
TAG="$(resolve_version)"

if [ "$OS" = "windows" ]; then
  need_cmd unzip
  EXT="zip"
  BIN_FILE="${BIN_NAME}.exe"
else
  EXT="tar.gz"
  BIN_FILE="${BIN_NAME}"
fi

ARCHIVE="${BIN_NAME}_${TAG}_${OS}_${ARCH}.${EXT}"
BASE_URL="https://github.com/${REPO}/releases/download/${TAG}"

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

archive_path="${tmpdir}/${ARCHIVE}"
checksum_path="${tmpdir}/checksums.txt"

printf "downloading %s\n" "$ARCHIVE"
download "${BASE_URL}/${ARCHIVE}" "$archive_path"

if download "${BASE_URL}/checksums.txt" "$checksum_path"; then
  verify_checksum "$archive_path" "$checksum_path"
else
  printf "warning: checksums.txt not found, skipping checksum verification\n" >&2
fi

if [ "$OS" = "windows" ]; then
  unzip -q "$archive_path" -d "$tmpdir"
else
  tar -xzf "$archive_path" -C "$tmpdir"
fi

binary_path="${tmpdir}/${BIN_FILE}"
if [ ! -f "$binary_path" ]; then
  printf "binary not found in archive: %s\n" "$BIN_FILE" >&2
  exit 1
fi

install_binary "$binary_path"

printf "done\n"
