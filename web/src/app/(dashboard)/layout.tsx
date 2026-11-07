"use client";

import { useCallback, useEffect, useRef, useState, type ReactNode } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useAuth } from "@/lib/auth-context";
import { useT } from "@/lib/i18n";
import {
  NAV_GROUPS,
  findActiveGroupKey,
  isNavItemActive,
  resolveNavGroups,
  type NavGroup,
  type ResolvedNavGroup,
} from "@/lib/nav";
import { PermissionGuard } from "@/components/permission-guard";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
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
  useSidebar,
} from "@/components/ui/sidebar";
import {
  TooltipProvider,
  TooltipRoot,
  TooltipTrigger,
  TooltipContent,
} from "@/components/ui/tooltip";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";
import { PreferenceControls } from "@/components/preference-controls";
import { NavIndicator } from "@/components/motion/nav-indicator";
import { useIsMobile } from "@/hooks/use-mobile";
import { NavCommandPalette, NavSearchButton } from "@/components/nav-command-palette";
import {
  ChevronDown,
  Lock,
  LogOut,
  Menu,
  PanelLeftClose,
  PanelLeftOpen,
  Search,
} from "lucide-react";
import { TruncatedText } from "@/components/truncated-text";
import { cn } from "@/lib/utils";

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
  // 移动端抽屉永远是展开形态，不继承桌面端的折叠态
  const collapsed = state === "collapsed" && !isMobile;
  const t = useT();

  // demo 用户：全部显示，未开放的模块置灰锁定；其余按 admin 权限过滤（规则见 resolveNavGroups）
  const visibleGroups = resolveNavGroups(groups, {
    isAdmin: isAdmin(),
    isDemo: isDemo(),
    isModuleOpen,
  });

  // Anything that can move the active item re-targets the indicator.
  const layoutKey = [
    pathname,
    collapsed ? "c" : "e",
    visibleGroups
      .map(
        (g) =>
          `${g.key}:${g.items.length}:${g.pinned || collapsed || openGroups.has(g.key) ? 1 : 0}`,
      )
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

  const renderItem = (item: ResolvedNavGroup["items"][number]) => {
    const isActive = isNavItemActive(pathname, item.href);
    const label = t(item.labelKey);
    const demoLocked = item.locked;
    const Icon = item.icon;

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
            demoLocked ? { children: `${label} · ${t("nav.demo_locked")}`, hidden: false } : label
          }
          className="h-8 gap-2.5 rounded-md px-2.5 text-sidebar-foreground/80 hover:text-sidebar-foreground data-active:font-medium data-active:text-sidebar-foreground"
        >
          <Icon
            className={cn(
              "transition-colors",
              isActive
                ? "text-sidebar-primary"
                : "text-sidebar-foreground/60 group-hover/menu-button:text-sidebar-foreground",
            )}
          />
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
  };

  return (
    <TooltipProvider>
      <nav ref={navRef} className="relative flex flex-col px-2">
        <NavIndicator layoutKey={layoutKey} />
        {visibleGroups.map((group, groupIndex) => {
          // 置顶分组（仪表盘）：没有组头可折叠，条目直接铺在顶层，一次点击到达
          if (group.pinned) {
            return (
              <SidebarMenu
                key={group.key}
                className={cn("nav-items flex flex-col gap-px", groupIndex > 0 && "mt-4")}
              >
                {group.items.map(renderItem)}
              </SidebarMenu>
            );
          }
          // 窄栏模式无组头，子项平铺不折叠
          const groupOpen = collapsed || openGroups.has(group.key);
          return (
            <Collapsible
              key={group.key}
              open={groupOpen}
              onOpenChange={() => onToggleGroup(group.key)}
              className={cn(groupIndex > 0 && (collapsed ? "mt-2" : "mt-4"))}
            >
              {collapsed ? (
                <Separator className="mx-2 mb-2 bg-sidebar-border/70" />
              ) : (
                // 组头做成小号大写的 eyebrow，与条目拉开层级；展开时箭头只在悬停/聚焦时出现，
                // 收起的组常显右箭头，提示「这里还有内容」
                <SidebarGroupLabel
                  render={<CollapsibleTrigger />}
                  className="group/label mb-0.5 h-7 w-full cursor-pointer gap-2 px-2.5 text-[11px] font-medium tracking-[0.08em] text-sidebar-foreground/70 uppercase hover:text-sidebar-foreground [&>svg]:size-3.5"
                >
                  <TruncatedText className="min-w-0 flex-1 text-left" side="right">
                    {t(group.labelKey)}
                  </TruncatedText>
                  <ChevronDown className="-rotate-90 transition-opacity in-data-panel-open:rotate-0 in-data-panel-open:opacity-0 in-data-panel-open:group-hover/label:opacity-100 in-data-panel-open:group-focus-visible/label:opacity-100" />
                </SidebarGroupLabel>
              )}
              {/* data-instant：NavIndicator 在 layoutKey 变化时一次性测量激活项位置，
                  高度过渡会让测量落在动画起点，所以侧栏分组不做展开动画 */}
              <CollapsibleContent data-instant="">
                <SidebarMenu className="nav-items flex flex-col gap-px">
                  {group.items.map(renderItem)}
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

  const name = user.name ?? user.email ?? t("layout.user");
  const role = t(`permission.${user.permission}`);
  const avatar = (
    <Avatar size="sm" className="shrink-0">
      {user.avatar && <AvatarImage src={user.avatar} alt={user.name ?? ""} />}
      <AvatarFallback className="bg-sidebar-primary/15 text-xs font-medium text-sidebar-foreground">
        {initials}
      </AvatarFallback>
    </Avatar>
  );

  // 窄栏：偏好控件竖排 + 头像（悬停看身份），不再塞进卡片里被挤压
  if (collapsed) {
    return (
      <div className="flex flex-col items-center gap-1.5">
        <PreferenceControls tone="sidebar" orientation="vertical" />
        <TooltipProvider>
          <TooltipRoot>
            <TooltipTrigger render={<span className="mt-1 flex rounded-full" />}>
              {avatar}
            </TooltipTrigger>
            <TooltipContent side="right">
              {name} · {role}
            </TooltipContent>
          </TooltipRoot>
        </TooltipProvider>
      </div>
    );
  }

  // 宽栏：偏好控件一行，身份一行（登出跟着账号走）；去掉卡片外壳和角色徽章，省出一行高度
  return (
    <div className="flex flex-col gap-1.5 text-sidebar-foreground">
      <PreferenceControls tone="sidebar" className="px-1" />
      <div className="flex items-center gap-2.5 rounded-lg px-1 py-1">
        {avatar}
        <div className="min-w-0 flex-1">
          <TruncatedText render={<p />} className="text-sm leading-tight font-medium" side="right">
            {name}
          </TruncatedText>
          <p className="mt-0.5 text-xs leading-tight text-sidebar-foreground/70">{role}</p>
        </div>
        <TooltipProvider>
          <TooltipRoot>
            <TooltipTrigger
              render={
                <Button
                  variant="ghost"
                  size="icon-sm"
                  onClick={logout}
                  aria-label={t("nav.logout")}
                  className="shrink-0 text-sidebar-foreground/60 hover:bg-sidebar-accent hover:text-sidebar-foreground"
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
  );
}

/** 移动端顶栏（SidebarTrigger 需要 SidebarProvider context，故独立组件）。 */
function MobileTopBar({ onSearch }: { onSearch: () => void }) {
  const { toggleSidebar } = useSidebar();
  const t = useT();

  return (
    <header className="flex h-14 items-center gap-3 border-b border-border/70 bg-background/80 px-4 backdrop-blur-md md:hidden">
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
          {t("layout.aris_mass")}
        </span>
      </span>
      <Button
        variant="ghost"
        size="icon-sm"
        onClick={onSearch}
        aria-label={t("nav.search.trigger")}
        className="ml-auto text-foreground/60 hover:bg-secondary hover:text-foreground"
      >
        <Search className="size-5" />
      </Button>
    </header>
  );
}

export default function DashboardLayout({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  const [collapsed, setCollapsed] = useState(false);
  const isMobile = useIsMobile();

  const t = useT();

  // ⌘K / Ctrl+K 页面跳转面板：侧栏入口、移动端顶栏入口与快捷键共用同一个开关
  const [paletteOpen, setPaletteOpen] = useState(false);
  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key.toLowerCase() !== "k" || event.shiftKey || event.altKey) return;
      if (!(event.metaKey || event.ctrlKey)) return;
      event.preventDefault();
      setPaletteOpen((open) => !open);
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, []);

  // 初始只展开当前页面所在组；用户手动切换后由 localStorage 接管
  const [openGroups, setOpenGroups] = useState<ReadonlySet<string>>(() => {
    const active = findActiveGroupKey(pathname);
    return active ? new Set([active]) : new Set();
  });

  // Persist collapsed state.
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

  // 导航到收起组内的页面时自动展开该组
  useEffect(() => {
    const active = findActiveGroupKey(pathname);
    if (active) {
      setOpenGroups((prev) => (prev.has(active) ? prev : new Set(prev).add(active)));
    }
  }, [pathname]);
  /* eslint-enable react-hooks/set-state-in-effect */

  // SidebarProvider 受控：桌面折叠态走 localStorage。
  const handleOpenChange = useCallback((open: boolean) => {
    setCollapsed(!open);
    localStorage.setItem("sidebar-collapsed", String(!open));
  }, []);

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
        className="page-surface h-dvh overflow-hidden bg-background text-foreground"
      >
        {/* 桌面固定侧栏；移动端自动降级为 Sheet 抽屉（宽度 w-72） */}
        <Sidebar collapsible="icon" className="border-sidebar-border">
          <SidebarHeader className="flex h-14 flex-row items-center justify-between border-b border-sidebar-border/60 px-3">
            {(!collapsed || isMobile) && (
              <span className="flex min-w-0 items-center gap-2.5">
                <span className="flex size-7 shrink-0 items-center justify-center rounded-lg bg-sidebar-primary font-display text-sm font-semibold text-sidebar-primary-foreground shadow-2xs">
                  A
                </span>
                <span className="font-display text-lg font-semibold tracking-tight text-sidebar-foreground">
                  {t("layout.aris_mass")}
                </span>
              </span>
            )}
            {/* 移动端抽屉点遮罩即可关闭，折叠开关只对桌面有意义 */}
            {!isMobile && (
              <TooltipProvider>
                <TooltipRoot>
                  <TooltipTrigger
                    render={
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        onClick={toggleCollapsed}
                        aria-label={t("layout.sidebar_toggle")}
                        className={cn(
                          "text-sidebar-foreground/60 hover:bg-sidebar-accent hover:text-sidebar-foreground",
                          collapsed && "mx-auto",
                        )}
                      >
                        {collapsed ? (
                          <PanelLeftOpen className="size-4" />
                        ) : (
                          <PanelLeftClose className="size-4" />
                        )}
                      </Button>
                    }
                  />
                  <TooltipContent side={collapsed ? "right" : "bottom"}>
                    {t("layout.sidebar_toggle")}
                  </TooltipContent>
                </TooltipRoot>
              </TooltipProvider>
            )}
          </SidebarHeader>
          <SidebarContent className="overflow-y-auto! py-3">
            <NavSearchButton
              collapsed={collapsed && !isMobile}
              onClick={() => setPaletteOpen(true)}
            />
            <SidebarNav groups={NAV_GROUPS} openGroups={openGroups} onToggleGroup={toggleGroup} />
          </SidebarContent>
          <SidebarFooter className="border-t border-sidebar-border/60 p-2">
            <UserBar collapsed={collapsed && !isMobile} />
          </SidebarFooter>
        </Sidebar>

        {/* Main content */}
        <SidebarInset className="overflow-hidden">
          <MobileTopBar onSearch={() => setPaletteOpen(true)} />
          {/* 预留滚动条槽：页面随翻译变长/变短时滚动条出现与消失不改变主体宽度 */}
          <div className="flex-1 overflow-y-auto [scrollbar-gutter:stable] p-4 md:p-8 lg:p-10">
            {/* 所有控制台页面共用同一主体宽度，不要在页面里自行加宽或收窄 */}
            <div className="mx-auto max-w-[1600px]">{children}</div>
          </div>
        </SidebarInset>
        <NavCommandPalette open={paletteOpen} onOpenChange={setPaletteOpen} />
      </SidebarProvider>
    </PermissionGuard>
  );
}
