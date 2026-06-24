"use client";

import { useEffect, type ReactNode } from "react";
import { useAuth } from "@/lib/auth-context";
import { useT } from "@/lib/i18n";

interface PermissionGuardProps {
  children: ReactNode;
  adminOnly?: boolean;
}

function GuardState({
  title,
  description,
}: {
  title: string;
  description: string;
}) {
  return (
    <div className="flex min-h-screen items-center justify-center bg-background px-4">
      <div className="w-full max-w-md rounded-3xl border bg-card p-8 text-center shadow-[0_24px_70px_rgba(92,62,29,0.14)]">
        <h1 className="font-display text-4xl font-bold tracking-tight text-foreground">
          {title}
        </h1>
        <p className="mt-3 text-sm leading-6 text-muted-foreground">
          {description}
        </p>
      </div>
    </div>
  );
}

export function PermissionGuard({
  children,
  adminOnly = false,
}: PermissionGuardProps) {
  const { user, isLoading, isUser, isAdmin } = useAuth();
  const t = useT();

  useEffect(() => {
    if (!isLoading && !user) {
      window.location.href = "/web/login/";
    }
  }, [isLoading, user]);

  if (isLoading) {
    return <GuardState title={t("permission_guard.loading")} description={t("permission_guard.preparing")} />;
  }

  if (!user) {
    // Redirecting to login
    return null;
  }

  if (user.permission === "pending") {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="text-center">
          <h1 className="text-2xl font-bold">{t("permission_guard.access_pending")}</h1>
          <p className="mt-2 text-muted-foreground">
            {t("permission_guard.access_pending_desc")}
          </p>
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