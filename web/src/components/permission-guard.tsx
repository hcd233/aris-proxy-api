"use client";

import { useEffect, type ReactNode } from "react";
import { ShieldAlert } from "lucide-react";
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from "@/components/ui/empty";
import { useAuth } from "@/lib/auth-context";
import { useT } from "@/lib/i18n";
import type { DemoModule } from "@/lib/types";

interface PermissionGuardProps {
  children: ReactNode;
  adminOnly?: boolean;
  /** 页面对应的 demo 模块 key；demo 用户仅当该模块开放时放行 */
  module?: DemoModule;
  /** demo 用户直接放行（仅用于 dashboard 布局壳，页面内容各自按 module 守卫） */
  allowDemo?: boolean;
}

function GuardState({ title, description }: { title: string; description: string }) {
  return (
    <div className="page-surface flex min-h-screen items-center justify-center bg-background px-4">
      <Empty className="max-w-md">
        <EmptyHeader>
          <EmptyMedia variant="icon">
            <ShieldAlert />
          </EmptyMedia>
          <EmptyTitle>{title}</EmptyTitle>
          <EmptyDescription>{description}</EmptyDescription>
        </EmptyHeader>
      </Empty>
    </div>
  );
}

export function PermissionGuard({
  children,
  adminOnly = false,
  module,
  allowDemo = false,
}: PermissionGuardProps) {
  const { user, isLoading, isUser, isAdmin, isDemo, isModuleOpen } = useAuth();
  const t = useT();

  useEffect(() => {
    if (!isLoading && !user) {
      window.location.href = "/web/login/";
    }
  }, [isLoading, user]);

  if (isLoading) {
    return (
      <GuardState
        title={t("permission_guard.loading")}
        description={t("permission_guard.preparing")}
      />
    );
  }

  if (!user) {
    // Redirecting to login
    return null;
  }

  if (isDemo()) {
    // demo 只读受限：布局壳直接放行；页面按模块开放判断（module 未声明视为不开放）
    if (allowDemo || (module && isModuleOpen(module))) {
      return <>{children}</>;
    }
    return (
      <GuardState
        title={t("permission_guard.access_denied")}
        description={t("permission_guard.demo_denied_desc")}
      />
    );
  }

  if (user.permission === "pending") {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="text-center">
          <h1 className="text-2xl font-bold">{t("permission_guard.access_pending")}</h1>
          <p className="mt-2 text-muted-foreground">{t("permission_guard.access_pending_desc")}</p>
        </div>
      </div>
    );
  }

  if (adminOnly && !isAdmin()) {
    return (
      <GuardState
        title={t("permission_guard.access_denied")}
        description={t("permission_guard.access_denied_desc")}
      />
    );
  }

  if (!isUser()) {
    return null;
  }

  return <>{children}</>;
}
