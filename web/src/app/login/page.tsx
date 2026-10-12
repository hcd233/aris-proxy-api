"use client";

import { useCallback, useEffect, useState } from "react";
import { useAuth } from "@/lib/auth-context";
import { useT } from "@/lib/i18n";
import { api } from "@/lib/api-client";
import { parseOAuthCallbackParams, resolveOAuthPlatform } from "@/lib/oauth";
import type { OAuth2Provider } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { SplitText } from "@/components/motion/split-text";

/* Runs during HTML parsing, before the first animated frame: marks repeat
 * visits in this browser session so the choreography plays at 40% length
 * (html[data-login-seen] .login-stage → --login-pace: 0.4). */
const LOGIN_SEEN_SCRIPT =
  "(function(){try{var k='motion.login.seen';if(sessionStorage.getItem(k))" +
  "document.documentElement.dataset.loginSeen='1';sessionStorage.setItem(k,'1');}catch(e){}})();";

const PENDING_RESET_MS = 4000;

export default function LoginPage() {
  const { login, handleCallback, accessToken, user } = useAuth();
  const [error, setError] = useState<string | null>(null);
  const [processing, setProcessing] = useState(false);
  const [demoAvailable, setDemoAvailable] = useState(false);
  const [pending, setPending] = useState<OAuth2Provider | null>(null);
  const t = useT();

  const startOAuth = (provider: OAuth2Provider) => {
    setPending(provider);
    // The redirect is never delayed for the effect. login() resolves once
    // navigation has started (or failed silently), so the waiting cue stays
    // up while the provider page loads and only clears if we are still here.
    // login() is typed `void` in the auth context but is async at runtime.
    void Promise.resolve(login(provider)).finally(() =>
      window.setTimeout(() => setPending(null), PENDING_RESET_MS),
    );
  };

  const processCallback = useCallback(async () => {
    const params = parseOAuthCallbackParams(window.location.search);
    if (!params) return;

    setProcessing(true);
    const platform = resolveOAuthPlatform(params.state);
    if (!platform) {
      setError(t("login.invalid_state"));
      setProcessing(false);
      return;
    }

    try {
      const rsp = await api.oauth2Callback({
        platform,
        code: params.code,
        state: params.state,
      });
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

  useEffect(() => {
    // demo 入口显隐：开关开启且已配置 demo 账户时展示按钮
    api
      .getDemoStatus()
      .then((rsp) => {
        setDemoAvailable(rsp.loginEnabled && rsp.demoUserExists);
      })
      .catch(() => setDemoAvailable(false));
  }, []);
  /* eslint-enable react-hooks/set-state-in-effect */

  useEffect(() => {
    if (accessToken && user) {
      window.location.href = "/web/";
    }
  }, [accessToken, user]);

  const demoLogin = useCallback(async () => {
    setError(null);
    setProcessing(true);
    try {
      const rsp = await api.demoLogin();
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

  if (processing) {
    return (
      <div className="page-surface flex min-h-screen items-center justify-center bg-background px-4">
        <div className="animate-scale-in w-full max-w-sm rounded-xl border border-border/70 bg-card p-8 text-center shadow-lg">
          <p className="font-display text-3xl font-semibold text-foreground">Aris Proxy</p>
          <p className="mt-3 text-sm text-muted-foreground">{t("common.loading")}</p>
        </div>
      </div>
    );
  }

  return (
    <div className="login-stage page-surface flex min-h-screen items-center justify-center bg-background px-4 py-10">
      <script dangerouslySetInnerHTML={{ __html: LOGIN_SEEN_SCRIPT }} />
      <div className="login-card relative grid w-full max-w-5xl overflow-hidden rounded-2xl border border-border/70 bg-card shadow-xl md:grid-cols-[1.05fr_0.95fr]">
        <div className="hidden bg-sidebar p-10 text-sidebar-foreground md:flex md:flex-col md:justify-between bg-[radial-gradient(120%_90%_at_0%_0%,color-mix(in_oklab,var(--sidebar-primary)_14%,transparent),transparent_55%)]">
          <div>
            <div className="login-mark mb-10 inline-flex size-14 items-center justify-center rounded-xl bg-sidebar-primary font-display text-3xl font-semibold text-sidebar-primary-foreground shadow-sm">
              A
            </div>
            <h1 className="login-title font-display text-5xl font-semibold leading-none tracking-tight">
              <SplitText text="Aris Proxy" />
            </h1>
            <p className="login-at login-at-3 mt-4 max-w-sm text-sm leading-6 text-sidebar-foreground/70">
              {t("auth.login_subtitle")}
            </p>
          </div>
          <p className="login-at login-at-5 text-xs text-sidebar-foreground/50">
            {t("login.secure_access")}
          </p>
        </div>

        <div className="flex flex-col justify-center p-8 md:p-10">
          <div className="mb-8 md:hidden">
            <h1 className="login-title font-display text-5xl font-semibold tracking-tight text-foreground">
              <SplitText text="Aris Proxy" />
            </h1>
            <p className="login-at login-at-3 mt-2 text-sm text-muted-foreground">
              {t("auth.login_subtitle")}
            </p>
          </div>
          <div className="login-at login-at-2 hidden md:block">
            <h2 className="font-display text-4xl font-semibold tracking-tight text-foreground">
              {t("login.welcome_back")}
            </h2>
            <p className="mt-2 text-sm text-muted-foreground">{t("login.sign_in")}</p>
          </div>

          {error && (
            <div className="animate-fade mt-6 rounded-xl border border-destructive/20 bg-destructive/10 px-4 py-3 text-sm text-destructive">
              {error}
            </div>
          )}

          <div className="login-actions mt-8 flex flex-col gap-3">
            <Button
              size="lg"
              className="relative"
              data-pending={pending === "github" ? "" : undefined}
              aria-busy={pending === "github"}
              onClick={() => startOAuth("github")}
            >
              {t("auth.login_github")}
            </Button>
            <Button
              size="lg"
              variant="outline"
              className="relative"
              data-magnetic=""
              data-pending={pending === "google" ? "" : undefined}
              aria-busy={pending === "google"}
              onClick={() => startOAuth("google")}
            >
              {t("auth.login_google")}
            </Button>
            {demoAvailable && (
              <>
                <div className="my-1 flex items-center gap-3 text-xs text-muted-foreground">
                  <span className="h-px flex-1 bg-border" aria-hidden />
                  <span>{t("login.demo_or")}</span>
                  <span className="h-px flex-1 bg-border" aria-hidden />
                </div>
                <Button size="lg" variant="secondary" data-magnetic="" onClick={demoLogin}>
                  {t("login.demo_continue")}
                </Button>
              </>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
