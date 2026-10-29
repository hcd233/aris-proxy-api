"use client";

import { Suspense, useCallback, useEffect, useRef, useState, type ReactNode } from "react";
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
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarInset,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarProvider,
  SidebarSeparator,
  useSidebar,
} from "@/components/ui/sidebar";
import {
  TooltipProvider,
  TooltipRoot,
  TooltipTrigger,
  TooltipContent,
} from "@/components/ui/tooltip";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";
import { LanguageSwitcher } from "@/components/language-switcher";
import { ThemeSwitcher } from "@/components/theme/theme-switcher";
import { LocaleFade } from "@/components/locale-fade";
import { NavIndicator } from "@/components/motion/nav-indicator";
import { SessionHistorySidebar } from "@/components/session-detail/session-history-sidebar";
import { useIsCompact, useIsMobile } from "@/hooks/use-mobile";
import {
  LayoutDashboard,
  MessageSquare,
  Key,
  User,
  LogOut,
  Menu,
  Share2,
  ScrollText,
  Play,
  Ban,
  Timer,
  Activity,
  Database,
  Users,
  Lock,
  Settings2,
  ChevronDown,
  Bot,
  Footprints,
  Layers,
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
    ],
  },
  {
    key: "gateway",
    labelKey: "nav.group.gateway",
    items: [
      { labelKey: "nav.api_keys", href: "/apikeys/", icon: <Key className="size-4" /> },
      {
        labelKey: "nav.upstream",
        href: "/upstream/",
        icon: <Layers className="size-4" />,
        demoModule: "upstream",
      },
      {
        labelKey: "nav.trigger",
        href: "/trigger/",
        icon: <Ban className="size-4" />,
        adminOnly: true,
        demoModule: "trigger",
      },
      {
        labelKey: "nav.harness",
        href: "/harness/",
        icon: <Bot className="size-4" />,
        adminOnly: true,
      },
      {
        labelKey: "nav.playground",
        href: "/playground/",
        icon: <Play className="size-4" />,
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
        labelKey: "nav.demo_access_audit",
        href: "/audit/demo/",
        icon: <Footprints className="size-4" />,
        adminOnly: true,
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
      {
        labelKey: "nav.users",
        href: "/users/",
        icon: <Users className="size-4" />,
        adminOnly: true,
      },
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
}: {
  groups: NavGroup[];
  openGroups: ReadonlySet<string>;
  onToggleGroup: (key: string) => void;
}) {
  const pathname = usePathname();
  const { isAdmin, isDemo, isModuleOpen } = useAuth();
  const { state, isMobile, setOpenMobile, openMobile } = useSidebar();
  const collapsed = state === "collapsed";
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

  // Anything that can move the active item re-targets the indicator.
  const layoutKey = [
    pathname,
    collapsed ? "c" : "e",
    visibleGroups
      .map((g) => `${g.key}:${g.items.length}:${collapsed || openGroups.has(g.key) ? 1 : 0}`)
      .join(","),
  ].join("|");

  // 视口较矮时侧栏会溢出：导航到新页面后把激活项滚进视口，
  // 否则当前页可能落在用户卡片下方，wayfinding 完全失效。
  // 只在 pathname / 移动端抽屉打开时滚动；同页展开/折叠分组不应把用户拽回激活项。
  const navRef = useRef<HTMLElement>(null);
  const lastScrolledKey = useRef<string | null>(null);
  const scrollKey = isMobile && openMobile ? `${pathname}|mobile-open` : pathname;
  useEffect(() => {
    if (lastScrolledKey.current === scrollKey) return;
    const active = navRef.current?.querySelector<HTMLElement>("[data-nav-item][data-active]");
    if (!active) return;
    active.scrollIntoView({ block: "nearest" });
    lastScrolledKey.current = scrollKey;
  }, [layoutKey, scrollKey]);

  return (
    <TooltipProvider>
      <nav ref={navRef} className="relative flex flex-col px-2">
        <NavIndicator layoutKey={layoutKey} />
        {visibleGroups.map((group, groupIndex) => {
          // 窄栏模式无组头，子项平铺不折叠
          const groupOpen = collapsed || openGroups.has(group.key);
          return (
            <Collapsible
              key={group.key}
              open={groupOpen}
              onOpenChange={() => onToggleGroup(group.key)}
              className={groupIndex > 0 ? "mt-3" : ""}
            >
              {collapsed ? (
                <Separator className="mx-1 mb-1 bg-sidebar-border/60" />
              ) : (
                <SidebarGroupLabel
                  render={<CollapsibleTrigger />}
                  className="w-full cursor-pointer gap-2 hover:text-sidebar-foreground"
                >
                  <TooltipRoot>
                    <TooltipTrigger
                      render={
                        <span className="min-w-0 flex-1 truncate text-left">
                          {t(group.labelKey)}
                        </span>
                      }
                    />
                    <TooltipContent side="right">{t(group.labelKey)}</TooltipContent>
                  </TooltipRoot>
                  <ChevronDown className="-rotate-90 in-data-panel-open:rotate-0" />
                </SidebarGroupLabel>
              )}
              {/* data-instant：NavIndicator 在 layoutKey 变化时一次性测量激活项位置，
                  高度过渡会让测量落在动画起点，所以侧栏分组不做展开动画 */}
              <CollapsibleContent data-instant="">
                <SidebarMenu className="nav-items flex flex-col gap-0.5">
                  {group.items.map((item) => {
                    const isActive = isItemActive(pathname, item.href);
                    const label = t(item.labelKey);
                    const demoLocked =
                      isDemo() && !(item.demoModule !== undefined && isModuleOpen(item.demoModule));

                    return (
                      <SidebarMenuItem key={item.href}>
                        <SidebarMenuButton
                          variant="nav"
                          isActive={isActive}
                          data-nav-item=""
                          render={
                            demoLocked ? (
                              <span aria-disabled="true" />
                            ) : (
                              <Link
                                href={item.href}
                                onClick={() => {
                                  if (isMobile) setOpenMobile(false);
                                }}
                              />
                            )
                          }
                          tooltip={
                            demoLocked
                              ? { children: `${label} · ${t("nav.demo_locked")}`, hidden: false }
                              : label
                          }
                          className="h-auto gap-3 rounded-lg px-3 py-2 font-medium"
                        >
                          <span className={isActive ? "text-sidebar-primary" : ""}>
                            {item.icon}
                          </span>
                          {/* 静态分析假阳性豁免：truncate 的 tooltip 由 SidebarMenuButton 的
                              tooltip prop 内部提供（collapsed/locked 时弹出），变量引用不构成
                              ESLint parent 链，规则无法静态判定。 */}
                          {/* eslint-disable-next-line truncate-requires-tooltip/truncate-requires-tooltip */}
                          <span className="min-w-0 truncate">
                            {label}
                            {demoLocked && <Lock className="ml-1 inline size-3.5 opacity-60" />}
                          </span>
                        </SidebarMenuButton>
                      </SidebarMenuItem>
                    );
                  })}
                </SidebarMenu>
              </CollapsibleContent>
            </Collapsible>
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
            <TooltipRoot>
              <TooltipTrigger
                render={
                  <p className="truncate text-sm font-medium leading-none">
                    {user.name ?? user.email ?? t("layout.user")}
                  </p>
                }
              />
              <TooltipContent side="right" className="max-w-xs break-all">
                {user.name ?? user.email ?? t("layout.user")}
              </TooltipContent>
            </TooltipRoot>
            <div className="mt-1 flex items-center gap-1.5">
              <Badge variant="secondary" className="px-1.5 py-0 text-[10px] font-medium">
                {t(`permission.${user.permission}`)}
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

/** 移动端顶栏（SidebarTrigger 需要 SidebarProvider context，故独立组件）。 */
function MobileTopBar() {
  const { toggleSidebar } = useSidebar();
  const t = useT();

  return (
    <header className="flex h-[calc(3.5rem+env(safe-area-inset-top))] items-center gap-3 border-b border-border/70 bg-background/80 px-4 pt-[env(safe-area-inset-top)] backdrop-blur-md md:hidden">
      <Button
        variant="ghost"
        size="icon-sm"
        onClick={toggleSidebar}
        aria-label={t("layout.sidebar_toggle")}
        className="text-foreground/60 hover:text-foreground hover:bg-secondary"
      >
        <Menu className="size-5" />
      </Button>
      <span className="flex items-center gap-2.5">
        <span className="flex size-6 items-center justify-center rounded-md bg-primary font-display text-xs font-semibold text-primary-foreground shadow-2xs">
          A
        </span>
        <span className="font-display text-lg font-semibold tracking-tight">
          {t("layout.aris_proxy")}
        </span>
      </span>
    </header>
  );
}

export default function DashboardLayout({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  const [collapsed, setCollapsed] = useState(false);
  const isMobile = useIsMobile();
  const isCompact = useIsCompact();

  const isSessionDetail = pathname.startsWith("/sessions/detail");
  // 会话详情需要历史侧栏常驻：只在桌面（≥1024px）锁定展开；平板历史改走页内抽屉
  const lockExpanded = isSessionDetail && !isCompact;

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
    // 平板首次进入默认收成图标栏，给内容让出宽度；用户手动展开后照常记忆
    else setCollapsed(window.matchMedia("(min-width: 768px) and (max-width: 1023px)").matches);

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
    // 直接读 matchMedia：hydration 首帧 useIsCompact 还是服务端快照 false，
    // 用 lockExpanded 判断会在平板上误把侧栏展开
    if (isSessionDetail && !window.matchMedia("(max-width: 1023px)").matches) {
      setCollapsed(false);
    }
  }, [isSessionDetail, isCompact]);

  // 导航到收起组内的页面时自动展开该组
  useEffect(() => {
    const active = findActiveGroupKey(pathname);
    if (active) {
      setOpenGroups((prev) => (prev.has(active) ? prev : new Set(prev).add(active)));
    }
  }, [pathname]);
  /* eslint-enable react-hooks/set-state-in-effect */

  // SidebarProvider 受控：桌面折叠态走 localStorage；session-detail 需要历史侧栏常驻，锁定展开。
  const handleOpenChange = useCallback(
    (open: boolean) => {
      if (lockExpanded) return;
      setCollapsed(!open);
      localStorage.setItem("sidebar-collapsed", String(!open));
    },
    [lockExpanded],
  );

  const toggleCollapsed = useCallback(
    () => handleOpenChange(collapsed),
    [handleOpenChange, collapsed],
  );

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
      <SidebarProvider
        open={!collapsed}
        onOpenChange={handleOpenChange}
        className="page-surface h-screen overflow-hidden bg-background text-foreground"
      >
        {/* 桌面固定侧栏；移动端自动降级为 Sheet 抽屉（宽度 w-72） */}
        <Sidebar collapsible="icon" className="border-sidebar-border">
          <SidebarHeader className="flex h-14 flex-row items-center justify-between border-b border-sidebar-border/50 px-3">
            {(!collapsed || isMobile) && (
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
              disabled={lockExpanded}
              aria-label={t("layout.sidebar_toggle")}
              className={
                collapsed
                  ? "mx-auto text-sidebar-foreground/50 hover:text-sidebar-foreground hover:bg-sidebar-accent"
                  : "text-sidebar-foreground/50 hover:text-sidebar-foreground hover:bg-sidebar-accent"
              }
            >
              <Menu className="size-4" />
            </Button>
          </SidebarHeader>
          <SidebarContent className="overflow-y-auto! py-3">
            <SidebarNav groups={NAV_GROUPS} openGroups={openGroups} onToggleGroup={toggleGroup} />
            {!collapsed && lockExpanded && (
              <Suspense fallback={null}>
                <SessionHistorySidebar />
              </Suspense>
            )}
          </SidebarContent>
          <SidebarSeparator className="mx-0 bg-sidebar-border/50" />
          <SidebarFooter>
            <UserBar collapsed={collapsed} />
          </SidebarFooter>
        </Sidebar>

        {/* Main content */}
        <SidebarInset className="overflow-hidden">
          <MobileTopBar />
          <div className="flex-1 overflow-y-auto px-5 pt-4 pb-[calc(env(safe-area-inset-bottom)+1rem)] md:p-8 lg:p-10">
            <LocaleFade>
              {/* data-page-wide：信息密度高的页面（dashboard）放宽到 1600px，其余页面保持 6xl 阅读宽度 */}
              <div className="mx-auto max-w-6xl has-[[data-page-wide]]:max-w-[1600px]">
                {children}
              </div>
            </LocaleFade>
          </div>
        </SidebarInset>
      </SidebarProvider>
    </PermissionGuard>
  );
}
