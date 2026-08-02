/**
 * 前后端共享的业务错误码常量。
 *
 * 后端定义在 `internal/common/ierr/sentinels.go`，前端在此处镜像。
 * 新增/修改业务错误码时必须同步更新两侧。
 *
 * ── 错误码分段规则 ──
 * 10001–10099  认证／授权错误
 * 10100–10199  通用业务错误
 * 20000–29999  后端保留（暂未定）
 */

export const BusinessErrorCode = {
  /** ─── 10001–10099: 认证／授权 ─── */

  /** 未授权（token 无效、缺失或过期） */
  Unauthorized: 10001,

  /** ─── 10100–10199: 通用业务 ─── */

  /** 请求参数校验失败 */
  InvalidArgument: 10100,

  /** 资源不存在 */
  NotFound: 10101,

  /** 资源已存在（创建重复条目） */
  AlreadyExists: 10102,

  /** 操作被拒绝（权限不足或违反业务规则） */
  PermissionDenied: 10103,

  /** 速率限制 */
  RateLimitExceeded: 10104,

  /** 内部服务错误 */
  Internal: 10199,
} as const;

export type BusinessErrorCode = (typeof BusinessErrorCode)[keyof typeof BusinessErrorCode];

// ── 错误严重级别 ──────────────────────────────────────────────────────────────

export type ErrorSeverity = "critical" | "error" | "warning" | "info";

// ── 结构化错误 ────────────────────────────────────────────────────────────────

export interface StructuredError {
  /** 业务错误码，0 表示非业务层错误（如网络断开、HTTP 5xx 等） */
  code: number;
  /** 用户可读的错误描述（兜底文本，可能未本地化） */
  message: string;
  /** 严重级别 */
  severity: ErrorSeverity;
  /** HTTP 状态码（仅网络层错误时有值） */
  httpStatus?: number;
  /** 可本地化展示的 i18n key（若存在）；消费方应优先用 translate(key) */
  messageKey?: string;
  /** 后端返回的原始 body 文本 */
  rawBody?: string;
  /** 原始 Error 对象 */
  raw?: unknown;
}

// ── 错误码 → 严重级别映射 ────────────────────────────────────────────────────

const ERROR_CODE_SEVERITY: Partial<Record<BusinessErrorCode, ErrorSeverity>> = {
  [BusinessErrorCode.Unauthorized]: "critical",
  [BusinessErrorCode.InvalidArgument]: "warning",
  [BusinessErrorCode.NotFound]: "warning",
  [BusinessErrorCode.AlreadyExists]: "warning",
  [BusinessErrorCode.PermissionDenied]: "error",
  [BusinessErrorCode.RateLimitExceeded]: "warning",
  [BusinessErrorCode.Internal]: "error",
};

/** HTTP 状态码 → 严重级别 */
function httpStatusSeverity(status: number): ErrorSeverity {
  if (status >= 500) return "critical";
  if (status >= 400) return "error";
  return "warning";
}

// ── 业务错误码 / HTTP 状态码 → i18n key 映射 ────────────────────────────────
// 键名与 locales/{en,zh,ja}.json 中的扁平 key 对齐（error.* 段）。

export const ERROR_I18N_KEY: Partial<Record<BusinessErrorCode, string>> = {
  [BusinessErrorCode.Unauthorized]: "error.unauthorized",
  [BusinessErrorCode.InvalidArgument]: "error.bad_request",
  [BusinessErrorCode.NotFound]: "error.data_not_exists",
  [BusinessErrorCode.AlreadyExists]: "error.data_exists",
  [BusinessErrorCode.PermissionDenied]: "error.no_permission",
  [BusinessErrorCode.RateLimitExceeded]: "error.too_many_requests",
  [BusinessErrorCode.Internal]: "error.internal",
};

/** HTTP 状态码兜底消息：message 为未本地化兜底，key 供消费方本地化。 */
const HTTP_FALLBACK: Record<number, { message: string; key: string }> = {
  400: { message: "请求参数有误", key: "error.http_400" },
  404: { message: "请求的资源不存在", key: "error.http_404" },
  409: { message: "资源冲突", key: "error.http_409" },
  413: { message: "请求内容过大", key: "error.http_413" },
  429: { message: "请求过于频繁，请稍后再试", key: "error.http_429" },
  500: { message: "服务器内部错误", key: "error.http_500" },
  502: { message: "网关错误", key: "error.http_502" },
  503: { message: "服务暂不可用", key: "error.http_503" },
  504: { message: "网关超时", key: "error.http_504" },
};

// ── 解析函数 ──────────────────────────────────────────────────────────────────

/**
 * 将任何形式的错误（ApiError、Error、string、unknown）统一解析为 StructuredError。
 * 此函数是纯函数，无副作用。
 */
export function parseError(err: unknown): StructuredError {
  if (err && typeof err === "object" && "error" in err) {
    // 后端统一错误响应结构：顶层 { error: { code, message } }
    const biz = (err as { error?: { code?: number; message?: string } }).error;
    if (biz && typeof biz === "object" && biz.code !== undefined && biz.message !== undefined) {
      return {
        code: biz.code,
        message: biz.message,
        severity: ERROR_CODE_SEVERITY[biz.code as BusinessErrorCode] ?? "error",
        raw: err,
      };
    }
  }

  if (err && typeof err === "object" && "code" in err && "message" in err) {
    // 后端返回的业务错误 ({ code, message })
    const biz = err as { code: number; message: string };
    return {
      code: biz.code,
      message: biz.message,
      severity: ERROR_CODE_SEVERITY[biz.code as BusinessErrorCode] ?? "error",
      messageKey: ERROR_I18N_KEY[biz.code as BusinessErrorCode],
    };
  }

  if (err && typeof err === "object" && "name" in err && (err as Error).name === "ApiError") {
    // ApiError（HTTP 层错误）
    const apiErr = err as unknown as { status: number; body: string; message: string };
    let bodyObj: { code?: number; message?: string } | null = null;
    try {
      bodyObj = JSON.parse(apiErr.body);
    } catch {
      // body 不是 JSON，使用原始文本
    }

    if (bodyObj?.code && bodyObj?.message) {
      return {
        code: bodyObj.code,
        message: bodyObj.message,
        severity:
          ERROR_CODE_SEVERITY[bodyObj.code as BusinessErrorCode] ??
          httpStatusSeverity(apiErr.status),
        messageKey: ERROR_I18N_KEY[bodyObj.code as BusinessErrorCode],
        httpStatus: apiErr.status,
        rawBody: apiErr.body,
      };
    }

    const fallback = HTTP_FALLBACK[apiErr.status];
    return {
      code: 0,
      message: fallback?.message ?? apiErr.message,
      severity: httpStatusSeverity(apiErr.status),
      messageKey: fallback?.key,
      httpStatus: apiErr.status,
      rawBody: apiErr.body,
    };
  }

  if (err instanceof TypeError && err.message === "Failed to fetch") {
    return {
      code: 0,
      message: "网络连接失败，请检查网络后重试",
      severity: "critical",
      messageKey: "error.network_failed",
      raw: err,
    };
  }

  if (err instanceof Error) {
    return {
      code: 0,
      message: err.message,
      severity: "error",
      raw: err,
    };
  }

  if (typeof err === "string") {
    return { code: 0, message: err, severity: "error", raw: err };
  }

  return {
    code: 0,
    message: "发生未知错误",
    severity: "error",
    messageKey: "error.unknown",
    raw: err,
  };
}
