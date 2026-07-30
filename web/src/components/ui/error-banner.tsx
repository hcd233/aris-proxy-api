/**
 * error-banner.tsx — 内联错误横幅组件。
 *
 * 用于在页面/卡片/表单顶部展示非阻断性错误，支持:
 * - 左 accent 边框 + 斜线装饰（工业风区分度）
 * - 严重级别着色（critical / error / warning / info）
 * - 折叠/展开（紧凑模式）
 * - 重试按钮 + 自定义操作
 * - 入口滑入动画
 *
 * 用法:
 * ```tsx
 * <ErrorBanner
 *   severity="error"
 *   message="加载会话列表失败"
 *   onRetry={() => fetchData()}
 * />
 * ```
 */
"use client";

import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";
import {
  OctagonXIcon,
  TriangleAlertIcon,
  InfoIcon,
  RefreshCwIcon,
  XIcon,
  ChevronDownIcon,
} from "lucide-react";
import { Button } from "@/components/ui/button";

// ── 类型 ──────────────────────────────────────────────────────────────────────

export type BannerSeverity = "critical" | "error" | "warning" | "info";

export interface ErrorBannerProps {
  /** 严重级别（默认 error） */
  severity?: BannerSeverity;
  /** 错误标题（粗体显示在 description 上方） */
  title?: string;
  /** 错误描述 */
  message: string;
  /** 详细错误信息（前端调试用，可通过折叠展开） */
  detail?: string;
  /** 重试回调 */
  onRetry?: () => void;
  /** 自定义操作按钮配置 */
  actions?: { label: string; onClick: () => void; variant?: "default" | "outline" | "ghost" }[];
  /** 是否允许关闭 */
  dismissible?: boolean;
  /** 关闭回调 */
  onDismiss?: () => void;
  /** 紧凑模式（少 padding、小字号） */
  compact?: boolean;
  /** 额外的 className */
  className?: string;
  /** 子元素（放在 banner 底部，如堆栈详情） */
  children?: ReactNode;
}

// ── 严重级别 → 视觉映射 ──────────────────────────────────────────────────────

interface SeverityStyle {
  borderColor: string;
  bgColor: string;
  iconColor: string;
  textColor: string;
  stripeColor: string;
}

const SEVERITY_STYLES: Record<BannerSeverity, SeverityStyle> = {
  critical: {
    borderColor: "border-red-600",
    bgColor: "bg-red-50 dark:bg-red-950/30",
    iconColor: "text-red-600 dark:text-red-400",
    textColor: "text-red-800 dark:text-red-300",
    stripeColor: "bg-red-200/40 dark:bg-red-800/30",
  },
  error: {
    borderColor: "border-red-500",
    bgColor: "bg-red-50/60 dark:bg-red-950/20",
    iconColor: "text-red-500 dark:text-red-400",
    textColor: "text-red-700 dark:text-red-300",
    stripeColor: "bg-red-200/30 dark:bg-red-800/20",
  },
  warning: {
    borderColor: "border-amber-500",
    bgColor: "bg-amber-50/60 dark:bg-amber-950/20",
    iconColor: "text-amber-600 dark:text-amber-400",
    textColor: "text-amber-800 dark:text-amber-300",
    stripeColor: "bg-amber-200/30 dark:bg-amber-800/20",
  },
  info: {
    borderColor: "border-blue-500",
    bgColor: "bg-blue-50/60 dark:bg-blue-950/20",
    iconColor: "text-blue-600 dark:text-blue-400",
    textColor: "text-blue-800 dark:text-blue-300",
    stripeColor: "bg-blue-200/30 dark:bg-blue-800/20",
  },
};

const ICONS: Record<BannerSeverity, ReactNode> = {
  critical: <OctagonXIcon className="size-5" />,
  error: <OctagonXIcon className="size-5" />,
  warning: <TriangleAlertIcon className="size-5" />,
  info: <InfoIcon className="size-5" />,
};

// ── 组件 ──────────────────────────────────────────────────────────────────────

export function ErrorBanner({
  severity = "error",
  title,
  message,
  detail,
  onRetry,
  actions,
  dismissible = true,
  onDismiss,
  compact = false,
  className = "",
  children,
}: ErrorBannerProps) {
  const [visible, setVisible] = useState(true);
  const [expanded, setExpanded] = useState(false);
  const bannerRef = useRef<HTMLDivElement>(null);
  const style = SEVERITY_STYLES[severity];

  // 入口动画
  useEffect(() => {
    const el = bannerRef.current;
    if (!el) return;
    requestAnimationFrame(() => {
      el?.classList.remove("opacity-0", "-translate-y-2");
    });
  }, []);

  const handleDismiss = useCallback(() => {
    setVisible(false);
    onDismiss?.();
  }, [onDismiss]);

  if (!visible) return null;

  const mergedActions = [
    ...(onRetry
      ? [
          {
            label: "重试",
            onClick: onRetry,
            variant: "outline" as const,
          },
        ]
      : []),
    ...(actions ?? []),
  ];

  return (
    <div
      ref={bannerRef}
      role="alert"
      className={[
        "relative overflow-hidden rounded-lg border-l-4 opacity-0 -translate-y-2 transition-all duration-300 ease-out motion-reduce:transition-none motion-reduce:opacity-100 motion-reduce:translate-y-0",
        style.borderColor,
        style.bgColor,
        compact ? "p-3" : "p-4",
        className,
      ].join(" ")}
    >
      {/* 斜线装饰条纹 — 工业风视觉区分 */}
      <div
        aria-hidden
        className={[
          "pointer-events-none absolute inset-0",
          "bg-[repeating-linear-gradient(135deg,transparent,transparent_8px,var(--stripe-color)_8px,var(--stripe-color)_9px)]",
        ].join(" ")}
        style={
          { "--stripe-color": style.stripeColor } as React.CSSProperties
        }
      />

      <div className="relative flex items-start gap-3">
        {/* Icon */}
        <span className={["mt-0.5 shrink-0", style.iconColor].join(" ")}>
          {ICONS[severity]}
        </span>

        {/* Content */}
        <div className="min-w-0 flex-1">
          {title && (
            <p className={["text-sm font-semibold", style.textColor].join(" ")}>
              {title}
            </p>
          )}
          <p
            className={[
              "text-sm",
              title ? "mt-1" : "",
              style.textColor,
            ].join(" ")}
          >
            {message}
          </p>

          {/* 详细错误（可折叠） */}
          {detail && (
            <>
              <button
                type="button"
                onClick={() => setExpanded(!expanded)}
                className={[
                  "mt-1.5 inline-flex items-center gap-1 text-xs underline-offset-2 hover:underline",
                  style.iconColor,
                ].join(" ")}
              >
                <ChevronDownIcon
                  className={[
                    "size-3 transition-transform duration-200",
                    expanded ? "rotate-180" : "",
                  ].join(" ")}
                />
                {expanded ? "收起详情" : "查看详情"}
              </button>
              {expanded && (
                <pre className="mt-2 overflow-x-auto rounded bg-black/5 p-2 text-xs text-muted-foreground dark:bg-white/5">
                  {detail}
                </pre>
              )}
            </>
          )}

          {/* 操作按钮 */}
          {mergedActions.length > 0 && (
            <div className="mt-3 flex flex-wrap items-center gap-2">
              {mergedActions.map((action) => (
                <Button
                  key={action.label}
                  variant={action.variant}
                  size="sm"
                  onClick={action.onClick}
                >
                  {action.label === "重试" && (
                    <RefreshCwIcon className="mr-1 size-3.5" />
                  )}
                  {action.label}
                </Button>
              ))}
            </div>
          )}

          {children}
        </div>

        {/* 关闭按钮 */}
        {dismissible && (
          <button
            type="button"
            onClick={handleDismiss}
            className={[
              "shrink-0 rounded-sm p-1 opacity-60 transition-opacity hover:opacity-100",
              style.iconColor,
            ].join(" ")}
            aria-label="关闭"
          >
            <XIcon className="size-4" />
          </button>
        )}
      </div>
    </div>
  );
}
