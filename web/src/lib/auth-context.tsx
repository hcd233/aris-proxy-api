"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { api } from "./api-client";
import { normalizeDemoModules } from "./demo-modules";
import type { DemoModule, DetailedUser, OAuth2Provider } from "./types";

interface AuthContextValue {
  user: DetailedUser | null;
  accessToken: string | null;
  isLoading: boolean;
  login: (provider: OAuth2Provider) => void;
  handleCallback: (accessToken: string, refreshToken: string) => Promise<void>;
  logout: () => void;
  isAdmin: () => boolean;
  isUser: () => boolean;
  isDemo: () => boolean;
  /** demo 用户可访问的模块（非 demo 用户为 null） */
  demoModules: DemoModule[] | null;
  isModuleOpen: (module: DemoModule) => boolean;
}

const AuthContext = createContext<AuthContextValue | null>(null);

function readStorage(key: string): string | null {
  if (typeof window === "undefined") return null;
  return localStorage.getItem(key);
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<DetailedUser | null>(null);
  const [accessToken, setAccessToken] = useState<string | null>(() => readStorage("access_token"));
  const [isLoading, setIsLoading] = useState(true);
  const [demoModules, setDemoModules] = useState<DemoModule[] | null>(null);
  const initRan = useRef(false);

  const fetchUser = useCallback(async () => {
    try {
      const rsp = await api.getCurrentUser();
      if (rsp.error) {
        throw new Error(rsp.error.message);
      }
      setUser(rsp.user ?? null);
      // demo 用户加载模块配置（导航/守卫渲染用）；失败按空模块处理（fail-closed）
      if (rsp.user?.permission === "demo") {
        try {
          const cfgRsp = await api.getDemoConfig();
          setDemoModules(normalizeDemoModules(cfgRsp.config?.modules));
        } catch {
          setDemoModules([]);
        }
      } else {
        setDemoModules(null);
      }
    } catch {
      setUser(null);
      setDemoModules(null);
    }
  }, []);

  /* eslint-disable react-hooks/set-state-in-effect -- Auth initialization requires setting state from localStorage on mount */
  useEffect(() => {
    if (initRan.current) return;
    initRan.current = true;

    const storedAccess = readStorage("access_token");
    const storedRefresh = readStorage("refresh_token");

    if (storedAccess) {
      fetchUser().finally(() => setIsLoading(false));
    } else if (storedRefresh) {
      api
        .refreshToken({ refreshToken: storedRefresh })
        .then((rsp) => {
          if (rsp.error) {
            throw new Error(rsp.error.message);
          }
          if (rsp.accessToken) {
            localStorage.setItem("access_token", rsp.accessToken);
            setAccessToken(rsp.accessToken);
          }
          if (rsp.refreshToken) {
            localStorage.setItem("refresh_token", rsp.refreshToken);
          }
          return fetchUser();
        })
        .catch(() => {
          localStorage.removeItem("access_token");
          localStorage.removeItem("refresh_token");
          setAccessToken(null);
          setUser(null);
        })
        .finally(() => setIsLoading(false));
    } else {
      setIsLoading(false);
    }
  }, [fetchUser]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const login = useCallback(async (provider: OAuth2Provider) => {
    try {
      const rsp = await api.oauth2Login(provider);
      if (rsp.error) {
        console.error("[Auth] Login failed", rsp.error);
        return;
      }
      if (rsp.redirectURL) {
        window.location.href = rsp.redirectURL;
      }
    } catch (err) {
      console.error("[Auth] Login error", err);
    }
  }, []);

  const handleCallback = useCallback(
    async (newAccessToken: string, newRefreshToken: string) => {
      localStorage.setItem("access_token", newAccessToken);
      localStorage.setItem("refresh_token", newRefreshToken);
      setAccessToken(newAccessToken);
      await fetchUser();
    },
    [fetchUser],
  );

  const logout = useCallback(() => {
    localStorage.removeItem("access_token");
    localStorage.removeItem("refresh_token");
    setAccessToken(null);
    setUser(null);
    setDemoModules(null);
  }, []);

  const isAdmin = useCallback(() => user?.permission === "admin", [user]);
  const isUser = useCallback(
    () => user?.permission === "user" || user?.permission === "admin",
    [user],
  );
  const isDemo = useCallback(() => user?.permission === "demo", [user]);
  const isModuleOpen = useCallback(
    (module: DemoModule) => demoModules?.includes(module) ?? false,
    [demoModules],
  );

  return (
    <AuthContext.Provider
      value={{
        user,
        accessToken,
        isLoading,
        login,
        handleCallback,
        logout,
        isAdmin,
        isUser,
        isDemo,
        demoModules,
        isModuleOpen,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return ctx;
}

export { AuthContext };
