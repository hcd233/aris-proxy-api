"use client";

import { useEffect, type ReactNode } from "react";
import { useAuth } from "@/lib/auth-context";

interface PermissionGuardProps {
  children: ReactNode;
  adminOnly?: boolean;
}

export function PermissionGuard({
  children,
  adminOnly = false,
}: PermissionGuardProps) {
  const { user, isLoading, isUser, isAdmin } = useAuth();

  useEffect(() => {
    if (!isLoading && !user) {
      window.location.href = "/web/login/";
    }
  }, [isLoading, user]);

  if (isLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <p className="text-muted-foreground">Loading...</p>
      </div>
    );
  }

  if (!user) {
    // Redirecting to login
    return null;
  }

  if (user.permission === "pending") {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="text-center">
          <h1 className="text-2xl font-bold">Access Pending</h1>
          <p className="mt-2 text-muted-foreground">
            Your account is awaiting approval from an administrator.
          </p>
        </div>
      </div>
    );
  }

  if (adminOnly && !isAdmin()) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="text-center">
          <h1 className="text-2xl font-bold">Access Denied</h1>
          <p className="mt-2 text-muted-foreground">
            You need administrator privileges to access this page.
          </p>
        </div>
      </div>
    );
  }

  if (!isUser()) {
    return null;
  }

  return <>{children}</>;
}