"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api-client";
import { showErrorToast } from "@/lib/api-error-handler";
import { DEMO_MODULES, normalizeDemoModules } from "@/lib/demo-modules";
import { useT } from "@/lib/i18n";
import type { DemoConfig, DemoModule } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Field, FieldLabel, FieldLegend, FieldSet, FieldTitle } from "@/components/ui/field";
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
      <CardContent className="flex flex-col gap-5">
        <p className="text-sm text-muted-foreground">{t("demo.config_desc")}</p>

        <Field orientation="horizontal" className="w-auto">
          <Switch
            id="demo-login-enabled"
            checked={config.loginEnabled}
            onCheckedChange={(checked) =>
              setConfig((prev) => (prev ? { ...prev, loginEnabled: checked } : prev))
            }
          />
          <FieldLabel htmlFor="demo-login-enabled" className="min-w-20">
            {t("demo.login_enabled")}
          </FieldLabel>
        </Field>

        <FieldSet>
          <FieldLegend variant="label">{t("demo.modules")}</FieldLegend>
          <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
            {DEMO_MODULES.map((module) => (
              <FieldLabel key={module} htmlFor={`demo-module-${module}`}>
                <Field orientation="horizontal">
                  <Checkbox
                    id={`demo-module-${module}`}
                    checked={config.modules.includes(module)}
                    onCheckedChange={(checked) => toggleModule(module, checked)}
                  />
                  <FieldTitle>{t(`demo.module_${module}`)}</FieldTitle>
                </Field>
              </FieldLabel>
            ))}
          </div>
        </FieldSet>

        <div className="flex justify-end">
          <Button onClick={save} disabled={saving}>
            {saving ? t("common.processing") : t("demo.save")}
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
