"use client";

import { useState } from "react";
import { ChevronDown, RotateCw } from "lucide-react";

import { useT } from "@/lib/i18n";
import { REFRESH_OFF, formatRefreshInterval } from "@/lib/refresh-interval";
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
  // 关闭时保留上一次的频率文字，让收起动画里文字随宽度一起消失而不是先变空
  const [shownMs, setShownMs] = useState(intervalMs);
  if (enabled && intervalMs !== shownMs) setShownMs(intervalMs);
  const label = formatRefreshInterval(shownMs > 0 ? shownMs : options[0]);

  return (
    <div role="group" className="inline-flex">
      <Button
        variant="outline"
        size="sm"
        onClick={onRefresh}
        data-auto-refresh={enabled ? "on" : "off"}
        data-fetching={fetching ? "" : undefined}
        aria-label={
          enabled
            ? t("auto_refresh.refresh_now_every").replace("{interval}", label)
            : t("auto_refresh.refresh_now")
        }
        className="group/refresh min-w-0 gap-0 rounded-r-none px-1.5"
      >
        {/* 环、图标、加载圈叠在同一个 20px 格子里，共用同一圆心 */}
        <span className="grid size-5 shrink-0 place-items-center *:[grid-area:1/1]">
          {/* svg 必须显式 size-full：Button 会把未声明 size-* 的 svg 强制缩成 14px */}
          <svg
            viewBox="0 0 20 20"
            aria-hidden="true"
            className="size-full -rotate-90 opacity-0 transition-opacity duration-300 group-data-[auto-refresh=on]/refresh:opacity-100 group-data-[auto-refresh=on]/refresh:group-data-fetching/refresh:opacity-30"
          >
            <circle cx="10" cy="10" r="9" fill="none" strokeWidth="1.5" className="stroke-border" />
            {enabled && (
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
            )}
          </svg>
          {/* 定时开启时图标缩进环内；拉取中图标转出、加载圈淡入 */}
          <RotateCw
            aria-hidden="true"
            className="size-3.5 transition-[scale,rotate,opacity] duration-300 ease-out group-data-[auto-refresh=on]/refresh:scale-[0.72] group-data-fetching/refresh:scale-50 group-data-fetching/refresh:rotate-180 group-data-fetching/refresh:opacity-0"
          />
          <Spinner
            aria-hidden="true"
            className="size-3.5 scale-50 opacity-0 transition-[scale,opacity] duration-300 ease-out group-data-fetching/refresh:scale-100 group-data-[auto-refresh=on]/refresh:group-data-fetching/refresh:scale-[0.72] group-data-fetching/refresh:opacity-100"
          />
        </span>
        {/* 频率文字随开关展开/收起：0fr↔1fr 让宽度可过渡 */}
        <span
          aria-hidden="true"
          className="grid grid-cols-[0fr] opacity-0 transition-[grid-template-columns,opacity] duration-300 ease-out group-data-[auto-refresh=on]/refresh:grid-cols-[1fr] group-data-[auto-refresh=on]/refresh:opacity-100"
        >
          <span className="overflow-hidden">
            <span className="block pr-0.5 pl-1.5 font-mono text-xs tabular-nums">{label}</span>
          </span>
        </span>
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
              <DropdownMenuRadioItem value={String(REFRESH_OFF)} closeOnClick>
                {t("auto_refresh.off")}
              </DropdownMenuRadioItem>
              <DropdownMenuSeparator />
              {options.map((ms) => (
                <DropdownMenuRadioItem key={ms} value={String(ms)} closeOnClick>
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
