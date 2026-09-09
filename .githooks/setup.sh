#!/bin/bash

# 设置Git hooks目录
git config core.hooksPath .githooks

chmod +x .githooks/pre-commit

# Git LFS：docs/diagrams 下的生成产物（archify HTML/GIF）由 LFS 跟踪，
# 未安装 git-lfs 时 checkout 得到的是指针文件。
if command -v git-lfs >/dev/null 2>&1; then
    git lfs install --local
else
    echo "Warning: git-lfs not installed; docs/diagrams/* will checkout as LFS pointers. Install: https://git-lfs.com"
fi

if [ -f script/sync-agent-instructions-symlinks.sh ]; then
    chmod +x script/sync-agent-instructions-symlinks.sh
    sh script/sync-agent-instructions-symlinks.sh
fi

if [ -f script/sync-skills-symlinks.sh ]; then
    chmod +x script/sync-skills-symlinks.sh
    sh script/sync-skills-symlinks.sh
fi

echo "Git hooks have been configured successfully!"
