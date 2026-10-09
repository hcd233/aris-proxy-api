"use client";

import Link from "next/link";
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Sheet, SheetContent, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import type { APIKeyItem } from "@/lib/types";
import type {
  PlaygroundModelOption,
  PlaygroundParams,
} from "@/app/(dashboard)/playground/playground-logic";
import { useT } from "@/lib/i18n";

interface ParamsDrawerProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  models: PlaygroundModelOption[];
  keys: APIKeyItem[];
  keysLoaded: boolean;
  params: PlaygroundParams;
  onParamsChange: (patch: Partial<PlaygroundParams>) => void;
}

export function ParamsDrawer({
  open,
  onOpenChange,
  models,
  keys,
  keysLoaded,
  params,
  onParamsChange,
}: ParamsDrawerProps) {
  const t = useT();
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side="right" className="flex w-[21rem] flex-col gap-4 sm:max-w-[21rem]">
        <SheetHeader>
          <SheetTitle>{t("playground.params.title")}</SheetTitle>
        </SheetHeader>

        <div className="space-y-1">
          <Label htmlFor="pg-model">{t("playground.model")}</Label>
          <Select value={params.model} onValueChange={(v) => onParamsChange({ model: String(v) })}>
            <SelectTrigger id="pg-model" className="w-full">
              <SelectValue placeholder={t("playground.model.placeholder")} />
            </SelectTrigger>
            <SelectContent>
              {models.map((m) => (
                <SelectItem key={m.alias} value={m.alias}>
                  {m.alias}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <div className="space-y-1">
          <Label htmlFor="pg-apikey">{t("playground.api_key")}</Label>
          {keysLoaded && keys.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              {t("playground.api_key.empty")}{" "}
              <Link href="/apikeys/" className="underline underline-offset-2">
                {t("playground.api_key.create")}
              </Link>
            </p>
          ) : (
            <Select
              value={params.apiKeyID === null ? "" : String(params.apiKeyID)}
              onValueChange={(v) => onParamsChange({ apiKeyID: Number(v) })}
            >
              <SelectTrigger id="pg-apikey" className="w-full">
                <SelectValue placeholder={t("playground.api_key.placeholder")} />
              </SelectTrigger>
              <SelectContent>
                {keys.map((k) => (
                  <SelectItem key={k.id} value={String(k.id)}>
                    {k.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
          <p className="text-[11px] text-muted-foreground">{t("playground.api_key.hint")}</p>
        </div>

        <div className="grid grid-cols-2 gap-2">
          <div className="space-y-1">
            <Label htmlFor="pg-temperature">{t("playground.temperature")}</Label>
            <Input
              id="pg-temperature"
              type="number"
              min={0}
              max={2}
              step={0.1}
              value={params.temperature}
              onChange={(e) => onParamsChange({ temperature: e.target.value })}
            />
          </div>
          <div className="space-y-1">
            <Label htmlFor="pg-max-tokens">{t("playground.max_tokens")}</Label>
            <Input
              id="pg-max-tokens"
              type="number"
              min={1}
              value={params.maxTokens}
              onChange={(e) => onParamsChange({ maxTokens: e.target.value })}
            />
          </div>
        </div>

        <div className="flex items-center gap-2">
          <Switch
            checked={params.stream}
            onCheckedChange={(v) => onParamsChange({ stream: v })}
            id="pg-stream"
          />
          <Label htmlFor="pg-stream">{t("playground.stream")}</Label>
        </div>
      </SheetContent>
    </Sheet>
  );
}
