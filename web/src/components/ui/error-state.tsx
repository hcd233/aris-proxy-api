/**
 * error-state.tsx — 数据加载失败时的全状态组件。
 *
 * 用于替代页面/卡片中数据加载失败时的空白/骨架屏，包含:
 * - 视觉化的错误图标（根据严重级别不同）
 * - 错误描述
 * - 重试按钮
 * - 可折叠的技术详情
 *
 * 用法:
 * ```tsx
 * // 加载中 → 骨架屏
 * if (loading) return <Skeleton className="h-40" />;
 *
 * // 错误状态
 * if (error) return <ErrorState error={error} onRetry={fetchData} />;
 *
 * // 空状态
 * if (!data) return <EmptyState icon={<Inbox />} message="暂无数据" />;
 *
 * // 正常渲染
 * return <DataTable data={data} />;
 * ```
 */
"use client";

import { type ReactNode, useState } from "react";
import {
  OctagonXIcon,
  TriangleAlertIcon,
  WifiOffIcon,
  RefreshCwIcon,
  ChevronDownIcon,
  InboxIcon,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { parseError, type StructuredError } from "@/lib/api-error-handler";

// ── ErrorState ────────────────────────────────────────────────────────────────

export interface ErrorStateProps {
  /** 原始错误对象（任意类型） */
  error?: unknown;
  /** 或直接传入解析后的结构（二者选其一） */
  structured?: StructuredError;
  /** 覆盖标题 */
  title?: string;
  /** 覆盖描述 */
  message?: string;
  /** 重试回调 */
  onRetry?: () => void;
  /** 额外的 className */
  className?: string;
  /** 紧凑模式 */
  compact?: boolean;
}

export function ErrorState({
  error,
  structured: explicit,
  title,
  message,
  onRetry,
  className = "",
  compact = false,
}: ErrorStateProps) {
  const [showDetail, setShowDetail] = useState(false);

  // 优先使用显式传入的 structured，否则从 raw error 解析
  const st = explicit ?? (error ? parseError(error) : null);
  const displayTitle = title ?? "数据加载失败";
  const displayMessage = message ?? st?.message ?? "请稍后重试";

  const isNetworkError = st?.httpStatus === 0 || st?.code === 0;
  const Icon = isNetworkError ? WifiOffIcon : OctagonXIcon;

  if (compact) {
    return (
      <div
        role="alert"
        className={[
          "flex items-center gap-3 rounded-lg border border-destructive/30 bg-destructive/5 p-4",
          className,
        ].join(" ")}
      >
        <Icon className="size-5 shrink-0 text-destructive/60" />
        <p className="flex-1 text-sm text-destructive-foreground">{displayMessage}</p>
        {onRetry && (
          <Button variant="outline" size="sm" onClick={onRetry}>
            <RefreshCwIcon className="mr-1 size-3.5" />
            重试
          </Button>
        )}
      </div>
    );
  }

  return (
    <div
      role="alert"
      className={[
        "flex flex-col items-center justify-center px-4 py-16 text-center",
        className,
      ].join(" ")}
    >
      {/* Icon */}
      <div className="relative mb-5">
        <Icon className="size-12 text-destructive/50" />
        {isNetworkError && (
          <span className="absolute -bottom-1 -right-1 flex size-5 items-center justify-center rounded-full bg-destructive/10">
            <TriangleAlertIcon className="size-3 text-destructive/60" />
          </span>
        )}
      </div>

      {/* Title */}
      <h3 className="font-display text-lg font-semibold text-foreground">
        {displayTitle}
      </h3>

      {/* Description */}
      <p className="mt-2 max-w-sm text-sm text-muted-foreground">
        {displayMessage}
      </p>

      {/* 技术详情（可折叠） */}
      {st && (st.rawBody || st.httpStatus) && (
        <>
          <button
            type="button"
            onClick={() => setShowDetail(!showDetail)}
            className="mt-3 inline-flex items-center gap-1 text-xs text-muted-foreground underline-offset-2 hover:underline"
          >
            <ChevronDownIcon
              className={`size-3 transition-transform duration-200 ${showDetail ? "rotate-180" : ""}`}
            />
            {showDetail ? "收起详情" : "查看技术详情"}
          </button>
          {showDetail && (
            <pre className="mt-3 max-h-48 w-full max-w-md overflow-auto rounded-lg bg-muted p-3 text-left text-xs text-muted-foreground">
              {st.httpStatus && `HTTP ${st.httpStatus}`}
              {st.rawBody && `\n${st.rawBody}`}
              {st.code !== 0 && `\nErrorCode: ${st.code}`}
            </pre>
          )}
        </>
      )}

      {/* Actions */}
      <div className="mt-6 flex items-center gap-3">
        {onRetry && (
          <Button onClick={onRetry}>
            <RefreshCwIcon className="mr-1.5 size-4" />
            重试
          </Button>
        )}
      </div>
    </div>
  );
}

// ── EmptyState ────────────────────────────────────────────────────────────────

export interface EmptyStateProps {
  /** 图标（推荐 lucide-react 组件） */
  icon?: ReactNode;
  /** 标题 */
  title?: string;
  /** 描述 */
  message?: string;
  /** 操作按钮配置 */
  action?: { label: string; onClick: () => void };
  /** 额外的 className */
  className?: string;
  /** 子元素（放在底部） */
  children?: ReactNode;
}

export function EmptyState({
  icon,
  title,
  message,
  action,
  className = "",
  children,
}: EmptyStateProps) {
  return (
    <div
      className={[
        "flex flex-col items-center justify-center px-4 py-12 text-center",
        className,
      ].join(" ")}
    >
      {icon ? (
        <div className="mb-4 opacity-40">{icon}</div>
      ) : (
        <InboxIcon className="mb-4 size-10 text-muted-foreground/40" />
      )}

      {title && (
        <h3 className="font-display text-base font-semibold text-foreground">{title}</h3>
      )}

      <p className="mt-1.5 max-w-sm text-sm text-muted-foreground">
        {message ?? "暂无数据"}
      </p>

      {action && (
        <Button variant="outline" className="mt-4" onClick={action.onClick}>
          {action.label}
        </Button>
      )}

      {children}
    </div>
  );
}
