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

// Preset ranges with their date calculations
const PRESET_RANGES: Record<string, () => { start: Date; end: Date }> = {
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
  const [selectedPreset, setSelectedPreset] = useState<string | null>(
    value !== "custom" ? value : null
  );

  const handlePresetSelect = useCallback(
    (key: string) => {
      const preset = PRESET_RANGES[key];
      if (preset) {
        const { start, end } = preset();
        setDateRange({ from: start, to: end });
        setSelectedPreset(key);
        // Don't close popover, let user see the selection
      }
    },
    []
  );

  const handleCustomApply = useCallback(() => {
    if (dateRange?.from && dateRange?.to) {
      let start = dateRange.from;
      let end = dateRange.to;
      
      if (includeTime) {
        // Apply time to dates
        const [startHours, startMinutes] = startTime.split(":").map(Number);
        const [endHours, endMinutes] = endTime.split(":").map(Number);
        
        start = new Date(start);
        start.setHours(startHours, startMinutes, 0, 0);
        
        end = new Date(end);
        end.setHours(endHours, endMinutes, 59, 999);
      } else {
        // Use start of day and end of day
        start = startOfDay(start);
        end = endOfDay(end);
      }
      
      onChange("custom", start.toISOString(), end.toISOString());
      setOpen(false);
    }
  }, [dateRange, includeTime, startTime, endTime, onChange]);

  const handlePresetApply = useCallback(() => {
    if (selectedPreset) {
      onChange(selectedPreset as TimeRangeKey, "", "");
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
        setSelectedPreset(value);
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
        className="w-auto p-0"
        align="start"
        sideOffset={4}
      >
        <div className="flex flex-col">
          {/* Quick presets */}
          <div className="flex flex-col border-b border-border p-2">
            <div className="text-xs font-medium text-muted-foreground px-2 py-1">
              Quick Select
            </div>
            <div className="flex flex-wrap gap-1">
              {Object.keys(PRESET_RANGES).map((key) => (
                <Button
                  key={key}
                  variant="ghost"
                  size="sm"
                  className={cn(
                    "h-7 px-2 text-xs",
                    selectedPreset === key && "bg-accent text-accent-foreground"
                  )}
                  onClick={() => handlePresetSelect(key)}
                >
                  {selectedPreset === key && <Check className="size-3 mr-1" />}
                  {TIME_RANGE_LABELS[key as TimeRangeKey]}
                </Button>
              ))}
            </div>
          </div>

          {/* Calendar */}
          <div className="p-2">
            <Calendar
              mode="range"
              selected={dateRange}
              onSelect={setDateRange}
              numberOfMonths={1}
              defaultMonth={dateRange?.from}
              className="rounded-md border"
            />
          </div>

          {/* Time toggle and inputs */}
          <div className="border-t border-border p-2">
            <div className="flex items-center space-x-2 mb-2">
              <Switch
                id="include-time"
                checked={includeTime}
                onCheckedChange={setIncludeTime}
              />
              <Label htmlFor="include-time" className="text-xs">
                Include time
              </Label>
            </div>
            
            {includeTime && (
              <div className="grid grid-cols-2 gap-2">
                <div className="space-y-1">
                  <Label className="text-xs text-muted-foreground">Start time</Label>
                  <Input
                    type="time"
                    value={startTime}
                    onChange={(e) => setStartTime(e.target.value)}
                    className="h-8 text-xs"
                  />
                </div>
                <div className="space-y-1">
                  <Label className="text-xs text-muted-foreground">End time</Label>
                  <Input
                    type="time"
                    value={endTime}
                    onChange={(e) => setEndTime(e.target.value)}
                    className="h-8 text-xs"
                  />
                </div>
              </div>
            )}
          </div>

          {/* Apply button */}
          <div className="border-t border-border p-2 flex justify-end gap-2">
            {selectedPreset ? (
              <Button
                size="sm"
                onClick={handlePresetApply}
                className="h-7 px-3 text-xs"
              >
                Apply
              </Button>
            ) : (
              <Button
                size="sm"
                onClick={handleCustomApply}
                disabled={!dateRange?.from || !dateRange?.to}
                className="h-7 px-3 text-xs"
              >
                Apply Range
              </Button>
            )}
          </div>
        </div>
      </PopoverContent>
    </Popover>
  );
}