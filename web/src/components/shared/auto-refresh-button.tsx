"use client";

import { ChevronDown, RotateCw } from "lucide-react";

import { useT } from "@/lib/i18n";
import { REFRESH_OFF, formatRefreshInterval } from "@/lib/refresh-interval";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/spinner";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

export interface AutoRefreshButtonProps {
  /** 当前间隔，REFRESH_OFF 表示关闭 */
  intervalMs: number;
  options: readonly number[];
  onIntervalChange: (intervalMs: number) => void;
  /** useAutoRefresh 的 cycle：变化即重放倒计时 */
  cycle: number;
  onRefresh: () => void;
  /** 正在拉取：图标转动 */
  fetching?: boolean;
}

/**
 * 页头刷新按钮：主按钮立即刷新，图标外圈是倒计时环（转满即自动刷新）并写着频率；
 * 右侧箭头展开频率菜单，「关闭」与各档位同组单选。
 */
export function AutoRefreshButton({
  intervalMs,
  options,
  onIntervalChange,
  cycle,
  onRefresh,
  fetching = false,
}: AutoRefreshButtonProps) {
  const t = useT();
  const enabled = intervalMs > 0;
  const label = enabled ? formatRefreshInterval(intervalMs) : "";

  return (
    <div role="group" className="inline-flex">
      <Button
        variant="outline"
        size="sm"
        onClick={onRefresh}
        aria-label={
          enabled
            ? t("auto_refresh.refresh_now_every").replace("{interval}", label)
            : t("auto_refresh.refresh_now")
        }
        className={cn("min-w-0 rounded-r-none", enabled ? "gap-1.5 pr-2 pl-1.5" : "w-8 px-0")}
      >
        <span className="relative grid size-5 shrink-0 place-items-center">
          {enabled && (
            <svg viewBox="0 0 20 20" className="absolute inset-0 -rotate-90" aria-hidden="true">
              <circle
                cx="10"
                cy="10"
                r="9"
                fill="none"
                strokeWidth="1.5"
                className="stroke-border"
              />
              <circle
                key={`${intervalMs}:${cycle}`}
                cx="10"
                cy="10"
                r="9"
                fill="none"
                strokeWidth="1.5"
                strokeLinecap="round"
                pathLength={1}
                className="refresh-ring stroke-primary"
                style={{ "--refresh-ms": `${intervalMs}ms` } as React.CSSProperties}
              />
            </svg>
          )}
          {fetching ? (
            <Spinner aria-hidden="true" className={enabled ? "size-2.5" : "size-3.5"} />
          ) : (
            <RotateCw aria-hidden="true" className={enabled ? "size-2.5" : "size-3.5"} />
          )}
        </span>
        {enabled && <span className="font-mono text-xs tabular-nums">{label}</span>}
      </Button>
      <DropdownMenu>
        <DropdownMenuTrigger
          render={
            <Button
              variant="outline"
              size="icon-sm"
              aria-label={t("auto_refresh.interval")}
              className="w-6 rounded-l-none border-l-0 text-muted-foreground"
            />
          }
        >
          <ChevronDown aria-hidden="true" className="size-3" />
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="min-w-36">
          <DropdownMenuGroup>
            <DropdownMenuLabel>{t("auto_refresh.title")}</DropdownMenuLabel>
            <DropdownMenuRadioGroup
              value={String(intervalMs)}
              onValueChange={(v) => onIntervalChange(Number(v))}
            >
              <DropdownMenuRadioItem value={String(REFRESH_OFF)}>
                {t("auto_refresh.off")}
              </DropdownMenuRadioItem>
              <DropdownMenuSeparator />
              {options.map((ms) => (
                <DropdownMenuRadioItem key={ms} value={String(ms)}>
                  {t("auto_refresh.every").replace("{interval}", formatRefreshInterval(ms))}
                </DropdownMenuRadioItem>
              ))}
            </DropdownMenuRadioGroup>
          </DropdownMenuGroup>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}
