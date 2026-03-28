#!/usr/bin/env bash
set -euo pipefail

FORGECTL_ROOT="$(cd "$(dirname "$0")/.." && pwd)"

usage() {
  echo "Usage: $0 <target-project-path>"
  echo ""
  echo "Creates a .claude/ directory at the target path with symlinks to"
  echo "forgectl's commands/ and skills/ directories."
  exit 1
}

if [ $# -ne 1 ]; then
  usage
fi

TARGET="$(cd "$1" && pwd)"
CLAUDE_DIR="$TARGET/.claude"

if [ -e "$CLAUDE_DIR" ]; then
  echo "Error: .claude/ already exists at $CLAUDE_DIR"
  echo "Will not override the existing .claude directory."
  exit 1
fi

mkdir -p "$CLAUDE_DIR"
echo "Created $CLAUDE_DIR"

for dir in commands skills; do
  LINK="$CLAUDE_DIR/$dir"
  SRC="$FORGECTL_ROOT/$dir"

  if [ -L "$LINK" ]; then
    echo "Symlink already exists: $LINK -> $(readlink "$LINK")"
  elif [ -e "$LINK" ]; then
    echo "WARNING: $LINK exists but is not a symlink — skipping"
  else
    ln -s "$SRC" "$LINK"
    echo "Linked $LINK -> $SRC"
  fi
done

echo "Done."
