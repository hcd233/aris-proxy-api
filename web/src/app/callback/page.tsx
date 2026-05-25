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
