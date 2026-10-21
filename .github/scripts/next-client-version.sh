#!/usr/bin/env bash
# 解析 aris 客户端发布版本：master 上出现客户端相关变更时自动 patch 递增，
# 否则不发布。结果同时写入 GITHUB_OUTPUT，本地可直接跑（EXPORT 到终端）。
#
# 输入（环境变量）：
#   BEFORE         push 事件的上一个提交；空/全零/不可达时退回与父提交比较
#   GITHUB_SHA     本次提交；本地缺省 HEAD
#   EVENT_NAME     push / workflow_dispatch；dispatch 时与最近一个 tag 比较
#   INPUT_VERSION  手动指定版本，给了就直接发（跳过变更检测）
#
# 输出：
#   should_release=true|false
#   version=vX.Y.Z（should_release=false 时为空）
set -euo pipefail

readonly MODULE="github.com/hcd233/aris-proxy-api"
readonly CLIENT_PKG="./cmd/client"
event="${EVENT_NAME:-push}"
input_version="${INPUT_VERSION:-}"

emit() {
	if [ -n "${GITHUB_OUTPUT:-}" ]; then
		printf '%s=%s\n' "$1" "$2" >>"$GITHUB_OUTPUT"
	fi
	printf '%s=%s\n' "$1" "$2"
}

die() {
	printf 'error: %s\n' "$1" >&2
	exit 1
}

version_ok() { [[ "$1" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; }

latest_tag() { git tag --list 'v[0-9]*.[0-9]*.[0-9]*' --sort=-v:refname | head -n 1; }

bump_patch() {
	local major minor patch
	IFS=. read -r major minor patch <<<"${1#v}"
	printf 'v%s.%s.%s\n' "$major" "$minor" "$((patch + 1))"
}

# 递增到未被占用的版本，避免并发/重跑撞已有 tag
next_free_version() {
	local version="$1"
	while git rev-parse -q --verify "refs/tags/${version}" >/dev/null 2>&1; do
		version=$(bump_patch "$version")
	done
	printf '%s\n' "$version"
}

# 客户端编译输入：cmd/client 的包依赖闭包（覆盖 cmd/、internal/ 下的公共包）+ 构建入口
client_inputs() {
	go list -deps -f '{{if and .Module (eq .Module.Path "'"$MODULE"'")}}{{.ImportPath}}{{end}}' "$CLIENT_PKG" |
		sed "s#^${MODULE}/##" | grep -v '^$' || true
	printf '%s\n' go.mod go.sum Makefile
}

changed_files() {
	local before="${BEFORE:-}"
	if [ "$event" = "workflow_dispatch" ]; then
		local base
		base=$(latest_tag)
		[ -n "$base" ] && git diff --name-only "$base" "${GITHUB_SHA:-HEAD}" && return 0
	fi
	if [ -n "$before" ] && git cat-file -e "${before}^{commit}" 2>/dev/null; then
		git diff --name-only "$before" "${GITHUB_SHA:-HEAD}"
		return 0
	fi
	# 首次推送或强推：before 不可达，退回与父提交比较
	git diff --name-only HEAD~1 HEAD 2>/dev/null || git ls-tree -r --name-only HEAD
}

# 变更文件里是否命中客户端编译输入（目录前缀或文件全等）
touches_client() {
	local changed="$1" inputs file path
	inputs=$(mktemp)
	client_inputs | sort -u >"$inputs"
	while IFS= read -r file; do
		[ -n "$file" ] || continue
		while IFS= read -r path; do
			[ -n "$path" ] || continue
			if [ "$file" = "$path" ] || [[ "$file" == "$path"/* ]]; then
				rm -f "$inputs"
				return 0
			fi
		done <"$inputs"
	done <<<"$changed"
	rm -f "$inputs"
	return 1
}

if [ -n "$input_version" ]; then
	version_ok "$input_version" || die "invalid version: ${input_version}"
	echo "manually requested version: ${input_version}"
	emit should_release true
	emit version "$input_version"
	exit 0
fi

changed=$(changed_files || true)
if [ -z "$changed" ]; then
	echo "no changed files"
	emit should_release false
	emit version ""
	exit 0
fi

if ! touches_client "$changed"; then
	echo "no aris client changes"
	emit should_release false
	emit version ""
	exit 0
fi

latest=$(latest_tag)
version="v0.0.1"
[ -n "$latest" ] && version=$(bump_patch "$latest")
version=$(next_free_version "$version")
version_ok "$version" || die "invalid computed version: ${version}"
echo "aris client changed; next version: ${version}"
emit should_release true
emit version "$version"
