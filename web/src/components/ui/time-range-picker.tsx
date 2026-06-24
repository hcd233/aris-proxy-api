"use client";

import { useState, useCallback, useMemo } from "react";
import { format, startOfDay, endOfDay } from "date-fns";
import { CalendarIcon } from "lucide-react";
import type { DateRange } from "react-day-picker";

import { cn } from "@/lib/utils";
import { useT } from "@/lib/i18n";
import { Button } from "@/components/ui/button";
import { Calendar } from "@/components/ui/calendar";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import type { TimeRangeKey } from "@/lib/time-range";

export interface TimeRangePickerProps {
  value: TimeRangeKey;
  customStart: string;
  customEnd: string;
  onChange: (key: TimeRangeKey, customStart: string, customEnd: string) => void;
  className?: string;
}

const PRESET_KEYS: TimeRangeKey[] = ["1h", "24h", "7d", "30d"];

const PRESET_LABELS: Record<TimeRangeKey, string> = {
  "1h": "1H",
  "24h": "24H",
  "7d": "7D",
  "30d": "30D",
  custom: "Custom",
};

export function TimeRangePicker({
  value,
  customStart,
  customEnd,
  onChange,
  className,
}: TimeRangePickerProps) {
  const t = useT();
  const [open, setOpen] = useState(false);
  const [dateRange, setDateRange] = useState<DateRange | undefined>(() => {
    if (value === "custom" && customStart && customEnd) {
      return {
        from: new Date(customStart),
        to: new Date(customEnd),
      };
    }
    return undefined;
  });

  const [startTime, setStartTime] = useState<string>(() => {
    if (value === "custom" && customStart) {
      return format(new Date(customStart), "HH:mm");
    }
    return "00:00";
  });

  const [endTime, setEndTime] = useState<string>(() => {
    if (value === "custom" && customEnd) {
      return format(new Date(customEnd), "HH:mm");
    }
    return "23:59";
  });

  const [showTimePicker, setShowTimePicker] = useState(false);

  const handlePresetClick = useCallback(
    (key: TimeRangeKey) => {
      onChange(key, "", "");
    },
    [onChange]
  );

  const handleCustomClick = useCallback(() => {
    setOpen(true);
  }, []);

  const handleDateRangeSelect = useCallback((range: DateRange | undefined) => {
    setDateRange(range);
  }, []);

  const customRange = useMemo(() => {
    if (!dateRange?.from || !dateRange?.to) {
      return null;
    }

    let start = dateRange.from;
    let end = dateRange.to;

    if (showTimePicker) {
      const [startHours, startMinutes] = startTime.split(":").map(Number);
      const [endHours, endMinutes] = endTime.split(":").map(Number);

      start = new Date(start);
      start.setHours(startHours, startMinutes, 0, 0);

      end = new Date(end);
      end.setHours(endHours, endMinutes, 59, 999);
    } else {
      start = startOfDay(start);
      end = endOfDay(end);
    }

    return {
      start,
      end,
      isValid: start.getTime() <= end.getTime(),
    };
  }, [dateRange, showTimePicker, startTime, endTime]);

  const handleCustomApply = useCallback(() => {
    if (!customRange?.isValid) {
      return;
    }
    onChange("custom", customRange.start.toISOString(), customRange.end.toISOString());
    setOpen(false);
  }, [customRange, onChange]);

  const handleOpenChange = useCallback(
    (nextOpen: boolean) => {
      setOpen(nextOpen);
      if (nextOpen) {
        if (value === "custom" && customStart && customEnd) {
          setDateRange({
            from: new Date(customStart),
            to: new Date(customEnd),
          });
          setStartTime(format(new Date(customStart), "HH:mm"));
          setEndTime(format(new Date(customEnd), "HH:mm"));
          setShowTimePicker(true);
        } else {
          setDateRange(undefined);
          setStartTime("00:00");
          setEndTime("23:59");
          setShowTimePicker(false);
        }
      }
    },
    [value, customStart, customEnd]
  );

  const draftLabel = useMemo(() => {
    if (dateRange?.from && dateRange?.to) {
      if (showTimePicker) {
        return `${format(dateRange.from, "MMM d HH:mm")} – ${format(dateRange.to, "MMM d HH:mm")}`;
      }
      return `${format(dateRange.from, "MMM d")} – ${format(dateRange.to, "MMM d")}`;
    }
    if (dateRange?.from) {
      return `${format(dateRange.from, "MMM d")} – Select end`;
    }
    return "Select date range";
  }, [dateRange, showTimePicker]);

  const customRangeError = customRange && !customRange.isValid ? "Start must be before end" : "";
  const canApply = Boolean(customRange?.isValid);

  const customDisplayLabel = useMemo(() => {
    if (value === "custom" && customStart && customEnd) {
      const start = new Date(customStart);
      const end = new Date(customEnd);
      return `${format(start, "MMM d")} – ${format(end, "MMM d")}`;
    }
    return t("time.custom");
  }, [value, customStart, customEnd, t]);

  const timeLabels: Record<TimeRangeKey, string> = {
    "1h": t("time.last_1h"),
    "24h": t("time.last_24h"),
    "7d": t("time.last_7d"),
    "30d": t("time.last_30d"),
    custom: t("time.custom"),
  };

  return (
    <div className={cn("flex items-center gap-0.5 rounded-lg bg-muted p-0.5", className)}>
      {PRESET_KEYS.map((key) => (
        <button
          key={key}
          type="button"
          onClick={() => handlePresetClick(key)}
          className={cn(
            "inline-flex h-8 items-center justify-center rounded-md px-3 text-xs font-medium transition-colors",
            value === key
              ? "bg-background text-foreground shadow-sm"
              : "text-muted-foreground hover:text-foreground"
          )}
        >
          {timeLabels[key]}
        </button>
      ))}

      <Popover open={open} onOpenChange={handleOpenChange}>
        <PopoverTrigger
          render={
            <button
              type="button"
              onClick={handleCustomClick}
              className={cn(
                "inline-flex h-8 items-center justify-center gap-1.5 rounded-md px-3 text-xs font-medium transition-colors",
                value === "custom"
                  ? "bg-background text-foreground shadow-sm"
                  : "text-muted-foreground hover:text-foreground"
              )}
            />
          }
        >
          <CalendarIcon className="size-3" />
          <span className="hidden sm:inline">{customDisplayLabel}</span>
        </PopoverTrigger>
        <PopoverContent
          className="w-auto p-0 rounded-lg border bg-popover shadow-sm"
          align="end"
          sideOffset={8}
        >
          <div className="p-3">
            <Calendar
              mode="range"
              selected={dateRange}
              onSelect={handleDateRangeSelect}
              numberOfMonths={2}
              defaultMonth={dateRange?.from}
              className="rounded-md border bg-background"
            />
          </div>

          <div className="border-t border-border px-3 py-2">
            <button
              type="button"
              className="text-xs text-muted-foreground hover:text-foreground transition-colors"
              onClick={() => setShowTimePicker(!showTimePicker)}
            >
              {showTimePicker ? "Hide time" : "Add time"}
            </button>
          </div>

          {showTimePicker && (
            <div className="border-t border-border px-3 py-3">
              <div className="grid grid-cols-2 gap-3">
                <div className="space-y-1">
                  <Label className="text-[0.65rem] uppercase tracking-wider text-muted-foreground">
                    Start
                  </Label>
                  <Input
                    type="time"
                    value={startTime}
                    onChange={(e) => setStartTime(e.target.value)}
                    className="h-8 text-xs"
                  />
                </div>
                <div className="space-y-1">
                  <Label className="text-[0.65rem] uppercase tracking-wider text-muted-foreground">
                    End
                  </Label>
                  <Input
                    type="time"
                    value={endTime}
                    onChange={(e) => setEndTime(e.target.value)}
                    className="h-8 text-xs"
                  />
                </div>
              </div>
            </div>
          )}

          <div className="flex items-center justify-between border-t border-border bg-muted/30 px-3 py-2.5">
            <div className="min-w-0 text-xs text-muted-foreground">
              <span className="font-medium text-foreground">Range:</span>{" "}
              <span className="truncate">{draftLabel}</span>
              {customRangeError && (
                <span className="ml-2 text-destructive">{customRangeError}</span>
              )}
            </div>
            <div className="flex gap-2">
              <Button
                variant="ghost"
                size="sm"
                className="h-7 px-2.5 text-xs"
                onClick={() => setOpen(false)}
              >
                Cancel
              </Button>
              <Button
                size="sm"
                onClick={handleCustomApply}
                disabled={!canApply}
                className="h-7 px-3 text-xs"
              >
                Apply
              </Button>
            </div>
          </div>
        </PopoverContent>
      </Popover>
    </div>
  );
}
