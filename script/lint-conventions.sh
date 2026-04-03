#!/usr/bin/env bash
# lint-conventions.sh — 项目编码规范扫描脚本
# 扫描 internal/ 和 test/ 目录，检查是否违反项目约定
# 退出码: 0=全部通过, 1=存在违规
set -euo pipefail

RED='\033[0;31m'
YELLOW='\033[0;33m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

ERRORS=0
WARNINGS=0

error() {
    echo -e "${RED}[ERROR]${NC} $1"
    ERRORS=$((ERRORS + 1))
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
    WARNINGS=$((WARNINGS + 1))
}

info() {
    echo -e "${CYAN}[INFO]${NC} $1"
}

section() {
    echo ""
    echo -e "${CYAN}━━━ $1 ━━━${NC}"
}

# ─────────────────────────────────────────────
# 1. 错误处理规范
# ─────────────────────────────────────────────
section "Error Handling"

# 1.1 禁止使用 fmt.Errorf 创建内部错误
matches=$(grep -rn 'fmt\.Errorf' internal/ --include='*.go' 2>/dev/null || true)
if [[ -n "$matches" ]]; then
    error "禁止使用 fmt.Errorf，必须通过 ierr.Wrap/ierr.New 创建错误:"
    echo "$matches" | head -20
fi

# 1.2 禁止使用 errors.New 创建内部错误（排除 errors.Is/errors.As/errors.Unwrap）
matches=$(grep -rn 'errors\.New(' internal/ --include='*.go' 2>/dev/null || true)
if [[ -n "$matches" ]]; then
    error "禁止使用 errors.New()，必须通过 ierr.New 创建错误:"
    echo "$matches" | head -20
fi

# 1.3 禁止使用已废弃的 constant.ErrXxx
matches=$(grep -rn 'constant\.Err[A-Z]' internal/ --include='*.go' \
    | grep -v 'constant/error.go' 2>/dev/null || true)
if [[ -n "$matches" ]]; then
    error "禁止使用 constant.ErrXxx（已废弃），使用 ierr.ErrXxx.BizError():"
    echo "$matches" | head -20
fi

# ─────────────────────────────────────────────
# 2. 日志规范
# ─────────────────────────────────────────────
section "Logging"

# 2.1 日志消息必须使用 [ModuleName] 前缀格式
# 匹配 logger.Info/Error/Warn/Debug("xxx 但不以 [ 开头的消息
matches=$(grep -rn -E 'logger\.(Info|Error|Warn|Debug)\(' internal/ --include='*.go' \
    | grep -v '\[' 2>/dev/null || true)
# 过滤掉只有变量引用（无字符串字面量）的行
matches=$(echo "$matches" | grep '"' | grep -v '"\[' 2>/dev/null || true)
if [[ -n "$matches" ]]; then
    warn "日志消息应使用 [ModuleName] 前缀格式，如 logger.Info(\"[XxxService] ...\"):"
    echo "$matches" | head -20
fi

# 2.2 日志中禁止裸记录敏感信息（检查常见敏感字段名未用 MaskSecret）
matches=$(grep -rn -E 'zap\.String.*(Key|Token|Secret|Password)' internal/ --include='*.go' \
    | grep -v 'MaskSecret' \
    | grep -v 'CtxKey' \
    | grep -v 'apiKeyName\|APIKeyName' \
    | grep -v 'lockKey\|cacheKey\|configKey\|routeKey\|sortKey' \
    | grep -v 'tokenType\|TokenType\|tokenExpir' \
    | grep -v 'sessionAPIKeyName' \
    2>/dev/null || true)
if [[ -n "$matches" ]]; then
    warn "日志中记录敏感信息（Key/Token/Secret/Password）应使用 util.MaskSecret():"
    echo "$matches" | head -10
fi

# ─────────────────────────────────────────────
# 3. JSON 库规范
# ─────────────────────────────────────────────
section "JSON Library"

# 3.1 禁止使用 encoding/json
matches=$(grep -rn '"encoding/json"' internal/ test/ --include='*.go' 2>/dev/null || true)
if [[ -n "$matches" ]]; then
    error "禁止使用 encoding/json，统一使用 github.com/bytedance/sonic:"
    echo "$matches" | head -20
fi

# 3.2 禁止使用 json.RawMessage
matches=$(grep -rn 'json\.RawMessage' internal/ --include='*.go' 2>/dev/null || true)
if [[ -n "$matches" ]]; then
    error "禁止使用 json.RawMessage:"
    echo "$matches" | head -20
fi

# ─────────────────────────────────────────────
# 4. 测试规范
# ─────────────────────────────────────────────
section "Testing"

# 4.1 internal/ 下禁止存放测试文件
matches=$(find internal/ -name '*_test.go' 2>/dev/null || true)
if [[ -n "$matches" ]]; then
    error "禁止在 internal/ 目录中存放 *_test.go 文件，所有测试必须放在 test/ 目录:"
    echo "$matches"
fi

# 4.2 test/ 下禁止散落的测试文件（必须在子目录中）
matches=$(find test/ -maxdepth 1 -name '*_test.go' 2>/dev/null || true)
if [[ -n "$matches" ]]; then
    error "禁止在 test/ 根目录直接放 *_test.go，必须放入主题子目录:"
    echo "$matches"
fi

# 4.3 测试中禁止使用 testify 等第三方断言库
matches=$(grep -rn '"github.com/stretchr/testify' test/ --include='*.go' 2>/dev/null || true)
if [[ -n "$matches" ]]; then
    error "禁止使用 testify 等第三方断言库，使用标准库 testing 包:"
    echo "$matches" | head -20
fi

# ─────────────────────────────────────────────
# 5. 类型安全规范
# ─────────────────────────────────────────────
section "Type Safety"

# 5.1 禁止在核心业务代码中使用 interface{}（排除基础设施/第三方适配层）
matches=$(grep -rn 'interface{}' internal/service/ internal/handler/ internal/dto/ --include='*.go' 2>/dev/null || true)
if [[ -n "$matches" ]]; then
    warn "核心业务层避免使用 interface{}，优先使用具体类型或泛型:"
    echo "$matches" | head -10
fi

# ─────────────────────────────────────────────
# 6. 死代码检查
# ─────────────────────────────────────────────
section "Dead Code"

# 6.1 检查注释掉的代码块（连续注释的可执行代码）
matches=$(grep -rn '^[[:space:]]*//' internal/ --include='*.go' \
    | grep -v '// @' \
    | grep -v '//\t@' \
    | grep -v '// Package' \
    | grep -v '// nolint' \
    | grep -v '//go:' \
    | grep -v '//nolint' \
    | grep -v '//[[:space:]]	*return [a-zA-Z]' \
    | grep -E '//\s*(func |if |for |var |type |const |switch |case |err :?=|ctx\.|req\.|rsp\.)' \
    2>/dev/null || true)
if [[ -n "$matches" ]]; then
    warn "可能存在被注释掉的死代码，请确认是否需要删除:"
    echo "$matches" | head -10
fi

# ─────────────────────────────────────────────
# 7. 命名规范
# ─────────────────────────────────────────────
section "Naming"

# 7.1 禁止暴露实现细节的命名（userList, userMap, userSlice 等）
# 排除: lo.SliceToMap / lo.MapToSlice 等工具函数, 局部 map 状态变量
# 排除: map[string]any 类型的 JSON 反序列化临时变量 (bodyMap, dataMap, msgMap 等)
matches=$(grep -rn -E '[a-z](List|Map|Slice|Array)\b' internal/ --include='*.go' \
    | grep -v '_test.go' \
    | grep -v '// ' \
    | grep -v 'func ' \
    | grep -v 'lo\.' \
    | grep -v 'SliceToMap\|MapToSlice' \
    | grep -E '(var |:=)' \
    | grep -v -E '(state|State|choice|toolCall|block)Map' \
    | grep -v 'blackList\|whiteList\|allowList\|denyList' \
    | grep -v 'map\[string\]any' \
    | grep -v -E '(body|data|msg|message|tool|existing)Map' \
    | grep -v 'SchemaMap' \
    2>/dev/null || true)
if [[ -n "$matches" ]]; then
    warn "变量命名可能暴露了实现细节（如 xxxList/xxxMap），建议使用复数形式（如 users, orders）:"
    echo "$matches" | head -10
fi

# ─────────────────────────────────────────────
# 8. 导入规范
# ─────────────────────────────────────────────
section "Imports"

# 8.1 禁止使用 time.Sleep 做同步（测试中）
matches=$(grep -rn 'time\.Sleep' test/ --include='*.go' 2>/dev/null || true)
if [[ -n "$matches" ]]; then
    error "禁止在测试中使用 time.Sleep() 做同步，使用 channel/WaitGroup/deadline:"
    echo "$matches" | head -10
fi

# ─────────────────────────────────────────────
# 9. 中间件/Service 层规范
# ─────────────────────────────────────────────
section "Architecture"

# 9.1 Handler 层不应包含业务逻辑（检查 handler 中是否直接操作 dao/db）
matches=$(grep -rn -E '(dao\.|database\.GetDB|\.Where\(|\.Find\(|\.Create\(|\.Save\()' internal/handler/ --include='*.go' \
    | grep -v 'h\.svc\.' \
    | grep -v '// ' \
    2>/dev/null || true)
if [[ -n "$matches" ]]; then
    error "Handler 层禁止直接操作 DAO/DB，业务逻辑应放在 Service 层:"
    echo "$matches" | head -10
fi

# 9.2 Service 层不应直接返回 Go error 给 Handler（检查 return xxx, err 模式中 err 非 nil）
# 此检查比较复杂，仅做简单提示
matches=$(grep -rn 'return .*, err$' internal/service/ --include='*.go' \
    | grep -v 'nil$' \
    | grep -v '// ' \
    2>/dev/null || true)
if [[ -n "$matches" ]]; then
    warn "Service 层通常应 return rsp, nil（业务错误走 rsp.Error），请确认是否正确:"
    echo "$matches" | head -10
fi

# ─────────────────────────────────────────────
# 汇总
# ─────────────────────────────────────────────
echo ""
echo -e "${CYAN}━━━ Summary ━━━${NC}"
if [[ $ERRORS -eq 0 && $WARNINGS -eq 0 ]]; then
    echo -e "${GREEN}✅ All convention checks passed!${NC}"
    exit 0
elif [[ $ERRORS -eq 0 ]]; then
    echo -e "${YELLOW}⚠️  ${WARNINGS} warning(s), 0 error(s)${NC}"
    exit 0
else
    echo -e "${RED}❌ ${ERRORS} error(s), ${WARNINGS} warning(s)${NC}"
    exit 1
fi
