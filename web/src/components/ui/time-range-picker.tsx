"use client";

import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Check, Clock } from "lucide-react";
import type { TimeRangeKey } from "@/lib/time-range";
import { TIME_RANGE_LABELS, TIME_RANGE_PRESETS } from "@/lib/time-range";

export interface TimeRangePickerProps {
  value: TimeRangeKey;
  customStart: string;
  customEnd: string;
  onChange: (key: TimeRangeKey, customStart: string, customEnd: string) => void;
}

export function TimeRangePicker({ value, customStart, customEnd, onChange }: TimeRangePickerProps) {
  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger
          render={<Button variant="outline" size="sm" className="gap-1.5" />}
        >
          <Clock className="size-3.5" />
          {TIME_RANGE_LABELS[value]}
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start">
          {([...TIME_RANGE_PRESETS, "custom"] as TimeRangeKey[]).map((k) => (
            <DropdownMenuItem
              key={k}
              onClick={() => {
                onChange(k, customStart, customEnd);
              }}
            >
              {k === value && <Check className="size-4" />}
              <span className={k === value ? "ml-0" : "ml-6"}>
                {TIME_RANGE_LABELS[k]}
              </span>
            </DropdownMenuItem>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>
      {value === "custom" && (
        <div className="flex items-center gap-2">
          <input
            type="datetime-local"
            value={customStart}
            onChange={(e) => onChange("custom", e.target.value, customEnd)}
            className="h-8 rounded-md border border-input bg-transparent px-2 py-1 text-xs"
          />
          <span className="text-xs text-muted-foreground">–</span>
          <input
            type="datetime-local"
            value={customEnd}
            onChange={(e) => onChange("custom", customStart, e.target.value)}
            className="h-8 rounded-md border border-input bg-transparent px-2 py-1 text-xs"
          />
        </div>
      )}
    </>
  );
}
