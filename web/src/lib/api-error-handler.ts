/**
 * api-error-handler.ts — 统一 API 错误处理的工具函数和 React hook。
 *
 * 用法概述:
 * ```ts
 * // 纯函数式（无 UI 依赖）
 * const parsed = parseError(err);
 *
 * // toast 快捷方式
 * showErrorToast(err);
 * showErrorToast(err, { title: "保存失败", action: { label: "重试", onClick: retry } });
 * ```
 */
import { toast } from "sonner";
import {
  parseError,
  type StructuredError,
  BusinessErrorCode,
  type ErrorSeverity,
} from "./api-errors";
import { translate } from "./i18n";

// ── 暴露 parseError 作为主入口 ──────────────────────────────────────────────

export { parseError, BusinessErrorCode };
export type { StructuredError, ErrorSeverity };

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
  // 优先用 i18n key 本地化描述；无 key（如自定义 Error/string）时退回 parsed.message
  const description =
    opts?.description ??
    (parsed.messageKey ? translate(parsed.messageKey, parsed.message) : parsed.message);
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


