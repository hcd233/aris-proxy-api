"use client";

import { useCallback, useEffect, useState } from "react";
import { useAuth } from "@/lib/auth-context";
import type { OAuth2Provider } from "@/lib/types";

export default function CallbackPage() {
  const { handleCallback } = useAuth();
  const [error, setError] = useState<string | null>(null);

  const processCallback = useCallback(async () => {
    const params = new URLSearchParams(window.location.search);
    const code = params.get("code");
    const state = params.get("state");

    if (!code || !state) {
      setError("Missing authorization code or state parameter");
      return;
    }

    const platformMatch = state.match(/^provider:(github|google):/);
    const platform: OAuth2Provider = platformMatch
      ? (platformMatch[1] as OAuth2Provider)
      : "github";

    try {
      const { api } = await import("@/lib/api-client");
      const rsp = await api.oauth2Callback({ platform, code, state });
      if (rsp.error) {
        setError(rsp.error.message);
        return;
      }
      if (rsp.accessToken && rsp.refreshToken) {
        await handleCallback(rsp.accessToken, rsp.refreshToken);
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
      <div className="flex min-h-screen items-center justify-center">
        <div className="text-center">
          <h1 className="text-2xl font-bold text-destructive">Login Failed</h1>
          <p className="mt-2 text-muted-foreground">{error}</p>
          <a
            href="/web/login/"
            className="mt-4 inline-block text-primary hover:underline"
          >
            Back to login
          </a>
        </div>
      </div>
    );
  }

  return (
    <div className="flex min-h-screen items-center justify-center">
      <p className="text-muted-foreground">Completing login...</p>
    </div>
  );
}
