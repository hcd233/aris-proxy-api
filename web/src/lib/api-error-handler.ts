/**
 * api-error-handler.ts — 统一 API 错误处理的工具函数和 React hook。
 *
 * 用法概述:
 * ```ts
 * // 纯函数式（无 UI 依赖）
 * const parsed = parseApiError(err);
 *
 * // toast 快捷方式
 * showErrorToast(err);
 * showErrorToast(err, { title: "保存失败", action: { label: "重试", onClick: retry } });
 * ```
 */
import { toast } from "sonner";
import { parseError, type StructuredError, BusinessErrorCode, type ErrorSeverity } from "./api-errors";

// ── 暴露 parseError 作为主入口 ──────────────────────────────────────────────

export { parseError, BusinessErrorCode };
export type { StructuredError, ErrorSeverity };

// ── 错误消息本地化前缀映射 ──────────────────────────────────────────────────

/**
 * 业务错误码 → i18n key 前缀。
 * 用于组件内通过 `t()` 显示本地化后的错误描述。
 */
export const ERROR_I18N_KEY: Partial<Record<BusinessErrorCode, string>> = {
  [BusinessErrorCode.InvalidArgument]: "error.invalid_argument",
  [BusinessErrorCode.NotFound]: "error.not_found",
  [BusinessErrorCode.AlreadyExists]: "error.already_exists",
  [BusinessErrorCode.PermissionDenied]: "error.permission_denied",
  [BusinessErrorCode.RateLimitExceeded]: "error.rate_limit",
  [BusinessErrorCode.Internal]: "error.internal",
};

// ── Toast 快捷方式 ──────────────────────────────────────────────────────────

export interface ErrorToastOptions {
  /** 覆盖错误标题（显示在 message 上方） */
  title?: string;
  /** 覆盖/补充错误描述，不传则从 err 对象中提取 */
  description?: string;
  /** 持续时间（毫秒），默认 5000 */
  duration?: number;
  /** 操作按钮 */
  action?: {
    label: string;
    onClick: () => void;
  };
}

/**
 * 将任意错误以统一样式显示为 toast。
 * 会自动根据错误严重级别选择图标和样式。
 */
export function showErrorToast(err: unknown, opts?: ErrorToastOptions): StructuredError {
  const parsed = parseError(err);
  const description = opts?.description ?? parsed.message;
  const duration = opts?.duration ?? toastDuration(parsed.severity);

  if (opts?.title) {
    toast.error(opts.title, {
      description,
      duration,
      action: opts.action,
    });
  } else {
    toast.error(description, {
      duration,
      action: opts?.action,
    });
  }

  return parsed;
}

/** 根据严重级别返回合适的 toast 显示时长 */
function toastDuration(severity: ErrorSeverity): number {
  switch (severity) {
    case "critical":
      return 10_000;
    case "error":
      return 6_000;
    case "warning":
      return 4_000;
    case "info":
      return 3_000;
  }
}

// ── 工具函数 ──────────────────────────────────────────────────────────────────

/** 是否为可重试的间歇性错误（网络断开、5xx 等） */
export function isRetryable(err: StructuredError): boolean {
  if (err.httpStatus === undefined) return false;
  return err.httpStatus >= 500 || err.httpStatus === 429 || err.httpStatus === 0;
}

/** 业务错误码是否表示 "资源不存在" */
export function isNotFound(err: StructuredError): boolean {
  return err.code === BusinessErrorCode.NotFound || err.httpStatus === 404;
}

/** 业务错误码是否表示 "权限不足" */
export function isPermissionDenied(err: StructuredError): boolean {
  return err.code === BusinessErrorCode.PermissionDenied || err.httpStatus === 403;
}
