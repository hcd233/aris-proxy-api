#!/usr/bin/env bash
#
# upload-fonts-r2.sh - 上传 Maple Mono CN 分片到 Cloudflare R2 公开桶
#
# 字体已不再随构建产物发布：web/src/styles/maple-mono-cn.css 直接引用 R2 地址，
# 由本脚本负责把 woff2 放上去。文件名是内容哈希，可以放心长期缓存；重复执行会
# 覆盖同名对象，是幂等的。字体包升级后先跑 `npm run fonts:css` 再执行本脚本。
#
# 桶需要已开启公开访问并绑定自定义域（当前：web-storage → static.lvlvko.top），
# CORS 允许站点源以 GET 读取（见 R2 控制台「设置 → CORS 策略」）。
#
# 凭据只从环境变量读取，不落盘：
#   R2_ENDPOINT=https://<account_id>.r2.cloudflarestorage.com \
#   R2_ACCESS_KEY_ID=... R2_SECRET_ACCESS_KEY=... \
#   script/upload-fonts-r2.sh
#
#	@author centonhuang
#	@update 2026-11-07 19:40:00
set -euo pipefail

cd "$(dirname "$0")/.."

BUCKET="${R2_FONT_BUCKET:-web-storage}"
PREFIX="${R2_FONT_PREFIX:-fonts/maple-mono-cn}"
SRC_DIR="web/node_modules/@chinese-fonts/maple-mono-cn/dist"
CACHE_CONTROL="public, max-age=31536000, immutable"

: "${R2_ENDPOINT:?请设置 R2_ENDPOINT，例如 https://<account_id>.r2.cloudflarestorage.com}"
: "${R2_ACCESS_KEY_ID:?请设置 R2_ACCESS_KEY_ID}"
: "${R2_SECRET_ACCESS_KEY:?请设置 R2_SECRET_ACCESS_KEY}"

command -v rclone >/dev/null || { echo "缺少 rclone：https://rclone.org/install/" >&2; exit 1; }

export RCLONE_CONFIG_R2_TYPE=s3
export RCLONE_CONFIG_R2_PROVIDER=Cloudflare
export RCLONE_CONFIG_R2_ACCESS_KEY_ID="${R2_ACCESS_KEY_ID}"
export RCLONE_CONFIG_R2_SECRET_ACCESS_KEY="${R2_SECRET_ACCESS_KEY}"
export RCLONE_CONFIG_R2_ENDPOINT="${R2_ENDPOINT}"
export RCLONE_CONFIG_R2_ACL=private

for pair in Regular:regular Bold:bold; do
	weight="${pair%%:*}"
	slug="${pair##*:}"
	src="${SRC_DIR}/MapleMono-CN-${weight}"
	[ -d "${src}" ] || {
		echo "缺少 ${src}，先在 web/ 执行 npm ci" >&2
		exit 1
	}
	echo "→ ${slug}：$(find "${src}" -name '*.woff2' | wc -l | tr -d ' ') 个 woff2"
	rclone copy "${src}" "r2:${BUCKET}/${PREFIX}/${slug}" \
		--include "*.woff2" \
		--transfers 16 --checkers 16 --no-traverse --s3-no-check-bucket \
		--header-upload "Cache-Control: ${CACHE_CONTROL}" \
		--header-upload "Content-Type: font/woff2"
done

echo "完成：https://static.lvlvko.top/${PREFIX}/{regular,bold}/"
