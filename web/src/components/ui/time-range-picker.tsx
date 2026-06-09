"use client";

import { useState, useCallback, useMemo } from "react";
import { format } from "date-fns";
import { Check, Clock } from "lucide-react";

import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Calendar } from "@/components/ui/calendar";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { TimeRangeKey } from "@/lib/time-range";
import { TIME_RANGE_LABELS, TIME_RANGE_PRESETS } from "@/lib/time-range";

export interface TimeRangePickerProps {
  value: TimeRangeKey;
  customStart: string;
  customEnd: string;
  onChange: (key: TimeRangeKey, customStart: string, customEnd: string) => void;
}

export function TimeRangePicker({
  value,
  customStart,
  customEnd,
  onChange,
}: TimeRangePickerProps) {
  const [open, setOpen] = useState(false);
  const [startDate, setStartDate] = useState<Date | undefined>(
    customStart ? new Date(customStart) : undefined
  );
  const [endDate, setEndDate] = useState<Date | undefined>(
    customEnd ? new Date(customEnd) : undefined
  );
  const [startTime, setStartTime] = useState<string>(
    customStart ? format(new Date(customStart), "HH:mm") : "00:00"
  );
  const [endTime, setEndTime] = useState<string>(
    customEnd ? format(new Date(customEnd), "HH:mm") : "23:59"
  );

  const handlePresetSelect = useCallback(
    (key: TimeRangeKey) => {
      if (key !== "custom") {
        onChange(key, "", "");
        setOpen(false);
      } else {
        onChange(key, customStart, customEnd);
      }
    },
    [onChange, customStart, customEnd]
  );

  const handleCustomApply = useCallback(() => {
    if (startDate && endDate) {
      const start = new Date(startDate);
      const end = new Date(endDate);
      
      // Apply time
      const [startHours, startMinutes] = startTime.split(":").map(Number);
      const [endHours, endMinutes] = endTime.split(":").map(Number);
      
      start.setHours(startHours, startMinutes, 0, 0);
      end.setHours(endHours, endMinutes, 59, 999);
      
      onChange("custom", start.toISOString(), end.toISOString());
      setOpen(false);
    }
  }, [startDate, endDate, startTime, endTime, onChange]);

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
    if (nextOpen && value === "custom" && customStart && customEnd) {
      setStartDate(new Date(customStart));
      setEndDate(new Date(customEnd));
      setStartTime(format(new Date(customStart), "HH:mm"));
      setEndTime(format(new Date(customEnd), "HH:mm"));
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
          {/* Preset options */}
          <div className="flex flex-col border-b border-border p-2">
            <div className="text-xs font-medium text-muted-foreground px-2 py-1">
              Quick Select
            </div>
            <div className="flex flex-col gap-0.5">
              {TIME_RANGE_PRESETS.map((key) => (
                <Button
                  key={key}
                  variant="ghost"
                  size="sm"
                  className={cn(
                    "justify-start h-8 px-2 text-sm",
                    value === key && "bg-accent text-accent-foreground"
                  )}
                  onClick={() => handlePresetSelect(key)}
                >
                  {value === key && <Check className="size-3.5 mr-2" />}
                  <span className={value === key ? "" : "ml-5"}>
                    {TIME_RANGE_LABELS[key]}
                  </span>
                </Button>
              ))}
              <Button
                variant="ghost"
                size="sm"
                className={cn(
                  "justify-start h-8 px-2 text-sm",
                  value === "custom" && "bg-accent text-accent-foreground"
                )}
                onClick={() => handlePresetSelect("custom")}
              >
                {value === "custom" && <Check className="size-3.5 mr-2" />}
                <span className={value === "custom" ? "" : "ml-5"}>
                  Custom Range
                </span>
              </Button>
            </div>
          </div>

          {/* Custom range picker */}
          {value === "custom" && (
            <div className="p-3">
              <div className="grid grid-cols-2 gap-4">
                {/* Start date and time */}
                <div className="space-y-2">
                  <div className="text-xs font-medium text-muted-foreground">
                    Start
                  </div>
                  <Calendar
                    mode="single"
                    selected={startDate}
                    onSelect={setStartDate}
                    disabled={(date) =>
                      endDate ? date > endDate : false
                    }
                    className="rounded-md border"
                  />
                  <div className="flex items-center gap-2">
                    <Clock className="size-3.5 text-muted-foreground" />
                    <Select 
                      value={startTime} 
                      onValueChange={(value: unknown) => setStartTime(value as string)}
                    >
                      <SelectTrigger className="h-8 text-xs">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {Array.from({ length: 24 }, (_, i) => 
                          Array.from({ length: 4 }, (_, j) => {
                            const hour = i.toString().padStart(2, "0");
                            const minute = (j * 15).toString().padStart(2, "0");
                            const time = `${hour}:${minute}`;
                            return (
                              <SelectItem key={time} value={time}>
                                {time}
                              </SelectItem>
                            );
                          })
                        ).flat()}
                      </SelectContent>
                    </Select>
                  </div>
                </div>

                {/* End date and time */}
                <div className="space-y-2">
                  <div className="text-xs font-medium text-muted-foreground">
                    End
                  </div>
                  <Calendar
                    mode="single"
                    selected={endDate}
                    onSelect={setEndDate}
                    disabled={(date) =>
                      startDate ? date < startDate : false
                    }
                    className="rounded-md border"
                  />
                  <div className="flex items-center gap-2">
                    <Clock className="size-3.5 text-muted-foreground" />
                    <Select 
                      value={endTime} 
                      onValueChange={(value: unknown) => setEndTime(value as string)}
                    >
                      <SelectTrigger className="h-8 text-xs">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {Array.from({ length: 24 }, (_, i) => 
                          Array.from({ length: 4 }, (_, j) => {
                            const hour = i.toString().padStart(2, "0");
                            const minute = (j * 15).toString().padStart(2, "0");
                            const time = `${hour}:${minute}`;
                            return (
                              <SelectItem key={time} value={time}>
                                {time}
                              </SelectItem>
                            );
                          })
                        ).flat()}
                      </SelectContent>
                    </Select>
                  </div>
                </div>
              </div>

              {/* Apply button */}
              <div className="mt-4 flex justify-end">
                <Button
                  size="sm"
                  onClick={handleCustomApply}
                  disabled={!startDate || !endDate}
                  className="h-8 px-3 text-xs"
                >
                  Apply Range
                </Button>
              </div>
            </div>
          )}
        </div>
      </PopoverContent>
    </Popover>
  );
}