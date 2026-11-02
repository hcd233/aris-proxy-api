"use client";

import { useId } from "react";

import { useT } from "@/lib/i18n";
import type { RefreshIntervalOption } from "@/lib/refresh-interval";
import { Switch } from "@/components/ui/switch";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

export interface AutoRefreshControlProps {
  enabled: boolean;
  onEnabledChange: (enabled: boolean) => void;
  intervalMs: number;
  onIntervalChange: (intervalMs: number) => void;
  options: readonly RefreshIntervalOption[];
}

/** 页头定时刷新控制：开关 + 频率档位；关闭时档位禁用。 */
export function AutoRefreshControl({
  enabled,
  onEnabledChange,
  intervalMs,
  onIntervalChange,
  options,
}: AutoRefreshControlProps) {
  const t = useT();
  const switchId = useId();
  const items = options.map((option) => ({
    value: String(option.ms),
    label: option.label,
  }));

  return (
    <div className="flex items-center gap-2">
      <div className="flex items-center gap-1.5">
        <Switch id={switchId} size="sm" checked={enabled} onCheckedChange={onEnabledChange} />
        <label
          htmlFor={switchId}
          className="cursor-pointer text-xs whitespace-nowrap text-muted-foreground"
        >
          {t("auto_refresh.label")}
        </label>
      </div>
      <Select
        items={items}
        value={String(intervalMs)}
        disabled={!enabled}
        onValueChange={(value) => onIntervalChange(Number(value))}
      >
        <SelectTrigger
          size="sm"
          aria-label={t("auto_refresh.interval")}
          className="h-8 w-[4.25rem] px-2 text-xs"
        >
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {options.map((option) => (
            <SelectItem key={option.ms} value={String(option.ms)}>
              {option.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}
