"use client";

import { Fragment, Suspense, useCallback, useEffect, useState, type ReactNode } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useAuth } from "@/lib/auth-context";
import { useT } from "@/lib/i18n";
import type { DemoModule } from "@/lib/types";
import { PermissionGuard } from "@/components/permission-guard";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import {
  TooltipProvider,
  TooltipRoot,
  TooltipTrigger,
  TooltipContent,
} from "@/components/ui/tooltip";
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetTrigger } from "@/components/ui/sheet";
import { LanguageSwitcher } from "@/components/language-switcher";
import { ThemeSwitcher } from "@/components/theme/theme-switcher";
import { LocaleFade } from "@/components/locale-fade";
import { SessionHistorySidebar } from "@/components/session-detail/session-history-sidebar";
import {
  LayoutDashboard,
  MessageSquare,
  Key,
  Server,
  Cpu,
  User,
  LogOut,
  Menu,
  Share2,
  ScrollText,
  Ban,
  Timer,
  Activity,
  Database,
  Radar,
  Users,
  Lock,
  Settings2,
  ChevronDown,
} from "lucide-react";

interface NavItem {
  labelKey: string;
  href: string;
  icon: ReactNode;
  adminOnly?: boolean;
  /** demo 用户可配置开放的模块 key；无 key 的导航项对 demo 隐藏 */
  demoModule?: DemoModule;
}

interface NavGroup {
  key: string;
  labelKey: string;
  items: NavItem[];
}

const NAV_GROUPS: NavGroup[] = [
  {
    key: "overview",
    labelKey: "nav.group.overview",
    items: [
      {
        labelKey: "nav.dashboard",
        href: "/",
        icon: <LayoutDashboard className="size-4" />,
        demoModule: "dashboard",
      },
    ],
  },
  {
    key: "data",
    labelKey: "nav.group.data",
    items: [
      {
        labelKey: "nav.sessions",
        href: "/sessions/",
        icon: <MessageSquare className="size-4" />,
        demoModule: "sessions",
      },
      { labelKey: "nav.shares", href: "/shares/", icon: <Share2 className="size-4" /> },
      {
        labelKey: "nav.audit",
        href: "/audit/model/",
        icon: <ScrollText className="size-4" />,
        demoModule: "audit",
      },
      { labelKey: "nav.dataset", href: "/dataset/", icon: <Database className="size-4" /> },
      { labelKey: "nav.trace", href: "/trace/", icon: <Radar className="size-4" /> },
    ],
  },
  {
    key: "gateway",
    labelKey: "nav.group.gateway",
    items: [
      { labelKey: "nav.api_keys", href: "/apikeys/", icon: <Key className="size-4" /> },
      {
        labelKey: "nav.endpoints",
        href: "/endpoints/",
        icon: <Server className="size-4" />,
        adminOnly: true,
        demoModule: "endpoints",
      },
      {
        labelKey: "nav.models",
        href: "/models/",
        icon: <Cpu className="size-4" />,
        adminOnly: true,
        demoModule: "models",
      },
      {
        labelKey: "nav.trigger",
        href: "/trigger/",
        icon: <Ban className="size-4" />,
        adminOnly: true,
        demoModule: "trigger",
      },
    ],
  },
  {
    key: "ops",
    labelKey: "nav.group.ops",
    items: [
      {
        labelKey: "nav.cron",
        href: "/cron/",
        icon: <Timer className="size-4" />,
        adminOnly: true,
        demoModule: "cron",
      },
      {
        labelKey: "nav.cron_audit",
        href: "/audit/cron/",
        icon: <ScrollText className="size-4" />,
        adminOnly: true,
        demoModule: "cron_audit",
      },
      {
        labelKey: "nav.monitor",
        href: "/monitor/",
        icon: <Activity className="size-4" />,
        adminOnly: true,
        demoModule: "monitor",
      },
    ],
  },
  {
    key: "system",
    labelKey: "nav.group.system",
    items: [
      { labelKey: "nav.users", href: "/users/", icon: <Users className="size-4" />, adminOnly: true },
      {
        labelKey: "nav.demo",
        href: "/demo/",
        icon: <Settings2 className="size-4" />,
        adminOnly: true,
      },
      { labelKey: "nav.profile", href: "/profile/", icon: <User className="size-4" /> },
    ],
  },
];

function isItemActive(pathname: string, href: string): boolean {
  return href === "/" ? pathname === "/" : pathname.startsWith(href);
}

function findActiveGroupKey(pathname: string): string | undefined {
  return NAV_GROUPS.find((group) => group.items.some((item) => isItemActive(pathname, item.href)))
    ?.key;
}

const OPEN_GROUPS_STORAGE_KEY = "sidebar-nav-open-groups";

function SidebarNav({
  groups,
  openGroups,
  onToggleGroup,
  onNavigate,
  collapsed = false,
}: {
  groups: NavGroup[];
  openGroups: ReadonlySet<string>;
  onToggleGroup: (key: string) => void;
  onNavigate?: () => void;
  collapsed?: boolean;
}) {
  const pathname = usePathname();
  const { isAdmin, isDemo, isModuleOpen } = useAuth();
  const t = useT();

  const visibleGroups = groups
    .map((group) => ({
      ...group,
      items: group.items.filter((item) => {
        // demo 用户：全部显示，未开放的模块置灰锁定；其余按 admin 权限过滤
        if (isDemo()) return true;
        return !item.adminOnly || isAdmin();
      }),
    }))
    .filter((group) => group.items.length > 0);

  return (
    <TooltipProvider>
      <nav className="flex flex-col px-2">
        {visibleGroups.map((group, groupIndex) => {
          // 窄栏模式无组头，子项平铺不折叠
          const groupOpen = collapsed || openGroups.has(group.key);
          return (
            <div key={group.key} className={groupIndex > 0 ? "mt-3" : ""}>
              {collapsed ? (
                <Separator className="mx-1 mb-1 bg-sidebar-border/60" />
              ) : (
                <button
                  type="button"
                  onClick={() => onToggleGroup(group.key)}
                  aria-expanded={openGroups.has(group.key)}
                  className="flex w-full items-center gap-2 rounded-md px-3 pb-1 pt-1.5 text-[11px] font-medium uppercase tracking-wider text-sidebar-foreground/45 transition-colors hover:text-sidebar-foreground/80"
                >
                  <span className="min-w-0 flex-1 truncate text-left">{t(group.labelKey)}</span>
                  <ChevronDown
                    className={`size-3.5 shrink-0 text-sidebar-foreground/40 transition-transform duration-200 ${
                      openGroups.has(group.key) ? "" : "-rotate-90"
                    }`}
                  />
                </button>
              )}
              {groupOpen && (
                <div className="flex flex-col gap-0.5">
                  {group.items.map((item) => {
                    const isActive = isItemActive(pathname, item.href);
                    const label = t(item.labelKey);
                    const demoLocked =
                      isDemo() && !(item.demoModule !== undefined && isModuleOpen(item.demoModule));

                    const link = demoLocked ? (
                      <span
                        aria-disabled="true"
                        className={`flex cursor-not-allowed select-none items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium text-sidebar-foreground/45 ${
                          collapsed ? "justify-center" : ""
                        }`}
                      >
                        <span className="opacity-60">{item.icon}</span>
                        {!collapsed && (
                          <>
                            <span className="min-w-0 flex-1 truncate">{label}</span>
                            <Lock className="size-3.5 shrink-0 opacity-60" />
                          </>
                        )}
                      </span>
                    ) : (
                      <Link
                        href={item.href}
                        onClick={onNavigate}
                        className={`flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-all duration-150 ${
                          isActive
                            ? "bg-sidebar-accent text-sidebar-accent-foreground shadow-2xs"
                            : "text-sidebar-foreground hover:bg-sidebar-accent/60 hover:text-sidebar-accent-foreground"
                        } ${collapsed ? "justify-center" : ""}`}
                        aria-label={collapsed ? label : undefined}
                      >
                        <span className={isActive ? "text-sidebar-primary" : ""}>{item.icon}</span>
                        {!collapsed && <span>{label}</span>}
                      </Link>
                    );

                    return collapsed || demoLocked ? (
                      <TooltipRoot key={item.href}>
                        <TooltipTrigger render={link} />
                        <TooltipContent side="right">
                          {demoLocked ? `${label} · ${t("nav.demo_locked")}` : label}
                        </TooltipContent>
                      </TooltipRoot>
                    ) : (
                      <Fragment key={item.href}>{link}</Fragment>
                    );
                  })}
                </div>
              )}
            </div>
          );
        })}
      </nav>
    </TooltipProvider>
  );
}

function UserBar({ collapsed = false }: { collapsed?: boolean }) {
  const { user, logout } = useAuth();
  const t = useT();

  if (!user) return null;

  const initials = (user.name ?? user.email ?? "U")
    .split(" ")
    .map((n) => n[0])
    .join("")
    .toUpperCase()
    .slice(0, 2);

  return (
    <div className="flex flex-col gap-2 rounded-xl border border-sidebar-border/60 bg-sidebar-accent/50 p-2 text-sidebar-foreground transition-all duration-150">
      <div className={`flex items-center gap-3 ${collapsed ? "justify-center" : ""}`}>
        <Avatar size="sm">
          {user.avatar && <AvatarImage src={user.avatar} alt={user.name ?? ""} />}
          <AvatarFallback className="bg-sidebar-primary/20 text-sidebar-primary text-xs font-medium">
            {initials}
          </AvatarFallback>
        </Avatar>
        {!collapsed && (
          <div className="hidden min-w-0 flex-1 md:block">
            <p className="truncate text-sm font-medium leading-none">
              {user.name ?? user.email ?? t("layout.user")}
            </p>
            <div className="mt-1 flex items-center gap-1.5">
              <Badge variant="secondary" className="px-1.5 py-0 text-[10px] font-medium">
                {user.permission}
              </Badge>
            </div>
          </div>
        )}
      </div>
      {!collapsed && (
        <div className="flex items-center justify-between">
          <LanguageSwitcher />
          <div className="flex items-center gap-1">
            <ThemeSwitcher variant="inline" />
            <TooltipProvider>
              <TooltipRoot>
                <TooltipTrigger
                  render={
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      onClick={logout}
                      aria-label={t("nav.logout")}
                      className="text-sidebar-foreground/60 hover:text-sidebar-foreground hover:bg-sidebar-accent"
                    >
                      <LogOut className="size-4" />
                    </Button>
                  }
                />
                <TooltipContent side="top">{t("nav.logout")}</TooltipContent>
              </TooltipRoot>
            </TooltipProvider>
          </div>
        </div>
      )}
    </div>
  );
}

export default function DashboardLayout({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [collapsed, setCollapsed] = useState(false);

  const isSessionDetail = pathname.startsWith("/sessions/detail");

  const closeMobileSidebar = useCallback(() => setSidebarOpen(false), []);

  const t = useT();

  // 初始只展开当前页面所在组；用户手动切换后由 localStorage 接管
  const [openGroups, setOpenGroups] = useState<ReadonlySet<string>>(() => {
    const active = findActiveGroupKey(pathname);
    return active ? new Set([active]) : new Set();
  });

  // Persist collapsed state; auto-expand on session detail so history is visible.
  /* eslint-disable react-hooks/set-state-in-effect -- Reading localStorage requires setting state in effect on mount */
  useEffect(() => {
    const saved = localStorage.getItem("sidebar-collapsed");
    if (saved !== null) setCollapsed(saved === "true");

    const savedGroups = localStorage.getItem(OPEN_GROUPS_STORAGE_KEY);
    if (savedGroups) {
      try {
        const keys = JSON.parse(savedGroups);
        if (Array.isArray(keys)) setOpenGroups(new Set(keys.filter((k) => typeof k === "string")));
      } catch {
        // 存储损坏时保持默认（仅当前组展开）
      }
    }
  }, []);

  useEffect(() => {
    if (isSessionDetail) {
      setCollapsed(false);
    }
  }, [isSessionDetail]);

  // 导航到收起组内的页面时自动展开该组
  useEffect(() => {
    const active = findActiveGroupKey(pathname);
    if (active) {
      setOpenGroups((prev) => (prev.has(active) ? prev : new Set(prev).add(active)));
    }
  }, [pathname]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const toggleCollapsed = useCallback(() => {
    setCollapsed((prev) => {
      const next = !prev;
      localStorage.setItem("sidebar-collapsed", String(next));
      return next;
    });
  }, []);

  const toggleGroup = useCallback((key: string) => {
    setOpenGroups((prev) => {
      const next = new Set(prev);
      if (next.has(key)) {
        next.delete(key);
      } else {
        next.add(key);
      }
      localStorage.setItem(OPEN_GROUPS_STORAGE_KEY, JSON.stringify([...next]));
      return next;
    });
  }, []);

  return (
    <PermissionGuard allowDemo>
      <div className="page-surface flex h-screen overflow-hidden bg-background text-foreground">
        {/* Desktop sidebar */}
        <aside
          className={`hidden md:flex flex-col border-r border-sidebar-border bg-sidebar text-sidebar-foreground transition-[width] duration-200 ${
            collapsed ? "w-16" : "w-64"
          }`}
        >
          <div className="flex h-14 items-center justify-between border-b border-sidebar-border/50 px-3">
            {!collapsed && (
              <span className="flex items-center gap-2.5">
                <span className="flex size-7 items-center justify-center rounded-lg bg-sidebar-primary font-display text-sm font-semibold text-sidebar-primary-foreground shadow-2xs">
                  A
                </span>
                <span className="font-display text-lg font-semibold tracking-tight text-sidebar-foreground">
                  {t("layout.aris_proxy")}
                </span>
              </span>
            )}
            <Button
              variant="ghost"
              size="icon-sm"
              onClick={toggleCollapsed}
              disabled={isSessionDetail}
              className={
                collapsed
                  ? "mx-auto text-sidebar-foreground/50 hover:text-sidebar-foreground hover:bg-sidebar-accent"
                  : "text-sidebar-foreground/50 hover:text-sidebar-foreground hover:bg-sidebar-accent"
              }
            >
              <Menu className="size-4" />
            </Button>
          </div>
          <div className="flex-1 overflow-y-auto py-3">
            <SidebarNav
              groups={NAV_GROUPS}
              openGroups={openGroups}
              onToggleGroup={toggleGroup}
              collapsed={collapsed}
            />
            {!collapsed && isSessionDetail && (
              <Suspense fallback={null}>
                <SessionHistorySidebar />
              </Suspense>
            )}
          </div>
          <Separator className="bg-sidebar-border/50" />
          <div className="p-2">
            <UserBar collapsed={collapsed} />
          </div>
        </aside>

        {/* Mobile sidebar via Sheet */}
        <Sheet open={sidebarOpen} onOpenChange={setSidebarOpen}>
          {/* Main content */}
          <div className="flex flex-1 flex-col overflow-hidden">
            {/* Mobile top bar */}
            <header className="flex h-14 items-center gap-3 border-b border-border/70 bg-background/80 px-4 backdrop-blur-md md:hidden">
              <SheetTrigger
                render={
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    className="text-foreground/60 hover:text-foreground hover:bg-secondary"
                  />
                }
              >
                <Menu className="size-5" />
              </SheetTrigger>
              <span className="flex items-center gap-2.5">
                <span className="flex size-6 items-center justify-center rounded-md bg-primary font-display text-xs font-semibold text-primary-foreground shadow-2xs">
                  A
                </span>
                <span className="font-display text-lg font-semibold tracking-tight">
                  {t("layout.aris_proxy")}
                </span>
              </span>
            </header>

            <main className="flex-1 overflow-y-auto p-4 md:p-8 lg:p-10">
              <LocaleFade>
                <div className="mx-auto max-w-6xl">{children}</div>
              </LocaleFade>
            </main>
          </div>

          <SheetContent
            side="left"
            className="w-72 border-sidebar-border bg-sidebar p-0 text-sidebar-foreground"
          >
            <SheetHeader className="border-b border-sidebar-border/50 px-4 py-3">
              <SheetTitle className="flex items-center gap-2.5 font-display text-xl font-semibold tracking-tight">
                <span className="flex size-7 items-center justify-center rounded-lg bg-sidebar-primary font-display text-sm font-semibold text-sidebar-primary-foreground shadow-2xs">
                  A
                </span>
                {t("layout.aris_proxy")}
              </SheetTitle>
            </SheetHeader>
            <div className="flex-1 overflow-y-auto py-3">
              <SidebarNav
                groups={NAV_GROUPS}
                openGroups={openGroups}
                onToggleGroup={toggleGroup}
                onNavigate={closeMobileSidebar}
              />
            </div>
            <Separator className="bg-sidebar-border/50" />
            <div className="p-2">
              <UserBar />
            </div>
          </SheetContent>
        </Sheet>
      </div>
    </PermissionGuard>
  );
}
