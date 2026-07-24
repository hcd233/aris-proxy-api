"use client";

import { useCallback, useEffect, useState } from "react";
import { useAuth } from "@/lib/auth-context";
import { useT } from "@/lib/i18n";
import type { OAuth2Provider } from "@/lib/types";

export default function CallbackPage() {
  const { handleCallback } = useAuth();
  const t = useT();
  const [error, setError] = useState<string | null>(null);

  const processCallback = useCallback(async () => {
    const params = new URLSearchParams(window.location.search);
    const code = params.get("code");
    const state = params.get("state");

    if (!code || !state) {
      setError(t("callback.missing_params"));
      return;
    }

    const platformMatch = state.match(/^provider:(github|google):/);
    const platform: OAuth2Provider = platformMatch
      ? (platformMatch[1] as OAuth2Provider)
      : "github";

    try {
      const { api } = await import("@/lib/api-client");
      const data = await api.oauth2Callback({ platform, code, state });

      if (data.error) {
        setError(data.error.message || t("callback.token_exchange_failed"));
        return;
      }

      if (data.accessToken && data.refreshToken) {
        await handleCallback(data.accessToken, data.refreshToken);
        window.location.href = "/web/";
      } else {
        setError(t("callback.no_tokens"));
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : t("callback.token_exchange_failed"));
    }
  }, [handleCallback]);

  /* eslint-disable react-hooks/set-state-in-effect -- OAuth2 callback requires setting state from URL params on mount */
  useEffect(() => {
    processCallback();
  }, [processCallback]);
  /* eslint-enable react-hooks/set-state-in-effect */

  if (error) {
    return (
      <div className="page-surface flex min-h-screen items-center justify-center bg-background px-4">
        <div className="w-full max-w-sm rounded-xl border border-border bg-card p-8 text-center">
          <h1 className="font-display text-2xl font-semibold text-destructive">{t("callback.login_failed")}</h1>
          <p className="mt-3 text-sm text-muted-foreground">{error}</p>
          <a
            href="/web/login/"
            className="mt-6 inline-block text-sm font-medium text-primary hover:text-[var(--primary-hover)] transition-colors"
          >
            {t("callback.back_to_login")}
          </a>
        </div>
      </div>
    );
  }

  return (
    <div className="page-surface flex min-h-screen items-center justify-center bg-background">
      <div className="text-center">
        <p className="font-display text-xl font-semibold text-foreground">{t("callback.completing")}</p>
        <p className="mt-2 text-sm text-muted-foreground">{t("callback.please_wait")}</p>
      </div>
    </div>
  );
}
