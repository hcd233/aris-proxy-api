import {
  Activity,
  Ban,
  Bot,
  Database,
  Footprints,
  Key,
  Layers,
  LayoutDashboard,
  MessageSquare,
  Play,
  ScrollText,
  Settings2,
  Share2,
  Timer,
  User,
  Users,
  type LucideIcon,
} from "lucide-react";
import { withBasePath } from "./routes";
import type { DemoModule } from "@/lib/types";

/** 仪表盘首页的 Next 路由路径（不含 basePath），用于 <Link> 与导航配置。 */
export const DASHBOARD_HOME_PATH = "/dashboard/";

/** 仪表盘首页的完整 URL（含 basePath），用于 window.location 整页跳转。 */
export const DASHBOARD_HOME_URL = withBasePath(DASHBOARD_HOME_PATH);

function withTrailingSlash(path: string): string {
  return path.endsWith("/") ? path : `${path}/`;
}

/** 导航项高亮判定：按路径段前缀匹配，兼容 usePathname 带或不带尾斜杠。 */
export function isNavItemActive(pathname: string, href: string): boolean {
  return withTrailingSlash(pathname).startsWith(withTrailingSlash(href));
}

// ─── 导航配置：侧栏与 ⌘K 命令面板共用同一份 ──────────────────────────────────

export interface NavItem {
  labelKey: string;
  href: string;
  icon: LucideIcon;
  adminOnly?: boolean;
  /** demo 用户可配置开放的模块 key；无 key 的导航项对 demo 隐藏 */
  demoModule?: DemoModule;
}

export interface NavGroup {
  key: string;
  labelKey: string;
  items: NavItem[];
  /** 置顶分组：条目直接铺在导航顶层，没有可折叠的组头（避免「只有一项」的分组还要多点一次） */
  pinned?: boolean;
}

export const NAV_GROUPS: NavGroup[] = [
  {
    key: "overview",
    labelKey: "nav.group.overview",
    pinned: true,
    items: [
      {
        labelKey: "nav.dashboard",
        href: DASHBOARD_HOME_PATH,
        icon: LayoutDashboard,
        demoModule: "dashboard",
      },
    ],
  },
  {
    key: "data",
    labelKey: "nav.group.data",
    items: [
      { labelKey: "nav.sessions", href: "/sessions/", icon: MessageSquare, demoModule: "sessions" },
      { labelKey: "nav.shares", href: "/shares/", icon: Share2 },
      { labelKey: "nav.audit", href: "/audit/model/", icon: ScrollText, demoModule: "audit" },
      { labelKey: "nav.dataset", href: "/dataset/", icon: Database },
    ],
  },
  {
    key: "gateway",
    labelKey: "nav.group.gateway",
    items: [
      { labelKey: "nav.api_keys", href: "/apikeys/", icon: Key },
      { labelKey: "nav.upstream", href: "/upstream/", icon: Layers, demoModule: "upstream" },
      {
        labelKey: "nav.trigger",
        href: "/trigger/",
        icon: Ban,
        adminOnly: true,
        demoModule: "trigger",
      },
      { labelKey: "nav.harness", href: "/harness/", icon: Bot, adminOnly: true },
      { labelKey: "nav.playground", href: "/playground/", icon: Play },
    ],
  },
  {
    key: "ops",
    labelKey: "nav.group.ops",
    items: [
      { labelKey: "nav.cron", href: "/cron/", icon: Timer, adminOnly: true, demoModule: "cron" },
      {
        labelKey: "nav.cron_audit",
        href: "/audit/cron/",
        icon: ScrollText,
        adminOnly: true,
        demoModule: "cron_audit",
      },
      {
        labelKey: "nav.demo_access_audit",
        href: "/audit/demo/",
        icon: Footprints,
        adminOnly: true,
      },
      {
        labelKey: "nav.monitor",
        href: "/monitor/",
        icon: Activity,
        adminOnly: true,
        demoModule: "monitor",
      },
    ],
  },
  {
    key: "system",
    labelKey: "nav.group.system",
    items: [
      { labelKey: "nav.users", href: "/users/", icon: Users, adminOnly: true },
      { labelKey: "nav.demo", href: "/demo/", icon: Settings2, adminOnly: true },
      { labelKey: "nav.profile", href: "/profile/", icon: User },
    ],
  },
];

/** 当前页面所在的可折叠分组 key；置顶分组没有组头，不参与展开逻辑。 */
export function findActiveGroupKey(pathname: string): string | undefined {
  return NAV_GROUPS.find(
    (group) => !group.pinned && group.items.some((item) => isNavItemActive(pathname, item.href)),
  )?.key;
}

export interface NavAccess {
  isAdmin: boolean;
  isDemo: boolean;
  isModuleOpen: (module: DemoModule) => boolean;
}

export interface ResolvedNavItem extends NavItem {
  /** demo 用户未开放的模块：展示但置灰，不可进入 */
  locked: boolean;
}

export interface ResolvedNavGroup extends Omit<NavGroup, "items"> {
  items: ResolvedNavItem[];
}

/**
 * 按当前身份裁剪导航：
 * - demo：全部显示，未开放的模块置灰锁定（没有 demoModule 的页面一律锁定）
 * - 其余：adminOnly 页面仅管理员可见
 * 过滤后没有条目的分组整组移除。
 */
export function resolveNavGroups(groups: NavGroup[], access: NavAccess): ResolvedNavGroup[] {
  return groups
    .map((group) => ({
      ...group,
      items: group.items
        .filter((item) => access.isDemo || !item.adminOnly || access.isAdmin)
        .map((item) => ({
          ...item,
          locked:
            access.isDemo &&
            !(item.demoModule !== undefined && access.isModuleOpen(item.demoModule)),
        })),
    }))
    .filter((group) => group.items.length > 0);
}

/** 命令面板匹配：空查询匹配全部；否则任一检索文本（标题 / 路径 / 分组名）包含查询即命中，不区分大小写。 */
export function matchesNavQuery(query: string, haystacks: string[]): boolean {
  const q = query.trim().toLowerCase();
  if (!q) return true;
  return haystacks.some((h) => h.toLowerCase().includes(q));
}
