#!/usr/bin/env bash
set -euo pipefail

REPO="uiratan/ktwins"
PROJECT="ktwins"

log() {
  printf '[ktwins] %s\n' "$*"
}

fail() {
  printf '[ktwins] ERROR: %s\n' "$*" >&2
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "Missing required command: $1"
}

detect_os() {
  case "$(uname -s)" in
    Linux) echo "linux" ;;
    Darwin) echo "darwin" ;;
    *) fail "Unsupported OS: $(uname -s)" ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64 | amd64) echo "amd64" ;;
    aarch64 | arm64) echo "arm64" ;;
    *) fail "Unsupported architecture: $(uname -m)" ;;
  esac
}

fetch_latest_tag() {
  local tag
  tag="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep -m1 '"tag_name":' \
    | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')"
  if [ -z "$tag" ]; then
    fail "Could not determine latest release tag"
  fi
  echo "$tag"
}

choose_bindir() {
  if [ -n "${BIN_DIR:-}" ]; then
    echo "${BIN_DIR}"
    return
  fi
  if [ -w "/usr/local/bin" ]; then
    echo "/usr/local/bin"
  else
    echo "$(pwd)"
  fi
}

main() {
  require_cmd curl
  require_cmd tar

  local os arch tag version artifact url tmpdir bindir
  os="$(detect_os)"
  arch="$(detect_arch)"
  tag="$(fetch_latest_tag)"
  version="${tag#v}"

  artifact="${PROJECT}_${version}_${os}_${arch}.tar.gz"
  url="https://github.com/${REPO}/releases/download/${tag}/${artifact}"

  bindir="$(choose_bindir)"
  mkdir -p "${bindir}"

  tmpdir="$(mktemp -d)"
  trap 'rm -rf "${tmpdir}"' EXIT

  log "Downloading ${url}"
  curl -fL "${url}" -o "${tmpdir}/${artifact}" || fail "Download failed"

  log "Extracting"
  tar -xzf "${tmpdir}/${artifact}" -C "${tmpdir}" || fail "Extraction failed"

  if [ ! -f "${tmpdir}/${PROJECT}" ]; then
    fail "Binary ${PROJECT} not found in archive"
  fi

  log "Installing to ${bindir}"
  install -m 0755 "${tmpdir}/${PROJECT}" "${bindir}/${PROJECT}"

  log "Done. Run: ${bindir}/${PROJECT} [namespace]"
}

main "$@"
