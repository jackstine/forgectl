#!/usr/bin/env bash
# install-forgectl.sh — Standalone installer for forgectl.
# Downloads the Go binary and Python project from a GitHub release,
# installs the Python environment via uv, and verifies the installation.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/jackstine/forgectl/main/scripts/install/install-forgectl.sh | bash
#   bash scripts/install/install-forgectl.sh --version v0.2.0
set -euo pipefail

REPO="jackstine/forgectl"
INSTALL_BIN="${INSTALL_BIN:-$HOME/.local/bin}"
INSTALL_DATA="${INSTALL_DATA:-$HOME/.local/share/forgectl}"

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

info()  { printf '  \033[1;34m>\033[0m %s\n' "$*"; }
error() { printf '  \033[1;31mError:\033[0m %s\n' "$*" >&2; exit 1; }

need_cmd() {
    command -v "$1" >/dev/null 2>&1 || error "$1 is required but not found. $2"
}

detect_platform() {
    OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
    ARCH="$(uname -m)"
    case "$ARCH" in
        x86_64)        ARCH="amd64" ;;
        aarch64|arm64) ARCH="arm64" ;;
        *)             error "Unsupported architecture: $ARCH" ;;
    esac
    case "$OS" in
        linux|darwin) ;;
        *)            error "Unsupported OS: $OS (use WSL on Windows)" ;;
    esac
}

# ---------------------------------------------------------------------------
# Parse arguments
# ---------------------------------------------------------------------------

VERSION=""
while [[ $# -gt 0 ]]; do
    case "$1" in
        --version) VERSION="$2"; shift 2 ;;
        *)         error "Unknown argument: $1" ;;
    esac
done

# ---------------------------------------------------------------------------
# Prerequisites
# ---------------------------------------------------------------------------

need_cmd curl "Install curl first."
need_cmd tar  "Install tar first."
need_cmd uv   "Install uv: https://docs.astral.sh/uv/getting-started/installation/"

detect_platform

# ---------------------------------------------------------------------------
# Resolve version
# ---------------------------------------------------------------------------

if [[ -z "$VERSION" ]]; then
    info "Fetching latest release..."
    VERSION="$(curl -sL "https://api.github.com/repos/${REPO}/releases/latest" \
        | grep '"tag_name"' | head -1 | cut -d'"' -f4)"
    [[ -n "$VERSION" ]] || error "Could not determine latest version from GitHub API"
fi

info "Installing forgectl ${VERSION} for ${OS}/${ARCH}"

RELEASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"
TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

# ---------------------------------------------------------------------------
# Download artifacts
# ---------------------------------------------------------------------------

ARCHIVE_NAME="forgectl_${VERSION#v}_${OS}_${ARCH}.tar.gz"

info "Downloading binary..."
curl -fsSL -o "${TMPDIR}/binary.tar.gz" "${RELEASE_URL}/${ARCHIVE_NAME}"

info "Downloading Python project..."
curl -fsSL -o "${TMPDIR}/python.tar.gz" "${RELEASE_URL}/reverse-engineer-python.tar.gz"

info "Downloading checksums..."
curl -fsSL -o "${TMPDIR}/checksums.txt" "${RELEASE_URL}/checksums.txt"

# ---------------------------------------------------------------------------
# Verify checksums
# ---------------------------------------------------------------------------

info "Verifying checksums..."
cd "$TMPDIR"
# Extract the expected checksums for our two artifacts.
grep "$ARCHIVE_NAME" checksums.txt | shasum -a 256 -c --quiet 2>/dev/null \
    || grep "$ARCHIVE_NAME" checksums.txt | sha256sum -c --quiet 2>/dev/null \
    || error "Checksum verification failed for ${ARCHIVE_NAME}"

grep "reverse-engineer-python.tar.gz" checksums.txt | shasum -a 256 -c --quiet 2>/dev/null \
    || grep "reverse-engineer-python.tar.gz" checksums.txt | sha256sum -c --quiet 2>/dev/null \
    || error "Checksum verification failed for reverse-engineer-python.tar.gz"

# ---------------------------------------------------------------------------
# Install Go binary
# ---------------------------------------------------------------------------

info "Installing binary to ${INSTALL_BIN}/forgectl..."
mkdir -p "$INSTALL_BIN"
tar xzf "${TMPDIR}/binary.tar.gz" -C "$TMPDIR"
cp "${TMPDIR}/forgectl" "${INSTALL_BIN}/forgectl"
chmod +x "${INSTALL_BIN}/forgectl"

# ---------------------------------------------------------------------------
# Install Python environment
# ---------------------------------------------------------------------------

PYTHON_DIR="${INSTALL_DATA}/${VERSION}/python"
info "Installing Python environment to ${PYTHON_DIR}..."
mkdir -p "$PYTHON_DIR"
tar xzf "${TMPDIR}/python.tar.gz" -C "$PYTHON_DIR"

cd "$PYTHON_DIR"
uv sync --frozen --no-dev

info "Generating checksums..."
SHA_CMD="$(command -v sha256sum 2>/dev/null || echo "shasum -a 256")"
find src -type f \( -name '*.py' -o -name '*.md' \) -exec $SHA_CMD {} + > checksums.sha256

# ---------------------------------------------------------------------------
# Verify
# ---------------------------------------------------------------------------

info "Verifying installation..."
"${INSTALL_BIN}/forgectl" --version

echo ""
info "forgectl ${VERSION} installed successfully!"
echo ""

# Check PATH
case ":$PATH:" in
    *":${INSTALL_BIN}:"*) ;;
    *)
        echo "  Add ${INSTALL_BIN} to your PATH:"
        echo ""
        echo "    export PATH=\"${INSTALL_BIN}:\$PATH\""
        echo ""
        echo "  Add this to your ~/.bashrc or ~/.zshrc to make it permanent."
        echo ""
        ;;
esac
