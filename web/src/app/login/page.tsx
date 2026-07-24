"use client";

import { useCallback, useEffect, useState } from "react";
import { useAuth } from "@/lib/auth-context";
import { useT } from "@/lib/i18n";
import { Button } from "@/components/ui/button";
import type { OAuth2Provider } from "@/lib/types";

export default function LoginPage() {
  const { login, handleCallback, accessToken, user } = useAuth();
  const [error, setError] = useState<string | null>(null);
  const [processing, setProcessing] = useState(false);
  const t = useT();

  const processCallback = useCallback(async () => {
    const params = new URLSearchParams(window.location.search);
    const code = params.get("code");
    const state = params.get("state");

    if (!code || !state) return;

    setProcessing(true);
    const platformMatch = state.match(/^provider:(github|google):/);
    const platform: OAuth2Provider = platformMatch
      ? (platformMatch[1] as OAuth2Provider)
      : "github";

    try {
      const { api } = await import("@/lib/api-client");
      const rsp = await api.oauth2Callback({ platform, code, state });
      if (rsp.error) {
        setError(rsp.error.message);
        setProcessing(false);
        return;
      }
      if (rsp.accessToken && rsp.refreshToken) {
        await handleCallback(rsp.accessToken, rsp.refreshToken);
        window.location.href = "/web/";
      } else {
        setError(t("login.failed_no_tokens"));
        setProcessing(false);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : t("login.failed"));
      setProcessing(false);
    }
    }, [handleCallback, t]);

  /* eslint-disable react-hooks/set-state-in-effect -- OAuth2 callback requires setting state from URL params on mount */
  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    if (params.get("code") && params.get("state")) {
      processCallback();
    }
  }, [processCallback]);
  /* eslint-enable react-hooks/set-state-in-effect */

  useEffect(() => {
    if (accessToken && user) {
      window.location.href = "/web/";
    }
  }, [accessToken, user]);

  if (processing) {
    return (
      <div className="page-surface flex min-h-screen items-center justify-center bg-background px-4">
        <div className="w-full max-w-sm rounded-xl border border-border/70 bg-card p-8 text-center shadow-lg">
          <p className="font-display text-3xl font-semibold text-foreground">
            Aris Proxy
          </p>
          <p className="mt-3 text-sm text-muted-foreground">
            {t("common.loading")}
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="page-surface flex min-h-screen items-center justify-center bg-background px-4 py-10">
      <div className="grid w-full max-w-5xl overflow-hidden rounded-2xl border border-border/70 bg-card shadow-xl md:grid-cols-[1.05fr_0.95fr]">
        <div className="hidden bg-sidebar p-10 text-sidebar-foreground md:flex md:flex-col md:justify-between bg-[radial-gradient(120%_90%_at_0%_0%,color-mix(in_oklab,var(--sidebar-primary)_14%,transparent),transparent_55%)]">
          <div>
            <div className="mb-10 inline-flex size-14 items-center justify-center rounded-xl bg-sidebar-primary font-display text-3xl font-semibold text-sidebar-primary-foreground shadow-sm">
              A
            </div>
            <h1 className="font-display text-5xl font-semibold leading-none tracking-tight">
              Aris Proxy
            </h1>
            <p className="mt-4 max-w-sm text-sm leading-6 text-sidebar-foreground/70">
              {t("auth.login_subtitle")}
            </p>
          </div>
          <p className="text-xs text-sidebar-foreground/50">
            {t("login.secure_access")}
          </p>
        </div>

        <div className="flex flex-col justify-center p-8 md:p-10">
          <div className="mb-8 md:hidden">
            <h1 className="font-display text-5xl font-semibold tracking-tight text-foreground">
              Aris Proxy
            </h1>
            <p className="mt-2 text-sm text-muted-foreground">
              {t("auth.login_subtitle")}
            </p>
          </div>
          <div className="hidden md:block">
            <h2 className="font-display text-4xl font-semibold tracking-tight text-foreground">
              {t("login.welcome_back")}
            </h2>
            <p className="mt-2 text-sm text-muted-foreground">
              {t("login.sign_in")}
            </p>
          </div>

          {error && (
            <div className="mt-6 rounded-xl border border-destructive/20 bg-destructive/10 px-4 py-3 text-sm text-destructive">
              {error}
            </div>
          )}

          <div className="mt-8 flex flex-col gap-3">
            <Button size="lg" onClick={() => login("github")}>
              {t("auth.login_github")}
            </Button>
            <Button size="lg" variant="outline" onClick={() => login("google")}>
              {t("auth.login_google")}
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
}
