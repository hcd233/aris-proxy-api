#!/bin/sh
set -eu

# Sync AGENTS.md to CODEBUDDY.md and CLAUDE.md via relative symlinks.
# AGENTS.md is the single source of truth; the other two are mirror symlinks
# so any edit to AGENTS.md is automatically reflected everywhere.
#
# Idempotent: safe to run repeatedly (e.g. from .githooks/pre-commit).

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
REPO_ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)
SOURCE_FILE="AGENTS.md"
SOURCE_PATH="$REPO_ROOT/$SOURCE_FILE"

INFO="\033[34m[INFO]\033[0m"
WARN="\033[33m[WARN]\033[0m"
PASS="\033[32m[PASS]\033[0m"

if [ ! -f "$SOURCE_PATH" ]; then
    printf "$WARN %s not found — skipping agent instructions symlink sync\n" "$SOURCE_FILE"
    exit 0
fi

# Targets are sibling files in the repo root; symlink target is the bare
# filename so the link is relative and relocatable.
TARGETS="CODEBUDDY.md CLAUDE.md"

synced=0
skipped=0
warned=0

for target in $TARGETS; do
    link_path="$REPO_ROOT/$target"

    # Already a correct symlink -> nothing to do.
    if [ -L "$link_path" ]; then
        current_target=$(readlink "$link_path")
        if [ "$current_target" = "$SOURCE_FILE" ]; then
            skipped=$((skipped + 1))
            continue
        fi
        # Symlink exists but points elsewhere — fix it.
        rm "$link_path"
        ln -s "$SOURCE_FILE" "$link_path"
        printf "$INFO relinked %s -> %s\n" "$target" "$SOURCE_FILE"
        synced=$((synced + 1))
        continue
    fi

    # Real file (not a symlink) — do not clobber blindly, warn so the user
    # can resolve the divergence intentionally. This mirrors the behavior of
    # sync-skills-symlinks.sh.
    if [ -e "$link_path" ]; then
        printf "$WARN %s exists and is not a symlink — keeping it unchanged\n" "$target"
        warned=$((warned + 1))
        continue
    fi

    # Nothing exists yet — create the symlink.
    ln -s "$SOURCE_FILE" "$link_path"
    printf "$INFO linked %s -> %s\n" "$target" "$SOURCE_FILE"
    synced=$((synced + 1))
done

printf "$PASS agent instructions symlink sync complete: linked=%s skipped=%s warnings=%s\n" \
    "$synced" "$skipped" "$warned"
