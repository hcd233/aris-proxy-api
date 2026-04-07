#!/bin/bash

# 设置Git hooks目录
git config core.hooksPath .githooks

chmod +x .githooks/pre-commit

echo "Git hooks have been configured successfully!"