"use client";

import type { ReactNode } from "react";
import { RotateCw } from "lucide-react";
import { useT } from "@/lib/i18n";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";

export interface PanelProps {
  title: ReactNode;
  action?: ReactNode;
  footer?: ReactNode;
  loading?: boolean;
  error?: boolean;
  empty?: boolean;
  onRetry?: () => void;
  className?: string;
  bodyClassName?: string;
  children?: ReactNode;
}

/**
 * Dashboard 面板外壳：紧凑标题行 + 自适应高度主体（min-h-0，供图表 100% 撑满）。
 * 首次加载显示骨架；后续刷新保留旧数据（避免整屏闪烁），失败时给重试入口。
 */
export function Panel({
  title,
  action,
  footer,
  loading,
  error,
  empty,
  onRetry,
  className,
  bodyClassName,
  children,
}: PanelProps) {
  const t = useT();
  const showSkeleton = loading && empty;
  return (
    <Card size="sm" className={cn("min-h-0 gap-2", className)}>
      <div className="flex min-h-7 items-center justify-between gap-2 px-3">
        <h2 className="text-[11px] leading-tight font-medium tracking-wider text-muted-foreground uppercase">
          {title}
        </h2>
        {action}
      </div>
      <div className={cn("relative min-h-0 flex-1 px-3", bodyClassName)}>
        {showSkeleton ? (
          <Skeleton className="h-full min-h-24 w-full" />
        ) : error && empty ? (
          <div className="flex h-full min-h-24 flex-col items-center justify-center gap-2 text-xs text-muted-foreground">
            <span>{t("charts.failed_to_load")}</span>
            {onRetry && (
              <Button variant="outline" size="xs" onClick={onRetry}>
                <RotateCw />
                {t("charts.retry")}
              </Button>
            )}
          </div>
        ) : empty ? (
          <div className="flex h-full min-h-24 items-center justify-center text-xs text-muted-foreground">
            {t("dashboard.no_data")}
          </div>
        ) : (
          children
        )}
      </div>
      {footer && !showSkeleton && (
        <div className="px-3 text-[11px] text-muted-foreground">{footer}</div>
      )}
    </Card>
  );
}
