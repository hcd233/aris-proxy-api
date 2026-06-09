"use client";

import { useState, useCallback, useMemo } from "react";
import { format, startOfDay, endOfDay, subDays, subHours } from "date-fns";
import { Check, Clock } from "lucide-react";
import type { DateRange } from "react-day-picker";

import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Calendar } from "@/components/ui/calendar";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Input } from "@/components/ui/input";
import type { TimeRangeKey } from "@/lib/time-range";
import { TIME_RANGE_LABELS } from "@/lib/time-range";

export interface TimeRangePickerProps {
  value: TimeRangeKey;
  customStart: string;
  customEnd: string;
  onChange: (key: TimeRangeKey, customStart: string, customEnd: string) => void;
}

const PRESET_KEYS = ["1h", "24h", "7d", "30d"] as const;
type PresetTimeRangeKey = (typeof PRESET_KEYS)[number];

// Preset ranges with their date calculations
const PRESET_RANGES: Record<PresetTimeRangeKey, () => { start: Date; end: Date }> = {
  "1h": () => {
    const end = new Date();
    const start = subHours(end, 1);
    return { start, end };
  },
  "24h": () => {
    const end = new Date();
    const start = subHours(end, 24);
    return { start, end };
  },
  "7d": () => {
    const end = new Date();
    const start = subDays(end, 7);
    return { start, end };
  },
  "30d": () => {
    const end = new Date();
    const start = subDays(end, 30);
    return { start, end };
  },
};

export function TimeRangePicker({
  value,
  customStart,
  customEnd,
  onChange,
}: TimeRangePickerProps) {
  const [open, setOpen] = useState(false);
  const [includeTime, setIncludeTime] = useState(false);
  
  // Initialize date range from custom values or default
  const [dateRange, setDateRange] = useState<DateRange | undefined>(() => {
    if (value === "custom" && customStart && customEnd) {
      return {
        from: new Date(customStart),
        to: new Date(customEnd),
      };
    }
    return undefined;
  });

  // Initialize time values
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

  // Selected preset
  const [selectedPreset, setSelectedPreset] = useState<PresetTimeRangeKey | null>(
    value === "custom" ? null : value
  );

  const handlePresetSelect = useCallback(
    (key: PresetTimeRangeKey) => {
      const { start, end } = PRESET_RANGES[key]();
      setDateRange({ from: start, to: end });
      setSelectedPreset(key);
    },
    []
  );

  const handleDateRangeSelect = useCallback((range: DateRange | undefined) => {
    setDateRange(range);
    setSelectedPreset(null);
  }, []);

  const handleIncludeTimeChange = useCallback((checked: boolean) => {
    setIncludeTime(checked);
    setSelectedPreset(null);
  }, []);

  const customRange = useMemo(() => {
    if (!dateRange?.from || !dateRange?.to) {
      return null;
    }

    let start = dateRange.from;
    let end = dateRange.to;

    if (includeTime) {
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
  }, [dateRange, includeTime, startTime, endTime]);

  const handleCustomApply = useCallback(() => {
    if (!customRange?.isValid) {
      return;
    }

    onChange("custom", customRange.start.toISOString(), customRange.end.toISOString());
    setOpen(false);
  }, [customRange, onChange]);

  const handlePresetApply = useCallback(() => {
    if (selectedPreset) {
      onChange(selectedPreset, "", "");
      setOpen(false);
    }
  }, [selectedPreset, onChange]);

  const displayLabel = useMemo(() => {
    if (value === "custom" && customStart && customEnd) {
      const start = new Date(customStart);
      const end = new Date(customEnd);
      return `${format(start, "MMM d, yyyy HH:mm")} – ${format(end, "MMM d, yyyy HH:mm")}`;
    }
    return TIME_RANGE_LABELS[value];
  }, [value, customStart, customEnd]);

  const draftLabel = useMemo(() => {
    if (selectedPreset) {
      return TIME_RANGE_LABELS[selectedPreset];
    }
    if (customRange && includeTime) {
      return `${format(customRange.start, "MMM d, yyyy HH:mm")} – ${format(customRange.end, "MMM d, yyyy HH:mm")}`;
    }
    if (dateRange?.from && dateRange?.to) {
      return `${format(dateRange.from, "MMM d, yyyy")} – ${format(dateRange.to, "MMM d, yyyy")}`;
    }
    if (dateRange?.from) {
      return `${format(dateRange.from, "MMM d, yyyy")} – Select end date`;
    }
    return "Select a date range";
  }, [selectedPreset, customRange, includeTime, dateRange]);

  const customRangeError = customRange && !customRange.isValid ? "Start must be before end." : "";
  const canApply = Boolean(selectedPreset || customRange?.isValid);

  const handleOpenChange = useCallback((nextOpen: boolean) => {
    setOpen(nextOpen);
    if (nextOpen) {
      // Reset state when opening
      if (value === "custom" && customStart && customEnd) {
        setDateRange({
          from: new Date(customStart),
          to: new Date(customEnd),
        });
        setStartTime(format(new Date(customStart), "HH:mm"));
        setEndTime(format(new Date(customEnd), "HH:mm"));
        setIncludeTime(true);
        setSelectedPreset(null);
      } else {
        setSelectedPreset(value === "custom" ? null : value);
        setDateRange(undefined);
        setIncludeTime(false);
      }
    }
  }, [value, customStart, customEnd]);

  return (
    <Popover open={open} onOpenChange={handleOpenChange}>
      <PopoverTrigger
        render={
          <Button
            variant="outline"
            size="sm"
            className={cn(
              "gap-1.5 h-9 px-3 text-sm font-normal",
              "border-input bg-transparent hover:bg-accent hover:text-accent-foreground",
              "focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2",
              "transition-colors duration-150"
            )}
          />
        }
      >
        <Clock className="size-3.5 text-muted-foreground" />
        <span className="truncate max-w-[200px]">{displayLabel}</span>
      </PopoverTrigger>
      <PopoverContent
        className="w-[calc(100vw-2rem)] max-w-[42rem] overflow-hidden rounded-xl border bg-popover p-0 shadow-xl"
        align="start"
        sideOffset={8}
      >
        <div className="grid md:grid-cols-[11rem_1fr]">
          <div className="border-b border-border bg-muted/30 p-3 md:border-b-0 md:border-r">
            <div className="px-1 text-[0.7rem] font-semibold uppercase tracking-wide text-muted-foreground">
              Quick Select
            </div>
            <div className="mt-2 grid grid-cols-2 gap-1.5 md:grid-cols-1">
              {PRESET_KEYS.map((key) => (
                <Button
                  key={key}
                  variant="ghost"
                  size="sm"
                  aria-pressed={selectedPreset === key}
                  className={cn(
                    "h-9 justify-start gap-2 rounded-lg px-2.5 text-xs font-medium transition-colors duration-150",
                    selectedPreset === key
                      ? "bg-primary text-primary-foreground shadow-sm hover:bg-primary/90 hover:text-primary-foreground"
                      : "text-muted-foreground hover:bg-background hover:text-foreground"
                  )}
                  onClick={() => handlePresetSelect(key)}
                >
                  <span className="flex size-4 items-center justify-center">
                    {selectedPreset === key && <Check className="size-3" />}
                  </span>
                  {TIME_RANGE_LABELS[key]}
                </Button>
              ))}
            </div>
          </div>

          <div className="min-w-0">
            <div className="p-3 pb-2">
              <Calendar
                mode="range"
                selected={dateRange}
                onSelect={handleDateRangeSelect}
                numberOfMonths={1}
                defaultMonth={dateRange?.from}
                className="mx-auto rounded-lg border bg-background shadow-sm"
              />
            </div>

            <div className="border-t border-border bg-muted/20 p-3">
              <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                <div className="flex items-center gap-2">
                  <Switch
                    id="include-time"
                    checked={includeTime}
                    onCheckedChange={handleIncludeTimeChange}
                  />
                  <Label htmlFor="include-time" className="text-sm font-medium">
                    Include time
                  </Label>
                </div>

                {includeTime && (
                  <div className="grid grid-cols-2 gap-2 sm:w-56">
                    <div className="space-y-1">
                      <Label htmlFor="time-range-start-time" className="text-xs text-muted-foreground">Start time</Label>
                      <Input
                        id="time-range-start-time"
                        type="time"
                        value={startTime}
                        onChange={(e) => {
                          setStartTime(e.target.value);
                          setSelectedPreset(null);
                        }}
                        className="h-9 text-xs"
                      />
                    </div>
                    <div className="space-y-1">
                      <Label htmlFor="time-range-end-time" className="text-xs text-muted-foreground">End time</Label>
                      <Input
                        id="time-range-end-time"
                        type="time"
                        value={endTime}
                        onChange={(e) => {
                          setEndTime(e.target.value);
                          setSelectedPreset(null);
                        }}
                        className="h-9 text-xs"
                      />
                    </div>
                  </div>
                )}
              </div>
            </div>
          </div>
        </div>

        <div className="flex flex-col gap-3 border-t border-border bg-background p-3 sm:flex-row sm:items-center sm:justify-between">
          <div className="min-w-0 space-y-1 text-xs text-muted-foreground">
            <div className="flex min-w-0">
              <span className="shrink-0 font-medium text-foreground">Range:&nbsp;</span>
              <span className="min-w-0 truncate">{draftLabel}</span>
            </div>
            {customRangeError && (
              <div role="alert" className="text-destructive">
                {customRangeError}
              </div>
            )}
          </div>
          <div className="flex justify-end gap-2">
            <Button
              variant="ghost"
              size="sm"
              className="h-8 px-3 text-xs"
              onClick={() => setOpen(false)}
            >
              Cancel
            </Button>
            <Button
              size="sm"
              onClick={selectedPreset ? handlePresetApply : handleCustomApply}
              disabled={!canApply}
              className="h-8 px-4 text-xs font-medium"
            >
              Apply
            </Button>
          </div>
        </div>
      </PopoverContent>
    </Popover>
  );
}