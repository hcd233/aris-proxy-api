"use client";

import { useIsMobile } from "@/hooks/use-mobile";
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";

interface SegmentedChoiceProps<T extends string> {
  value: T;
  options: ReadonlyArray<{ value: T; label: string }>;
  onChange: (value: T) => void;
  "aria-label": string;
}

/** 面板右上角的维度切换：桌面是紧凑按钮组，手机一排放不下，改为下拉（spec 2026-10-29-mobile-tablet-adapt §5.4）。 */
export function SegmentedChoice<T extends string>({
  value,
  options,
  onChange,
  "aria-label": ariaLabel,
}: SegmentedChoiceProps<T>) {
  const isMobile = useIsMobile();

  if (isMobile) {
    return (
      <Select
        value={value}
        items={options}
        onValueChange={(next) => {
          if (next) onChange(next as T);
        }}
      >
        <SelectTrigger size="sm" aria-label={ariaLabel}>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectGroup>
            {options.map((o) => (
              <SelectItem key={o.value} value={o.value}>
                {o.label}
              </SelectItem>
            ))}
          </SelectGroup>
        </SelectContent>
      </Select>
    );
  }

  return (
    <ToggleGroup
      variant="segmented"
      size="sm"
      spacing={0.5}
      aria-label={ariaLabel}
      value={[value]}
      onValueChange={(next) => {
        if (next[0]) onChange(next[0] as T);
      }}
    >
      {options.map((o) => (
        <ToggleGroupItem key={o.value} value={o.value} className="h-6 px-2 text-[11px]">
          {o.label}
        </ToggleGroupItem>
      ))}
    </ToggleGroup>
  );
}
