#!/bin/sh
# 安装 Git Hook：将项目脚本链接到 .git/hooks 目录
HOOKS_DIR=".git/hooks"
SOURCE_HOOKS_DIR="scripts/git-hooks"

# 遍历所有钩子脚本，创建软链接
for hook in $(ls "$SOURCE_HOOKS_DIR"); do
  hook_path="$HOOKS_DIR/$hook"
  source_path="../../$SOURCE_HOOKS_DIR/$hook"
  # 若已存在钩子，备份后替换
  if [ -f "$hook_path" ]; then
    mv "$hook_path" "$hook_path.backup"
  fi
  # 创建软链接（相对路径，确保项目移动后仍生效）
  ln -s "$source_path" "$hook_path"
  echo "Installed hook: $hook"
done

echo "Git hooks installed successfully"