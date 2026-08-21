"use client";

import { PermissionGuard } from "@/components/permission-guard";
import { DemoConfigCard } from "@/components/demo-config-card";
import { DemoSessionsManager } from "@/components/demo-sessions-manager";
import { PageHeader } from "@/components/page-header";
import { useT } from "@/lib/i18n";

export default function DemoPage() {
  const t = useT();

  return (
    <PermissionGuard adminOnly>
      <div className="space-y-8">
        <PageHeader title={t("demo.title")} description={t("demo.subtitle")} />
        <DemoConfigCard />
        <DemoSessionsManager />
      </div>
    </PermissionGuard>
  );
}
