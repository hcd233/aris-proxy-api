"use client";

import { useCallback, useEffect, useState } from "react";
import { useAuth } from "@/lib/auth-context";

export default function CallbackPage() {
  const { handleCallback } = useAuth();
  const [error, setError] = useState<string | null>(null);

  const processCallback = useCallback(async () => {
    const params = new URLSearchParams(window.location.search);
    const code = params.get("code");

    if (!code) {
      setError("Missing authorization code");
      return;
    }

    try {
      const res = await fetch("/api/v1/oauth2/exchange-code", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ code }),
      });

      if (!res.ok) {
        const data = await res.json().catch(() => ({ error: "Exchange failed" }));
        setError(data.error || "Token exchange failed");
        return;
      }

      const data = await res.json();
      if (data.accessToken && data.refreshToken) {
        await handleCallback(data.accessToken, data.refreshToken);
        window.location.href = "/web/";
      } else {
        setError("Login failed: no tokens received");
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Login failed");
    }
  }, [handleCallback]);

  /* eslint-disable react-hooks/set-state-in-effect -- OAuth2 callback requires setting state from URL params on mount */
  useEffect(() => {
    processCallback();
  }, [processCallback]);
  /* eslint-enable react-hooks/set-state-in-effect */

  if (error) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-background px-4">
        <div className="w-full max-w-sm rounded-xl border border-border bg-card p-8 text-center">
          <h1 className="font-display text-2xl font-semibold text-destructive">Login Failed</h1>
          <p className="mt-3 text-sm text-muted-foreground">{error}</p>
          <a
            href="/web/login/"
            className="mt-6 inline-block text-sm font-medium text-primary hover:text-[var(--primary-hover)] transition-colors"
          >
            Back to login
          </a>
        </div>
      </div>
    );
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-background">
      <div className="text-center">
        <p className="font-display text-xl font-semibold text-foreground">Completing login...</p>
        <p className="mt-2 text-sm text-muted-foreground">Please wait a moment</p>
      </div>
    </div>
  );
}
