"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api-client";
import { showErrorToast } from "@/lib/api-error-handler";
import { DEMO_MODULES, normalizeDemoModules } from "@/lib/demo-modules";
import { useT } from "@/lib/i18n";
import type { DemoConfig, DemoModule } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { toast } from "sonner";

/** Demo 演示配置卡片：admin 配置 demo 登录开关与开放模块 */
export function DemoConfigCard() {
  const t = useT();
  const [config, setConfig] = useState<DemoConfig | null>(null);
  const [saving, setSaving] = useState(false);

  const fetchConfig = useCallback(async () => {
    try {
      const rsp = await api.getDemoConfig();
      setConfig(
        rsp.config ? { ...rsp.config, modules: normalizeDemoModules(rsp.config.modules) } : null,
      );
    } catch (err) {
      showErrorToast(err, { title: t("demo.load_error") });
    }
  }, [t]);

  /* eslint-disable react-hooks/set-state-in-effect -- Config loading requires setting state from async fetch on mount */
  useEffect(() => {
    fetchConfig();
  }, [fetchConfig]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const toggleModule = (module: DemoModule, open: boolean) => {
    setConfig((prev) => {
      if (!prev) return prev;
      const modules = open
        ? [...new Set([...prev.modules, module])]
        : prev.modules.filter((m) => m !== module);
      return { ...prev, modules };
    });
  };

  const save = useCallback(async () => {
    if (!config) return;
    setSaving(true);
    try {
      const rsp = await api.updateDemoConfig({
        config: {
          loginEnabled: config.loginEnabled,
          modules: config.modules,
        },
      });
      if (rsp.error) {
        throw new Error(rsp.error.message);
      }
      setConfig(rsp.config ?? config);
      toast.success(t("demo.save_success"));
    } catch (err) {
      showErrorToast(err, { title: t("demo.save_error") });
    } finally {
      setSaving(false);
    }
  }, [config, t]);

  if (!config) {
    return null;
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="font-display">{t("demo.config_title")}</CardTitle>
      </CardHeader>
      <CardContent className="space-y-5">
        <p className="text-sm text-muted-foreground">{t("demo.config_desc")}</p>

        <div className="flex flex-wrap items-center gap-6">
          <div className="flex items-center gap-3">
            <Switch
              id="demo-login-enabled"
              checked={config.loginEnabled}
              onCheckedChange={(checked) =>
                setConfig((prev) => (prev ? { ...prev, loginEnabled: checked } : prev))
              }
            />
            <Label htmlFor="demo-login-enabled" className="text-sm min-w-20">
              {t("demo.login_enabled")}
            </Label>
          </div>
        </div>

        <div className="space-y-2">
          <p className="text-sm font-medium">{t("demo.modules")}</p>
          <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
            {DEMO_MODULES.map((module) => (
              <label
                key={module}
                className="flex cursor-pointer items-center gap-2.5 rounded-lg border border-border/70 px-3 py-2 text-sm transition-colors hover:bg-secondary/50"
              >
                <input
                  type="checkbox"
                  className="size-4 accent-[var(--primary)]"
                  checked={config.modules.includes(module)}
                  onChange={(e) => toggleModule(module, e.target.checked)}
                />
                <span>{t(`demo.module_${module}`)}</span>
              </label>
            ))}
          </div>
        </div>

        <div className="flex justify-end">
          <Button onClick={save} disabled={saving}>
            {saving ? t("common.processing") : t("demo.save")}
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
