#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
REPO_ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)
SOURCE_DIR="$REPO_ROOT/.agents/skills"
TARGET_DIRS="$REPO_ROOT/.claude/skills $REPO_ROOT/.codebuddy/skills"
RELATIVE_SOURCE_PREFIX="../../.agents/skills"

INFO="\033[34m[INFO]\033[0m"
WARN="\033[33m[WARN]\033[0m"
PASS="\033[32m[PASS]\033[0m"

if [ ! -d "$SOURCE_DIR" ]; then
    printf "$INFO .agents/skills not found — skipping skills symlink sync\n"
    exit 0
fi

for target_dir in $TARGET_DIRS; do
    mkdir -p "$target_dir"
done

synced=0
skipped=0
warned=0

for skill_path in "$SOURCE_DIR"/*; do
    [ -e "$skill_path" ] || continue
    [ -d "$skill_path" ] || continue

    skill_name=${skill_path##*/}
    wanted_target="$RELATIVE_SOURCE_PREFIX/$skill_name"

    for target_dir in $TARGET_DIRS; do
        link_path="$target_dir/$skill_name"

        if [ -L "$link_path" ]; then
            current_target=$(readlink "$link_path")
            if [ "$current_target" = "$wanted_target" ]; then
                skipped=$((skipped + 1))
                continue
            fi
            rm "$link_path"
        elif [ -e "$link_path" ]; then
            printf "$WARN %s exists and is not a symlink — keeping it unchanged\n" "${link_path#$REPO_ROOT/}"
            warned=$((warned + 1))
            continue
        fi

        ln -s "$wanted_target" "$link_path"
        printf "$INFO linked %s -> %s\n" "${link_path#$REPO_ROOT/}" "$wanted_target"
        synced=$((synced + 1))
    done
done

printf "$PASS skills symlink sync complete: linked=%s skipped=%s warnings=%s\n" "$synced" "$skipped" "$warned"
