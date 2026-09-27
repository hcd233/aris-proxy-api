"use client";

import { useRef } from "react";
import { cn } from "@/lib/utils";

export interface ViewSwitchOption<T extends string = string> {
  value: T;
  label: string;
}

/**
 * 分段控件：等宽选项，选中项深墨底白字（与主按钮语义一致）。
 * 用于同一份数据的两种排布切换（如 分组 / 平铺）。
 *
 * 语义为 radiogroup/radio（互斥单选、没有 tabpanel 内容区对应）：
 * 选中项 tabIndex=0 其余 -1（roving tabindex），方向键在选项间移动并选中。
 * 泛型参数收窄 value/onChange/options 的字面量联合，调用方无需类型断言。
 */
export function ViewSwitch<T extends string = string>({
  value,
  onChange,
  options,
}: {
  value: T;
  onChange: (v: T) => void;
  options: ViewSwitchOption<T>[];
}) {
  const itemRefs = useRef<(HTMLButtonElement | null)[]>([]);

  const moveFocus = (from: number, delta: number) => {
    const next = (from + delta + options.length) % options.length;
    itemRefs.current[next]?.focus();
    onChange(options[next].value);
  };

  return (
    <div
      role="radiogroup"
      className="inline-flex shrink-0 overflow-hidden rounded-lg border border-input bg-background"
    >
      {options.map((o, i) => (
        <button
          key={o.value}
          ref={(el) => {
            itemRefs.current[i] = el;
          }}
          type="button"
          role="radio"
          aria-checked={value === o.value}
          tabIndex={value === o.value ? 0 : -1}
          className={cn(
            "px-3 py-1.5 text-xs transition-colors",
            value === o.value
              ? "bg-foreground font-semibold text-background"
              : "text-muted-foreground hover:bg-accent hover:text-foreground",
          )}
          onClick={() => onChange(o.value)}
          onKeyDown={(e) => {
            if (e.key === "ArrowRight" || e.key === "ArrowDown") {
              e.preventDefault();
              moveFocus(i, 1);
            } else if (e.key === "ArrowLeft" || e.key === "ArrowUp") {
              e.preventDefault();
              moveFocus(i, -1);
            }
          }}
        >
          {o.label}
        </button>
      ))}
    </div>
  );
}
