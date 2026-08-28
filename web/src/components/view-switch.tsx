"use client";

import { cn } from "@/lib/utils";

export interface ViewSwitchOption {
  value: string;
  label: string;
}

/**
 * 分段控件：等宽选项，选中项深墨底白字（与主按钮语义一致）。
 * 用于同一份数据的两种排布切换（如 分组 / 平铺）。
 */
export function ViewSwitch({
  value,
  onChange,
  options,
}: {
  value: string;
  onChange: (v: string) => void;
  options: ViewSwitchOption[];
}) {
  return (
    <div
      role="tablist"
      className="inline-flex shrink-0 overflow-hidden rounded-lg border border-input bg-background"
    >
      {options.map((o) => (
        <button
          key={o.value}
          type="button"
          role="tab"
          aria-selected={value === o.value}
          className={cn(
            "px-3 py-1.5 text-xs transition-colors",
            value === o.value
              ? "bg-foreground font-semibold text-background"
              : "text-muted-foreground hover:bg-accent hover:text-foreground",
          )}
          onClick={() => onChange(o.value)}
        >
          {o.label}
        </button>
      ))}
    </div>
  );
}
