"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import cronstrue from "cronstrue";
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
import { Label } from "@/components/ui/label";
import { useT } from "@/lib/i18n";

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
    return { mode: "advanced", minute: 0, hour: 0, dayOfMonth: 1, dayOfWeek: 0, advancedSpec: spec };
  }

  const [min, hr, dom, , dow] = parts;

  if (min === "*" && hr === "*" && dom === "*" && dow === "*") {
    return { mode: "minute", minute: 0, hour: 0, dayOfMonth: 1, dayOfWeek: 0, advancedSpec: spec };
  }
  if (hr === "*" && dom === "*" && dow === "*") {
    return { mode: "hour", minute: parseInt(min) || 0, hour: 0, dayOfMonth: 1, dayOfWeek: 0, advancedSpec: spec };
  }
  if (dom === "*" && dow === "*") {
    return { mode: "day", minute: parseInt(min) || 0, hour: parseInt(hr) || 0, dayOfMonth: 1, dayOfWeek: 0, advancedSpec: spec };
  }
  if (dom === "*" && dow !== "*") {
    return { mode: "week", minute: parseInt(min) || 0, hour: parseInt(hr) || 0, dayOfMonth: 1, dayOfWeek: parseInt(dow) || 0, advancedSpec: spec };
  }
  if (dow === "*") {
    return { mode: "month", minute: parseInt(min) || 0, hour: parseInt(hr) || 0, dayOfMonth: parseInt(dom) || 1, dayOfWeek: 0, advancedSpec: spec };
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

function specToHuman(spec: string): string {
  try {
    return cronstrue.toString(spec, { locale: "en" });
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

const WEEKDAYS = ["sunday", "monday", "tuesday", "wednesday", "thursday", "friday", "saturday"] as const;

interface ScheduleEditorDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  job: CronJobItem | null;
  onSave: (spec: string) => Promise<void>;
}

export function ScheduleEditorDialog({ open, onOpenChange, job, onSave }: ScheduleEditorDialogProps) {
  const t = useT();
  const [parsed, setParsed] = useState<ParsedSpec>({
    mode: "day", minute: 0, hour: 0, dayOfMonth: 1, dayOfWeek: 0, advancedSpec: "",
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
      return isValidCronSpec(parsed.advancedSpec) ? specToHuman(parsed.advancedSpec) : "";
    }
    return specToHuman(currentSpec);
  }, [parsed, currentSpec]);

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

        <div className="space-y-4 py-2">
          <div className="space-y-2">
            <Label>{t("schedule.repeat")}</Label>
            <Select
              value={parsed.mode}
              onValueChange={(value) => updateParsed({ mode: String(value) as RepeatMode })}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="minute">{t("schedule.every_minute")}</SelectItem>
                <SelectItem value="hour">{t("schedule.every_hour")}</SelectItem>
                <SelectItem value="day">{t("schedule.every_day")}</SelectItem>
                <SelectItem value="week">{t("schedule.every_week")}</SelectItem>
                <SelectItem value="month">{t("schedule.every_month")}</SelectItem>
                <SelectItem value="advanced">{t("schedule.advanced")}</SelectItem>
              </SelectContent>
            </Select>
          </div>

          {parsed.mode !== "minute" && parsed.mode !== "advanced" && (
            <div className="grid grid-cols-2 gap-3">
              {(parsed.mode === "hour" || parsed.mode === "day" || parsed.mode === "week" || parsed.mode === "month") && (
                <div className="space-y-2">
                  <Label>{t("schedule.minute")}</Label>
                  <Input
                    type="number"
                    min={0}
                    max={59}
                    value={parsed.minute}
                    onChange={(e) => updateParsed({ minute: Math.min(59, Math.max(0, parseInt(e.target.value) || 0)) })}
                  />
                </div>
              )}

              {(parsed.mode === "day" || parsed.mode === "week" || parsed.mode === "month") && (
                <div className="space-y-2">
                  <Label>{t("schedule.hour")}</Label>
                  <Input
                    type="number"
                    min={0}
                    max={23}
                    value={parsed.hour}
                    onChange={(e) => updateParsed({ hour: Math.min(23, Math.max(0, parseInt(e.target.value) || 0)) })}
                  />
                </div>
              )}

              {parsed.mode === "month" && (
                <div className="space-y-2">
                  <Label>{t("schedule.day_of_month")}</Label>
                  <Input
                    type="number"
                    min={1}
                    max={31}
                    value={parsed.dayOfMonth}
                    onChange={(e) => updateParsed({ dayOfMonth: Math.min(31, Math.max(1, parseInt(e.target.value) || 1)) })}
                  />
                </div>
              )}

              {parsed.mode === "week" && (
                <div className="space-y-2 col-span-2">
                  <Label>{t("schedule.day_of_week")}</Label>
                  <Select
                    value={String(parsed.dayOfWeek)}
                    onValueChange={(value) => updateParsed({ dayOfWeek: parseInt(String(value)) })}
                  >
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {WEEKDAYS.map((d, i) => (
                        <SelectItem key={i} value={String(i)}>
                          {t(`schedule.${d}`)}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              )}
            </div>
          )}

          {parsed.mode === "advanced" && (
            <div className="space-y-2">
              <Label>{t("schedule.cron_expression")}</Label>
              <Input
                placeholder="*/5 * * * *"
                value={parsed.advancedSpec}
                onChange={(e) => updateParsed({ advancedSpec: e.target.value })}
                className="font-mono"
              />
            </div>
          )}

          {humanReadable && (
            <div className="rounded-md bg-muted/50 px-3 py-2">
              <p className="text-sm text-muted-foreground">
                <span className="font-medium text-foreground">{humanReadable}</span>
              </p>
              <p className="mt-1 font-mono text-xs text-muted-foreground">{currentSpec}</p>
            </div>
          )}
        </div>

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
