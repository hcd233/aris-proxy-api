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
# 2. 常量定义规范
# ─────────────────────────────────────────────
section "Constant Definition"

# 2.1 禁止定义"转发封装常量"：即 const X = pkg.Y 或 const X = pkg.C 形式
# 这类定义只是给另一个包的具名常量起别名，毫无意义，直接使用原始常量即可
# 例：const HTTPStatusOK = fiber.StatusOK  ← 禁止
#     fiber.StatusOK                        ← 正确，直接使用
# 排除：
#   - 测试文件
#   - iota 枚举（不含包路径的纯 iota 表达式）
matches=$(grep -rn --include='*.go' \
    -E '^\s+[A-Za-z][A-Za-z0-9_]*\s*=\s*[a-zA-Z][a-zA-Z0-9_]*\.[A-Za-z][A-Za-z0-9_]*$' \
    internal/common/constant/ internal/enum/ internal/common/enum/ 2>/dev/null \
    | grep -v '_test.go' \
    | grep -v '// ' \
    || true)
if [[ -n "$matches" ]]; then
    error "禁止在 constant/enum 中定义转发封装常量（const X = pkg.Y），直接使用原始常量:"
    echo "$matches" | head -20
fi

# ─────────────────────────────────────────────
# 3. 日志规范
# ─────────────────────────────────────────────
section "Logging"

# 3.1 日志消息必须使用 [ModuleName] 前缀格式
# 匹配 logger.Info/Error/Warn/Debug("xxx 但不以 [ 开头的消息
matches=$(grep -rn -E 'logger\.(Info|Error|Warn|Debug)\(' internal/ --include='*.go' \
    | grep -v '\[' 2>/dev/null || true)
# 过滤掉只有变量引用（无字符串字面量）的行
matches=$(echo "$matches" | grep '"' | grep -v '"\[' 2>/dev/null || true)
if [[ -n "$matches" ]]; then
    warn "日志消息应使用 [ModuleName] 前缀格式，如 logger.Info(\"[XxxService] ...\"):"
    echo "$matches" | head -20
fi

# 3.2 日志中禁止裸记录敏感信息（检查常见敏感字段名未用 MaskSecret）
matches=$(grep -rn -E 'zap\.String.*(Key|Token|Secret|Password)' internal/ --include='*.go' \
    | grep -v 'MaskSecret' \
    | grep -v 'CtxKey' \
    | grep -v 'apiKeyName\|APIKeyName\|keyName' \
    | grep -v 'lockKey\|cacheKey\|configKey\|routeKey\|sortKey' \
    | grep -v 'tokenType\|TokenType\|tokenExpir' \
    | grep -v 'sessionAPIKeyName' \
    2>/dev/null || true)
if [[ -n "$matches" ]]; then
    warn "日志中记录敏感信息（Key/Token/Secret/Password）应使用 util.MaskSecret():"
    echo "$matches" | head -10
fi

# ─────────────────────────────────────────────
# 4. JSON 库规范
# ─────────────────────────────────────────────
section "JSON Library"

# 4.1 禁止使用 encoding/json
matches=$(grep -rn '"encoding/json"' internal/ test/ --include='*.go' 2>/dev/null || true)
if [[ -n "$matches" ]]; then
    error "禁止使用 encoding/json，统一使用 github.com/bytedance/sonic:"
    echo "$matches" | head -20
fi

# 4.2 禁止使用 json.RawMessage
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

# 9.3 接口逻辑层禁止使用 context.Background()/context.TODO()
# 这些层应从上层传递 context，而非自行创建空 context
# 排除: cron/（定时任务需要创建根 context）、infrastructure/（基础设施初始化）、util/（工具函数）、agent/（后台异步任务）
matches=$(grep -rn -E 'context\.(Background|TODO)\(\)' \
    internal/handler/ internal/service/ internal/middleware/ internal/router/ internal/proxy/ internal/converter/ internal/dto/ \
    --include='*.go' 2>/dev/null || true)
if [[ -n "$matches" ]]; then
    error "接口逻辑层禁止使用 context.Background()/context.TODO()，应从上层传递 context:"
    echo "$matches" | head -20
fi

# 9.4 DTO 层禁止依赖数据库模型层（dto 包禁止导入 infrastructure/database/model）
matches=$(grep -rn '"github.com/hcd233/aris-proxy-api/internal/infrastructure/database/model"' \
    internal/dto/ --include='*.go' 2>/dev/null || true)
if [[ -n "$matches" ]]; then
    error "DTO 层禁止依赖数据库模型层（internal/dto/ 禁止导入 infrastructure/database/model），DTO 函数需要 dbmodel 字段时应将具体字段作为参数传入:"
    echo "$matches" | head -20
fi

# 9.5 禁止透传封装函数（exported 方法仅 1:1 委托调用自身 receiver 的另一个方法）
# 检测模式：
#   func (recv) ExportedMethod(args...) ... {
#       return recv.anotherMethod(args...)
#   }
# 排除：
#   - handler/ 层（接口实现 + 分层架构设计，薄包装是预期行为）
#   - 工厂函数 NewXxx（返回 &struct{...} 初始化）
#   - init() 函数
#   - singleton getter（return someVar，无函数调用）
#   - 调用 receiver 字段的方法（如 recv.field.Method()，属于适配而非纯透传）
passthrough_matches=$(awk '
/^func / {
    if ($0 ~ /^func init\(/) next

    func_line = $0
    func_file = FILENAME
    func_lineno = FNR
    in_func = 1
    brace_depth = 0
    body_lines = 0
    first_body_line = ""

    # 提取 receiver 变量名（如有）: func (pm *PoolManager) → pm
    receiver = ""
    if ($0 ~ /^func \(/) {
        tmp = $0
        sub(/^func \(/, "", tmp)
        sub(/ .*/, "", tmp)
        receiver = tmp
    }

    n = split($0, chars, "")
    for (i = 1; i <= n; i++) {
        if (chars[i] == "{") brace_depth++
        if (chars[i] == "}") brace_depth--
    }
    next
}

in_func {
    n = split($0, chars, "")
    for (i = 1; i <= n; i++) {
        if (chars[i] == "{") brace_depth++
        if (chars[i] == "}") brace_depth--
    }

    trimmed = $0
    gsub(/^[[:space:]]+/, "", trimmed)
    gsub(/[[:space:]]+$/, "", trimmed)
    if (trimmed != "" && trimmed != "{" && trimmed != "}") {
        body_lines++
        if (body_lines == 1) first_body_line = trimmed
    }

    if (brace_depth == 0) {
        in_func = 0
        if (body_lines == 1 && first_body_line ~ /^return [a-zA-Z]/) {
            # 排除 singleton getter（无括号调用）
            if (first_body_line !~ /\(/) next
            # 排除工厂函数
            if (first_body_line ~ /return &/) next

            # 仅报告: 有 receiver 且 body 直接调用 receiver.method()
            # 排除 receiver.field.method() 形式（属于字段访问适配，非纯透传）
            if (receiver != "") {
                call_target = first_body_line
                sub(/^return /, "", call_target)
                sub(/\(.*/, "", call_target)
                expected_prefix = receiver "."
                if (index(call_target, expected_prefix) == 1) {
                    rest = substr(call_target, length(expected_prefix) + 1)
                    if (index(rest, ".") == 0 && rest != "") {
                        print func_file ":" func_lineno ": " func_line
                        print func_file ":" func_lineno+1 ":   " first_body_line
                    }
                }
            }
        }
    }
}
' $(find internal/ -name '*.go' -not -path '*/handler/*' 2>/dev/null) 2>/dev/null || true)

if [[ -n "$passthrough_matches" ]]; then
    warn "发现透传封装函数（exported 方法仅透传调用 receiver 的另一个方法），应将逻辑内联或合并方法:"
    echo "$passthrough_matches" | head -20
fi

# ─────────────────────────────────────────────
# 10. 魔法数字 & 魔法字符串
# ─────────────────────────────────────────────
section "Magic Values"

# 白名单排除路径：以下包允许定义字面量，不参与魔法值扫描
# 允许原因说明：
#   - constant/enum/ierr/     —— 这些本来就是定义常量的地方
#   - common/model/           —— 业务错误模型定义
#   - infrastructure/         —— 基础设施层（DB/Redis/对象存储），允许直接使用配置值
#   - config/                 —— 环境变量默认值配置，允许使用字面量
#   - logger/                 —— 日志框架初始化，log level/encoder key 属于框架配置
MAGIC_EXCLUDE_PATHS=(
    "internal/common/constant/"
    "internal/common/enum/"
    "internal/common/ierr/"
    "internal/common/model/"
    "internal/enum/"
    "internal/config/"
    "internal/router/"
)

# 构造路径排除的 grep -v 管道
magic_path_filter() {
    local input="$1"
    local result="$input"
    for exclude_path in "${MAGIC_EXCLUDE_PATHS[@]}"; do
        result=$(echo "$result" | grep -v "^$exclude_path" 2>/dev/null || true)
    done
    echo "$result"
}

# ── 10.1 魔法数字 ──────────────────────────────
# 检测：白名单排除后的目录内出现 >= 3 的裸整数字面量
# 语法层过滤（不属于业务豁免，只是减少 Go 语法噪音）：
#   - 纯注释行（// 开头）
#   - struct tag 行（含反引号 `，数字在 tag 内无业务语义）
#   - import 块行
#   - 包内 const 声明行（const X = N 是合法常量定义本身）
#   - logger 调用行中字符串内出现的数字（如 "non-200 status"）
# 其余所有数字字面量均应报告

magic_number_matches=$(grep -rn --include='*.go' \
    -E '[^a-zA-Z0-9_.][3-9][0-9][0-9]+[^a-zA-Z0-9_.]|[^a-zA-Z0-9_.][1-9][0-9][0-9]+[^a-zA-Z0-9_.]|[^a-zA-Z0-9_.][3-9][0-9][^a-zA-Z0-9_.]' \
    internal/ 2>/dev/null || true)

magic_number_matches=$(magic_path_filter "$magic_number_matches")

magic_number_matches=$(echo "$magic_number_matches" \
    | grep -v '^\s*//' \
    | grep -v '`' \
    | grep -v 'import' \
    | grep -v '^\s*const ' \
    | grep -v '\sconst\s' \
    | grep -v 'logger\.' \
    | grep -v '\.go:[0-9]*:\s*//' \
    | grep -v '^$' \
    || true)

if [[ -n "$magic_number_matches" ]]; then
    error "发现魔法数字，应提取为具名常量（constant/ 或包内 const 块）:"
    echo "$magic_number_matches" | head -30
fi

# ── 10.1b 魔法 duration 乘数（N * time.*）───────
# 检测：白名单目录外出现「整数字面量 * time.」形式的 duration 构造（如 5 * time.Minute、30 * time.Second）
# 说明：10.1 主要覆盖三位数等；小整数作 time 乘数需单独规则，否则易漏报
magic_duration_matches=$(grep -rn --include='*.go' \
    -E '\b[0-9]+\s*\*\s*time\.' \
    internal/ cmd/ 2>/dev/null || true)

magic_duration_matches=$(magic_path_filter "$magic_duration_matches")

magic_duration_matches=$(echo "$magic_duration_matches" \
    | grep -v '^\s*//' \
    | grep -v '\`' \
    | grep -v 'import' \
    | grep -v '^\s*const ' \
    | grep -v '\sconst\s' \
    | grep -v 'logger\.' \
    | grep -v '\.go:[0-9]*:\s*//' \
    | grep -v '^$' \
    || true)

if [[ -n "$magic_duration_matches" ]]; then
    error "发现魔法 duration 乘数（N * time.*），应提取为具名常量（constant/time.go 等）:"
    echo "$magic_duration_matches" | head -30
fi

# ── 10.2 魔法字符串 ────────────────────────────
# 10.2a 检测：白名单排除后的 internal/ 内在赋值/return/case/比较 语句中出现长度 >= 2 的裸字符串字面量
#      （仅扫描 internal/，避免 cmd/ 中 cobra 默认值等噪声）
# 10.2b 检测：复合字面量中独占一行、以 / 开头的路径字符串（如 []string{ "/x", }），
#      此类行无法被 10.2a 匹配；扫描 internal/ 与 cmd/
# 语法层过滤：
#   - 纯注释行
#   - struct tag 行（含反引号 `）
#   - import 块行
#   - 包内 const 声明行
#   - logger 调用行（日志消息字面量是可接受的）
#   - router.go 中的 HTML 内联模板行

magic_string_matches=$(grep -rn --include='*.go' \
    -E '(=|:=|return|case|\!=|==)[[:space:]]*"[^"]{2,}"' \
    internal/ 2>/dev/null || true)

magic_string_path_elems=$(grep -rn --include='*.go' \
    -E '^[[:space:]]*"/[^"]+",[[:space:]]*$' \
    internal/ cmd/ 2>/dev/null || true)

# 10.2c 检测：键值对形式的路径字面量（如 huma Operation 的 Path: "/health"），10.2a 无法匹配 Field: 语法
magic_string_struct_kv=$(grep -rn --include='*.go' \
    -E '[A-Za-z_][A-Za-z0-9_]*:\s*"/[^"]+"' \
    internal/ cmd/ 2>/dev/null || true)

magic_string_matches=$(printf '%s\n%s\n%s' "$magic_string_matches" "$magic_string_path_elems" "$magic_string_struct_kv")

magic_string_matches=$(magic_path_filter "$magic_string_matches")

magic_string_matches=$(echo "$magic_string_matches" \
    | grep -v '^\s*//' \
    | grep -v '`' \
    | grep -v 'import' \
    | grep -v '^\s*const ' \
    | grep -v '\sconst\s' \
    | grep -v 'logger\.' \
    | grep -v 'internal/router/router\.go' \
    | grep -v '\.go:[0-9]*:\s*//' \
    | grep -v '^$' \
    || true)

if [[ -n "$magic_string_matches" ]]; then
    error "发现魔法字符串，应提取为具名常量（constant/string.go 或包内 const 块）:"
    echo "$magic_string_matches" | head -30
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
