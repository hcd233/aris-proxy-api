"use client";

import { useCallback, useEffect, useState } from "react";
import { useAuth } from "@/lib/auth-context";
import type { OAuth2Provider } from "@/lib/types";

export default function LoginPage() {
  const { login, handleCallback, accessToken, user } = useAuth();
  const [error, setError] = useState<string | null>(null);
  const [processing, setProcessing] = useState(false);

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
        setError("Login failed: no tokens received");
        setProcessing(false);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Login failed");
      setProcessing(false);
    }
  }, [handleCallback]);

  /* eslint-disable react-hooks/set-state-in-effect -- OAuth2 callback requires setting state from URL params on mount */
  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    if (params.get("code") && params.get("state")) {
      processCallback();
    }
  }, [processCallback]);
  /* eslint-enable react-hooks/set-state-in-effect */

  // Redirect to home if already authenticated
  useEffect(() => {
    if (accessToken && user) {
      window.location.href = "/web/";
    }
  }, [accessToken, user]);

  if (processing) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <p className="text-muted-foreground">Processing login...</p>
      </div>
    );
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-background">
      <div className="flex flex-col items-center gap-8 px-8">
        <h1 className="text-3xl font-bold tracking-tight text-foreground">
          Aris Proxy API
        </h1>
        <p className="text-muted-foreground">Sign in to continue</p>

        {error && (
          <div className="rounded-md bg-destructive/10 px-4 py-3 text-sm text-destructive">
            {error}
          </div>
        )}

        <div className="flex flex-col gap-3">
          <button
            onClick={() => login("github")}
            className="flex h-11 w-64 items-center justify-center rounded-md border border-border bg-background px-4 text-sm font-medium text-foreground transition-colors hover:bg-accent"
          >
            Sign in with GitHub
          </button>
          <button
            onClick={() => login("google")}
            className="flex h-11 w-64 items-center justify-center rounded-md border border-border bg-background px-4 text-sm font-medium text-foreground transition-colors hover:bg-accent"
          >
            Sign in with Google
          </button>
        </div>
      </div>
    </div>
  );
}
