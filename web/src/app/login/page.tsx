"use client";

import { useCallback, useEffect, useState } from "react";
import { AlertCircle, ShieldCheck } from "lucide-react";
import { useAuth } from "@/lib/auth-context";
import { DASHBOARD_HOME_URL } from "@/lib/nav";
import { useT } from "@/lib/i18n";
import { api } from "@/lib/api-client";
import { parseOAuthCallbackParams, resolveOAuthPlatform } from "@/lib/oauth";
import type { OAuth2Provider } from "@/lib/types";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { PreferenceControls } from "@/components/preference-controls";
import { ProviderIcon } from "@/components/provider-icon";
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
        window.location.href = DASHBOARD_HOME_URL;
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
      window.location.href = DASHBOARD_HOME_URL;
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
        window.location.href = DASHBOARD_HOME_URL;
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
      <div className="page-surface flex min-h-dvh items-center justify-center bg-background px-6">
        <div className="animate-scale-in flex flex-col items-center gap-7 text-center">
          <span className="flex size-12 items-center justify-center rounded-[0.7rem] bg-primary font-display text-2xl font-semibold text-primary-foreground">
            A
          </span>
          <div>
            <p className="font-display text-3xl font-semibold tracking-tight text-foreground">
              Aris Proxy
            </p>
            <p className="mt-2 text-sm text-muted-foreground">{t("common.loading")}</p>
          </div>
          <span className="h-0.5 w-28 overflow-hidden rounded-full bg-border">
            <span className="block h-full w-1/2 animate-shimmer bg-foreground/50" />
          </span>
        </div>
      </div>
    );
  }

  return (
    <div className="login-stage page-surface relative grid min-h-dvh grid-cols-1 bg-background lg:grid-cols-[1fr_minmax(420px,37%)]">
      <script dangerouslySetInnerHTML={{ __html: LOGIN_SEEN_SCRIPT }} />

      {/* 页头控件：登录页自带主题/语言切换，不再挂左下角悬浮球 */}
      <PreferenceControls className="absolute right-5 top-5 z-20 sm:right-8 sm:top-6" />

      {/* 左：产品世界（纸 / 虚空） */}
      <div className="relative flex flex-col px-7 pb-2 pt-5 sm:px-12 lg:px-[clamp(3rem,6vw,5.5rem)] lg:py-10">
        <header className="flex items-center gap-3">
          <span className="login-mark flex size-9 items-center justify-center rounded-[0.6rem] bg-primary font-display text-base font-semibold text-primary-foreground">
            A
          </span>
          <span className="font-display text-[1.05rem] font-semibold tracking-tight text-foreground">
            {t("auth.login_title")}
          </span>
        </header>

        <div className="mt-10 lg:my-auto lg:py-10">
          <h1 className="login-title font-display text-[clamp(3.5rem,9.2vw,8.75rem)] font-semibold leading-[0.84] tracking-[-0.035em] text-foreground moonshot:[text-shadow:0_0_70px_rgb(255_255_255/0.14)]">
            <span className="block">
              <SplitText text="Aris" />
            </span>
            <span className="block">
              <SplitText text="Proxy" />
            </span>
          </h1>
          <p className="login-at login-at-3 mt-6 max-w-[24rem] text-base leading-[1.8] text-muted-foreground lg:mt-8 lg:text-lg">
            {t("auth.login_subtitle")}
          </p>
        </div>
      </div>

      {/* 右：门页 + 表单 */}
      <div className="relative flex items-center justify-center rounded-t-[1.75rem] border-t border-border bg-card px-6 pb-12 pt-10 sm:px-10 lg:rounded-l-[2rem] lg:rounded-tr-none lg:border-l lg:border-t-0 lg:px-10 lg:py-16 lg:shadow-[-28px_0_60px_-42px_color-mix(in_oklab,var(--foreground)_50%,transparent)] xl:px-14 moonshot:lg:border-white/[0.1] moonshot:lg:shadow-[inset_1px_0_0_rgb(255_255_255/0.08)]">
        {/* 门槛销钉 */}
        <span
          aria-hidden="true"
          className="absolute left-0 top-1/2 hidden -translate-x-1/2 -translate-y-1/2 lg:block"
        >
          <span className="block size-[7px] rounded-full bg-primary ring-[5px] ring-background" />
        </span>

        <div className="w-full max-w-[360px]">
          <div className="login-at login-at-2">
            <h2 className="font-display text-4xl font-semibold leading-[1.1] tracking-[-0.02em] text-foreground">
              {t("login.welcome_back")}
            </h2>
            <p className="mt-3 text-base text-muted-foreground">{t("login.sign_in")}</p>
          </div>

          {error && (
            <Alert variant="destructive" className="animate-fade mt-6">
              <AlertCircle />
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          )}

          <div className="login-actions mt-9 flex flex-col gap-2.5">
            <Button
              size="lg"
              className="relative h-12 w-full justify-center gap-2.5 rounded-xl bg-foreground text-background hover:bg-foreground/90"
              data-pending={pending === "github" ? "" : undefined}
              aria-busy={pending === "github"}
              onClick={() => startOAuth("github")}
            >
              <ProviderIcon protocol="github" size={18} className="size-[18px]" />
              {t("auth.login_github")}
            </Button>
            <Button
              size="lg"
              variant="secondary"
              className="relative h-12 w-full justify-center gap-2.5 rounded-xl"
              data-pending={pending === "google" ? "" : undefined}
              aria-busy={pending === "google"}
              onClick={() => startOAuth("google")}
            >
              <ProviderIcon protocol="google" size={18} className="size-[18px]" />
              {t("auth.login_google")}
            </Button>
            {demoAvailable && (
              <>
                <div aria-hidden="true" className="my-5 flex items-center gap-4">
                  <span className="h-px flex-1 bg-border" />
                  <span className="text-xs text-muted-foreground">{t("login.demo_or")}</span>
                  <span className="h-px flex-1 bg-border" />
                </div>
                <Button
                  variant="ghost"
                  className="h-11 w-full justify-center text-muted-foreground hover:text-foreground"
                  onClick={demoLogin}
                >
                  {t("login.demo_continue")}
                </Button>
              </>
            )}
          </div>

          <p className="login-at login-at-5 mt-10 flex items-center justify-center gap-2 text-xs text-muted-foreground">
            <ShieldCheck className="size-3.5" aria-hidden="true" />
            {t("login.secure_access")}
          </p>
        </div>
      </div>
    </div>
  );
}
