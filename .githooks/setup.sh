#!/bin/bash

# 设置Git hooks目录
git config core.hooksPath .githooks

chmod +x .githooks/pre-commit

if [ -f script/sync-agent-instructions-symlinks.sh ]; then
    chmod +x script/sync-agent-instructions-symlinks.sh
    sh script/sync-agent-instructions-symlinks.sh
fi

if [ -f script/sync-skills-symlinks.sh ]; then
    chmod +x script/sync-skills-symlinks.sh
    sh script/sync-skills-symlinks.sh
fi

echo "Git hooks have been configured successfully!"
