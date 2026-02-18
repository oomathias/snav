#!/usr/bin/env bash

set -euo pipefail
umask 022

REPO="oomathias/snav"
BIN_NAME="snav"

# Supported env vars:
# - SNAV_VERSION: release tag to install (default: latest)
# - SNAV_INSTALL_DIR: destination directory (default: /usr/local/bin)
# - SNAV_REQUIRE_SIGNATURE: signature mode (auto, 0, or 1; default: auto)
VERSION="${SNAV_VERSION:-latest}"
INSTALL_DIR="${SNAV_INSTALL_DIR:-/usr/local/bin}"
REQUIRE_SIGNATURE="${SNAV_REQUIRE_SIGNATURE:-auto}"

abort() {
  printf "%s\n" "$1" >&2
  exit 1
}

need() {
  command -v "$1" >/dev/null 2>&1 || abort "missing required command: $1"
}

download() {
  local url="$1"
  local out="$2"

  case "$url" in
    https://*) ;;
    *) abort "refusing non-https URL: $url" ;;
  esac

  curl --fail --show-error --silent --location --proto '=https' --tlsv1.2 --retry 3 --retry-delay 1 "$url" -o "$out"
}

download_optional() {
  local url="$1"
  local out="$2"

  case "$url" in
    https://*) ;;
    *) abort "refusing non-https URL: $url" ;;
  esac

  curl --fail --silent --location --proto '=https' --tlsv1.2 --retry 3 --retry-delay 1 "$url" -o "$out"
}

safe_member_path() {
  case "$1" in
    ""|/*|../*|*/../*|*"/.."|..|*\\*|*$'\n'*|*$'\r'*) return 1 ;;
    *) return 0 ;;
  esac
}

need curl
need install
need mktemp
need awk
need mkdir

case "$(uname -s)" in
  Linux*) OS="linux" ;;
  Darwin*) OS="darwin" ;;
  MINGW*|MSYS*|CYGWIN*|Windows_NT) OS="windows" ;;
  *) abort "unsupported operating system: $(uname -s)" ;;
esac

case "$(uname -m)" in
  x86_64|amd64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *) abort "unsupported architecture: $(uname -m)" ;;
esac

case "$VERSION" in
  latest) ;;
  ""|*[!A-Za-z0-9._+-]*) abort "invalid SNAV_VERSION: $VERSION" ;;
  *) ;;
esac

case "$REQUIRE_SIGNATURE" in
  auto)
    if command -v cosign >/dev/null 2>&1; then
      REQUIRE_SIGNATURE="1"
    else
      REQUIRE_SIGNATURE="0"
    fi
    ;;
  0|1) ;;
  *) abort "invalid SNAV_REQUIRE_SIGNATURE: $REQUIRE_SIGNATURE (expected auto, 0, or 1)" ;;
esac

if [ -z "$INSTALL_DIR" ]; then
  abort "invalid SNAV_INSTALL_DIR"
fi
if [ "$OS" = "windows" ]; then
  case "$INSTALL_DIR" in
    [A-Za-z]:/*|[A-Za-z]:\\*|/*) ;;
    *) abort "SNAV_INSTALL_DIR must be an absolute path: $INSTALL_DIR" ;;
  esac
else
  case "$INSTALL_DIR" in
    /*) ;;
    *) abort "SNAV_INSTALL_DIR must be an absolute path: $INSTALL_DIR" ;;
  esac
fi

if [ "$VERSION" = "latest" ]; then
  latest_release_json="$(curl --fail --show-error --silent --location --proto '=https' --tlsv1.2 "https://api.github.com/repos/${REPO}/releases/latest")"
  TAG="$(awk -F '"' '/"tag_name"[[:space:]]*:/ {print $4; exit}' <<<"$latest_release_json")"
  [ -n "$TAG" ] || abort "failed to resolve latest release tag"
else
  TAG="$VERSION"
fi
ARCHIVE_VERSION="${TAG#v}"

if [ "$OS" = "windows" ]; then
  need unzip
  EXT="zip"
  BIN_FILE="${BIN_NAME}.exe"
else
  need tar
  EXT="tar.gz"
  BIN_FILE="$BIN_NAME"
fi

ARCHIVE="${BIN_NAME}_${ARCHIVE_VERSION}_${OS}_${ARCH}.${EXT}"
BASE_URL="https://github.com/${REPO}/releases/download/${TAG}"

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

archive_path="${tmpdir}/${ARCHIVE}"
checksum_path="${tmpdir}/checksums.txt"
checksum_bundle_path="${tmpdir}/checksums.txt.sigstore.json"
binary_path="${tmpdir}/${BIN_FILE}"

printf "downloading %s\n" "$ARCHIVE"
download "${BASE_URL}/${ARCHIVE}" "$archive_path"

printf "downloading checksums.txt\n"
download "${BASE_URL}/checksums.txt" "$checksum_path"

printf "downloading checksums.txt.sigstore.json\n"
if download_optional "${BASE_URL}/checksums.txt.sigstore.json" "$checksum_bundle_path"; then
  if command -v cosign >/dev/null 2>&1; then
    printf "verifying checksums signature\n"
    cosign verify-blob \
      --bundle "$checksum_bundle_path" \
      --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
      --certificate-identity "https://github.com/${REPO}/.github/workflows/release.yml@refs/tags/${TAG}" \
      "$checksum_path" >/dev/null 2>&1 || abort "checksums signature verification failed"
  elif [ "$REQUIRE_SIGNATURE" = "1" ]; then
    abort "cosign is required for signature verification (install cosign or set SNAV_REQUIRE_SIGNATURE=0)"
  else
    printf "warning: cosign not found; skipping checksums signature verification\n" >&2
  fi
elif [ "$REQUIRE_SIGNATURE" = "1" ]; then
  abort "checksums signature not available for ${TAG}"
else
  printf "warning: checksums signature not available for ${TAG}; continuing with unsigned checksum\n" >&2
fi

expected="$(awk -v name="$ARCHIVE" '$2==name || $2=="*"name || $NF==name || $NF=="*"name {print $1; exit}' "$checksum_path")"
[ -n "$expected" ] || abort "checksum missing for ${ARCHIVE}"
case "$expected" in
  *[!A-Fa-f0-9]*) abort "invalid checksum format for ${ARCHIVE}" ;;
esac
[ "${#expected}" -eq 64 ] || abort "unexpected checksum length for ${ARCHIVE}"

if command -v shasum >/dev/null 2>&1; then
  actual="$(shasum -a 256 "$archive_path" | awk '{print $1}')"
elif command -v sha256sum >/dev/null 2>&1; then
  actual="$(sha256sum "$archive_path" | awk '{print $1}')"
else
  abort "cannot verify checksum (shasum or sha256sum not found)"
fi

[ "$expected" = "$actual" ] || abort "checksum mismatch for ${ARCHIVE}"

if [ "$OS" = "windows" ]; then
  archive_members="$(unzip -Z1 "$archive_path")"
  member="$(awk -v f="$BIN_FILE" '$0 !~ /\/$/ && $0 ~ "(^|/)" f "$" {print; exit}' <<<"$archive_members")"
else
  archive_members="$(tar -tzf "$archive_path")"
  member="$(awk -v f="$BIN_FILE" '$0 !~ /\/$/ && $0 ~ "(^|/)" f "$" {print; exit}' <<<"$archive_members")"
fi

[ -n "$member" ] || abort "binary not found in archive: ${BIN_FILE}"
safe_member_path "$member" || abort "unsafe binary path in archive: $member"

if [ "$OS" = "windows" ]; then
  unzip -p "$archive_path" "$member" >"$binary_path"
else
  tar -xOf "$archive_path" "$member" >"$binary_path"
fi

[ -s "$binary_path" ] || abort "failed to extract binary payload"
chmod 0755 "$binary_path"

if [ ! -d "$INSTALL_DIR" ]; then
  if ! mkdir -p "$INSTALL_DIR" >/dev/null 2>&1; then
    if command -v sudo >/dev/null 2>&1; then
      sudo mkdir -p "$INSTALL_DIR"
    else
      abort "failed to create install directory: $INSTALL_DIR"
    fi
  fi
fi

dst="${INSTALL_DIR}/${BIN_FILE}"
if [ -w "$INSTALL_DIR" ]; then
  install -m 0755 "$binary_path" "$dst"
elif command -v sudo >/dev/null 2>&1; then
  sudo install -m 0755 "$binary_path" "$dst"
else
  abort "no permission to write ${INSTALL_DIR} (and sudo not available)"
fi

printf "installed %s to %s\n" "$BIN_NAME" "$dst"
printf "done\n"
