"use client";

import * as React from "react";
import {
  Combobox,
  ComboboxContent,
  ComboboxEmpty,
  ComboboxInput,
  ComboboxItem,
  ComboboxList,
  ComboboxSeparator,
  ComboboxTrigger,
} from "@/components/ui/combobox";
import { Button } from "@/components/ui/button";
import { TooltipRoot, TooltipTrigger, TooltipContent } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import { useT } from "@/lib/i18n";

export interface MultiSelectPillProps {
  label: string;
  options: string[];
  value: string[];
  onChange: (next: string[]) => void;
  searchable?: boolean;
  emptyText?: string;
  className?: string;
  /** display label for option value (default = value itself) */
  formatOption?: (value: string) => string;
}

/**
 * 胶囊筛选器的多选弹层：shadcn Combobox（multiple + 弹层内搜索）承载
 * 过滤、键盘导航与选中管理，外观保持筛选条胶囊形态。
 */
export function MultiSelectPill({
  label,
  options,
  value,
  onChange,
  searchable,
  emptyText,
  className,
  formatOption,
}: MultiSelectPillProps) {
  const t = useT();
  const [open, setOpen] = React.useState(false);

  const showSearch = searchable ?? options.length > 8;
  const active = value.length > 0;
  const display = React.useCallback(
    (opt: string) => (formatOption ? formatOption(opt) : opt),
    [formatOption],
  );

  return (
    <Combobox
      multiple
      items={options}
      value={value}
      onValueChange={onChange}
      open={open}
      onOpenChange={setOpen}
      itemToStringLabel={display}
      onInputValueChange={(_inputValue, eventDetails) => {
        // 点选选项后保留过滤文本（取消 input-clear），便于连续勾选同一筛选项
        if (eventDetails.isItemPress) eventDetails.cancel();
      }}
    >
      <ComboboxTrigger
        render={
          <button
            type="button"
            className={cn(
              "inline-flex h-8 items-center gap-1.5 rounded-full border px-3 text-sm transition-colors",
              active
                ? "border-ring/40 bg-accent/60 text-foreground hover:bg-accent"
                : "border-input bg-transparent text-muted-foreground hover:border-ring hover:text-foreground",
              className,
            )}
          />
        }
      >
        <span>
          {label}
          {active ? ` · ${value.length}` : ""}
        </span>
      </ComboboxTrigger>
      <ComboboxContent align="start" className="w-auto min-w-[200px] max-w-[280px]">
        {showSearch && <ComboboxInput showTrigger={false} placeholder={t("multi_select.filter")} />}
        <ComboboxList>
          {options.map((opt) => (
            <ComboboxItem key={opt} value={opt}>
              <TooltipRoot>
                <TooltipTrigger
                  render={<span className="block truncate text-left">{display(opt)}</span>}
                />
                <TooltipContent side="top" className="max-w-xs break-all">
                  {display(opt)}
                </TooltipContent>
              </TooltipRoot>
            </ComboboxItem>
          ))}
          <ComboboxEmpty>{emptyText ?? t("multi_select.no_options")}</ComboboxEmpty>
        </ComboboxList>
        {active && (
          <>
            <ComboboxSeparator />
            <Button
              variant="ghost"
              size="sm"
              className="w-full justify-start text-xs text-muted-foreground"
              onClick={() => onChange([])}
            >
              {t("multi_select.clear")}
            </Button>
          </>
        )}
      </ComboboxContent>
    </Combobox>
  );
}
