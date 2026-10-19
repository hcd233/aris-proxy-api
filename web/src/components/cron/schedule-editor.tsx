"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import cronstrue from "cronstrue";
import "cronstrue/locales/zh_CN";
import type { CronJobItem } from "@/lib/types";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Field, FieldGroup, FieldLabel } from "@/components/ui/field";
import { useI18n } from "@/lib/i18n";

type RepeatMode = "minute" | "hour" | "day" | "week" | "month" | "advanced";

interface ParsedSpec {
  mode: RepeatMode;
  minute: number;
  hour: number;
  dayOfMonth: number;
  dayOfWeek: number;
  advancedSpec: string;
}

function specToParsed(spec: string): ParsedSpec {
  const parts = spec.trim().split(/\s+/);
  if (parts.length !== 5) {
    return {
      mode: "advanced",
      minute: 0,
      hour: 0,
      dayOfMonth: 1,
      dayOfWeek: 0,
      advancedSpec: spec,
    };
  }

  const [min, hr, dom, , dow] = parts;

  if (min === "*" && hr === "*" && dom === "*" && dow === "*") {
    return { mode: "minute", minute: 0, hour: 0, dayOfMonth: 1, dayOfWeek: 0, advancedSpec: spec };
  }
  if (hr === "*" && dom === "*" && dow === "*") {
    return {
      mode: "hour",
      minute: parseInt(min) || 0,
      hour: 0,
      dayOfMonth: 1,
      dayOfWeek: 0,
      advancedSpec: spec,
    };
  }
  if (dom === "*" && dow === "*") {
    return {
      mode: "day",
      minute: parseInt(min) || 0,
      hour: parseInt(hr) || 0,
      dayOfMonth: 1,
      dayOfWeek: 0,
      advancedSpec: spec,
    };
  }
  if (dom === "*" && dow !== "*") {
    return {
      mode: "week",
      minute: parseInt(min) || 0,
      hour: parseInt(hr) || 0,
      dayOfMonth: 1,
      dayOfWeek: parseInt(dow) || 0,
      advancedSpec: spec,
    };
  }
  if (dow === "*") {
    return {
      mode: "month",
      minute: parseInt(min) || 0,
      hour: parseInt(hr) || 0,
      dayOfMonth: parseInt(dom) || 1,
      dayOfWeek: 0,
      advancedSpec: spec,
    };
  }

  return { mode: "advanced", minute: 0, hour: 0, dayOfMonth: 1, dayOfWeek: 0, advancedSpec: spec };
}

function parsedToSpec(parsed: ParsedSpec): string {
  switch (parsed.mode) {
    case "minute":
      return "* * * * *";
    case "hour":
      return `${parsed.minute} * * * *`;
    case "day":
      return `${parsed.minute} ${parsed.hour} * * *`;
    case "week":
      return `${parsed.minute} ${parsed.hour} * * ${parsed.dayOfWeek}`;
    case "month":
      return `${parsed.minute} ${parsed.hour} ${parsed.dayOfMonth} * *`;
    case "advanced":
      return parsed.advancedSpec;
  }
}

function specToHuman(spec: string, locale: string): string {
  try {
    const cronLocale = locale === "zh" ? "zh_CN" : "en";
    return cronstrue.toString(spec, { locale: cronLocale });
  } catch {
    return spec;
  }
}

function isValidCronSpec(spec: string): boolean {
  if (!spec || spec.trim().split(/\s+/).length !== 5) return false;
  try {
    cronstrue.toString(spec);
    return true;
  } catch {
    return false;
  }
}

const WEEKDAYS = [
  "sunday",
  "monday",
  "tuesday",
  "wednesday",
  "thursday",
  "friday",
  "saturday",
] as const;

interface ScheduleEditorDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  job: CronJobItem | null;
  onSave: (spec: string) => Promise<void>;
}

export function ScheduleEditorDialog({
  open,
  onOpenChange,
  job,
  onSave,
}: ScheduleEditorDialogProps) {
  const { t, locale } = useI18n();
  // Base UI Select 只有传 items 时，触发按钮才渲染选项文案而不是原始枚举值
  const repeatItems: { value: RepeatMode; label: string }[] = [
    { value: "minute", label: t("schedule.every_minute") },
    { value: "hour", label: t("schedule.every_hour") },
    { value: "day", label: t("schedule.every_day") },
    { value: "week", label: t("schedule.every_week") },
    { value: "month", label: t("schedule.every_month") },
    { value: "advanced", label: t("schedule.advanced") },
  ];
  const weekdayItems = WEEKDAYS.map((day, index) => ({
    value: String(index),
    label: t(`schedule.${day}`),
  }));
  const [parsed, setParsed] = useState<ParsedSpec>({
    mode: "day",
    minute: 0,
    hour: 0,
    dayOfMonth: 1,
    dayOfWeek: 0,
    advancedSpec: "",
  });
  const [saving, setSaving] = useState(false);

  /* eslint-disable react-hooks/set-state-in-effect -- Reset parsed state when job changes */
  useEffect(() => {
    if (job) {
      setParsed(specToParsed(job.spec));
    }
  }, [job]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const currentSpec = useMemo(() => parsedToSpec(parsed), [parsed]);

  const humanReadable = useMemo(() => {
    if (parsed.mode === "advanced") {
      return isValidCronSpec(parsed.advancedSpec) ? specToHuman(parsed.advancedSpec, locale) : "";
    }
    return specToHuman(currentSpec, locale);
  }, [parsed, currentSpec, locale]);

  const canSave = useMemo(() => {
    if (parsed.mode === "advanced") return isValidCronSpec(parsed.advancedSpec);
    return isValidCronSpec(currentSpec);
  }, [parsed, currentSpec]);

  const specChanged = useMemo(() => {
    if (!job) return false;
    return currentSpec !== job.spec;
  }, [currentSpec, job]);

  const handleSave = useCallback(async () => {
    if (!canSave || !specChanged) return;
    setSaving(true);
    try {
      await onSave(currentSpec);
      onOpenChange(false);
    } finally {
      setSaving(false);
    }
  }, [canSave, specChanged, currentSpec, onSave, onOpenChange]);

  const updateParsed = useCallback((updates: Partial<ParsedSpec>) => {
    setParsed((prev) => ({ ...prev, ...updates }));
  }, []);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{t("schedule.title")}</DialogTitle>
          <DialogDescription>
            {t("schedule.description").replace("{name}", job?.name ?? "")}
          </DialogDescription>
        </DialogHeader>

        <FieldGroup className="gap-4 py-2">
          <Field>
            <FieldLabel htmlFor="schedule-repeat">{t("schedule.repeat")}</FieldLabel>
            <Select
              value={parsed.mode}
              items={repeatItems}
              onValueChange={(value) => updateParsed({ mode: String(value) as RepeatMode })}
            >
              <SelectTrigger id="schedule-repeat">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {repeatItems.map((item) => (
                  <SelectItem key={item.value} value={item.value}>
                    {item.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </Field>

          {parsed.mode !== "minute" && parsed.mode !== "advanced" && (
            <div className="grid grid-cols-2 gap-3">
              {(parsed.mode === "hour" ||
                parsed.mode === "day" ||
                parsed.mode === "week" ||
                parsed.mode === "month") && (
                <Field>
                  <FieldLabel htmlFor="schedule-minute">{t("schedule.minute")}</FieldLabel>
                  <Input
                    id="schedule-minute"
                    type="number"
                    min={0}
                    max={59}
                    value={parsed.minute}
                    onChange={(e) =>
                      updateParsed({
                        minute: Math.min(59, Math.max(0, parseInt(e.target.value) || 0)),
                      })
                    }
                  />
                </Field>
              )}

              {(parsed.mode === "day" || parsed.mode === "week" || parsed.mode === "month") && (
                <Field>
                  <FieldLabel htmlFor="schedule-hour">{t("schedule.hour")}</FieldLabel>
                  <Input
                    id="schedule-hour"
                    type="number"
                    min={0}
                    max={23}
                    value={parsed.hour}
                    onChange={(e) =>
                      updateParsed({
                        hour: Math.min(23, Math.max(0, parseInt(e.target.value) || 0)),
                      })
                    }
                  />
                </Field>
              )}

              {parsed.mode === "month" && (
                <Field>
                  <FieldLabel htmlFor="schedule-day-of-month">
                    {t("schedule.day_of_month")}
                  </FieldLabel>
                  <Input
                    id="schedule-day-of-month"
                    type="number"
                    min={1}
                    max={31}
                    value={parsed.dayOfMonth}
                    onChange={(e) =>
                      updateParsed({
                        dayOfMonth: Math.min(31, Math.max(1, parseInt(e.target.value) || 1)),
                      })
                    }
                  />
                </Field>
              )}

              {parsed.mode === "week" && (
                <Field className="col-span-2">
                  <FieldLabel htmlFor="schedule-day-of-week">
                    {t("schedule.day_of_week")}
                  </FieldLabel>
                  <Select
                    value={String(parsed.dayOfWeek)}
                    items={weekdayItems}
                    onValueChange={(value) => updateParsed({ dayOfWeek: parseInt(String(value)) })}
                  >
                    <SelectTrigger id="schedule-day-of-week">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {weekdayItems.map((item) => (
                        <SelectItem key={item.value} value={item.value}>
                          {item.label}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </Field>
              )}
            </div>
          )}

          {parsed.mode === "advanced" && (
            <Field>
              <FieldLabel htmlFor="schedule-cron">{t("schedule.cron_expression")}</FieldLabel>
              <Input
                id="schedule-cron"
                placeholder="*/5 * * * *"
                value={parsed.advancedSpec}
                onChange={(e) => updateParsed({ advancedSpec: e.target.value })}
                className="font-mono"
              />
            </Field>
          )}

          {humanReadable && (
            <div className="rounded-md bg-muted/50 px-3 py-2">
              <p className="text-sm text-muted-foreground">
                <span className="font-medium text-foreground">{humanReadable}</span>
              </p>
              <p className="mt-1 font-mono text-xs text-muted-foreground">{currentSpec}</p>
            </div>
          )}
        </FieldGroup>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t("schedule.cancel")}
          </Button>
          <Button onClick={handleSave} disabled={!canSave || !specChanged || saving}>
            {saving ? t("schedule.saving") : t("schedule.save")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
