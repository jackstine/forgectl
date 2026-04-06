#!/usr/bin/env bash
# package-python.sh — Creates a release tarball of the Python reverse-engineer project.
# Called by GoReleaser before.hooks or the GitHub Actions workflow.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
PYTHON_DIR="$PROJECT_ROOT/reverse_engineer"

mkdir -p "$PROJECT_ROOT/dist"

tar czf "$PROJECT_ROOT/dist/reverse-engineer-python.tar.gz" \
    --exclude='__pycache__' \
    --exclude='.venv' \
    --exclude='.pytest_cache' \
    --exclude='tests' \
    -C "$PYTHON_DIR" .

echo "Packaged Python project: dist/reverse-engineer-python.tar.gz"
