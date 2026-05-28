"use client";

import { useCallback, useEffect, useState, type ReactNode } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useAuth } from "@/lib/auth-context";
import { PermissionGuard } from "@/components/permission-guard";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet";
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
} from "lucide-react";

interface NavItem {
  label: string;
  href: string;
  icon: ReactNode;
  adminOnly?: boolean;
}

const navItems: NavItem[] = [
  {
    label: "Dashboard",
    href: "/",
    icon: <LayoutDashboard className="size-4" />,
  },
  {
    label: "Sessions",
    href: "/sessions/",
    icon: <MessageSquare className="size-4" />,
  },
  {
    label: "Shares",
    href: "/shares/",
    icon: <Share2 className="size-4" />,
  },
  {
    label: "API Keys",
    href: "/apikeys/",
    icon: <Key className="size-4" />,
  },
  {
    label: "Endpoints",
    href: "/endpoints/",
    icon: <Server className="size-4" />,
    adminOnly: true,
  },
  {
    label: "Models",
    href: "/models/",
    icon: <Cpu className="size-4" />,
    adminOnly: true,
  },
  {
    label: "Profile",
    href: "/profile/",
    icon: <User className="size-4" />,
  },
];

function SidebarNav({
  items,
  onNavigate,
  collapsed = false,
}: {
  items: NavItem[];
  onNavigate?: () => void;
  collapsed?: boolean;
}) {
  const pathname = usePathname();
  const { isAdmin } = useAuth();

  const visibleItems = items.filter(
    (item) => !item.adminOnly || isAdmin()
  );

  return (
    <nav className="flex flex-col gap-0.5 px-2">
      {visibleItems.map((item) => {
        const isActive =
          item.href === "/"
            ? pathname === "/"
            : pathname.startsWith(item.href);

        return (
          <Link
            key={item.href}
            href={item.href}
            onClick={onNavigate}
            className={`flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium transition-all duration-150 ${
              isActive
                ? "bg-sidebar-primary text-sidebar-primary-foreground"
                : "text-sidebar-foreground/70 hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
            } ${collapsed ? "justify-center" : ""}`}
            title={collapsed ? item.label : undefined}
          >
            <span className={isActive ? "text-white" : ""}>{item.icon}</span>
            {!collapsed && <span>{item.label}</span>}
          </Link>
        );
      })}
    </nav>
  );
}

function UserBar({ collapsed = false }: { collapsed?: boolean }) {
  const { user, logout } = useAuth();

  if (!user) return null;

  const initials =
    (user.name ?? user.email ?? "U")
      .split(" ")
      .map((n) => n[0])
      .join("")
      .toUpperCase()
      .slice(0, 2);

  return (
    <div className={`flex items-center gap-3 rounded-xl border border-sidebar-border/60 bg-sidebar-accent/50 p-2 text-sidebar-foreground transition-all duration-150 ${collapsed ? "justify-center" : ""}`}>
      <Avatar size="sm">
        {user.avatar && <AvatarImage src={user.avatar} alt={user.name ?? ""} />}
        <AvatarFallback className="bg-sidebar-primary/20 text-sidebar-primary text-xs font-medium">
          {initials}
        </AvatarFallback>
      </Avatar>
      {!collapsed && (
        <>
          <div className="hidden min-w-0 flex-1 md:block">
            <p className="truncate text-sm font-medium leading-none">
              {user.name ?? user.email ?? "User"}
            </p>
            <div className="mt-1 flex items-center gap-1.5">
              <Badge variant="secondary" className="px-1.5 py-0 text-[10px] font-medium">
                {user.permission}
              </Badge>
            </div>
          </div>
          <Button variant="ghost" size="icon-sm" onClick={logout} title="Logout" className="text-sidebar-foreground/60 hover:text-sidebar-foreground hover:bg-sidebar-accent">
            <LogOut className="size-4" />
          </Button>
        </>
      )}
    </div>
  );
}

export default function DashboardLayout({
  children,
}: {
  children: ReactNode;
}) {
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [collapsed, setCollapsed] = useState(false);

  const closeMobileSidebar = useCallback(() => setSidebarOpen(false), []);

  // Persist collapsed state
  /* eslint-disable react-hooks/set-state-in-effect -- Reading localStorage requires setting state in effect on mount */
  useEffect(() => {
    const saved = localStorage.getItem("sidebar-collapsed");
    if (saved !== null) setCollapsed(saved === "true");
  }, []);
  /* eslint-enable react-hooks/set-state-in-effect */

  const toggleCollapsed = useCallback(() => {
    setCollapsed((prev) => {
      const next = !prev;
      localStorage.setItem("sidebar-collapsed", String(next));
      return next;
    });
  }, []);

  return (
    <PermissionGuard>
      <div className="flex h-screen overflow-hidden bg-background text-foreground">
        {/* Desktop sidebar */}
        <aside
          className={`hidden md:flex flex-col border-r border-sidebar-border bg-sidebar text-sidebar-foreground transition-[width] duration-200 ${
            collapsed ? "w-16" : "w-64"
          }`}
        >
          <div className="flex h-14 items-center justify-between border-b border-sidebar-border/50 px-3">
            {!collapsed && (
              <span className="font-display text-lg font-semibold tracking-tight text-sidebar-foreground">
                Aris Proxy
              </span>
            )}
            <Button
              variant="ghost"
              size="icon-sm"
              onClick={toggleCollapsed}
              className={collapsed ? "mx-auto text-sidebar-foreground/50 hover:text-sidebar-foreground hover:bg-sidebar-accent" : "text-sidebar-foreground/50 hover:text-sidebar-foreground hover:bg-sidebar-accent"}
            >
              <Menu className="size-4" />
            </Button>
          </div>
          <div className="flex-1 overflow-y-auto py-3">
            <SidebarNav items={navItems} collapsed={collapsed} />
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
            <header className="flex h-14 items-center gap-3 border-b border-border bg-card/60 px-4 backdrop-blur-sm md:hidden">
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
              <span className="font-display text-xl font-semibold tracking-tight">Aris Proxy</span>
            </header>

            <main className="flex-1 overflow-y-auto p-4 md:p-8 lg:p-10">
              <div className="mx-auto max-w-6xl">{children}</div>
            </main>
          </div>

          <SheetContent side="left" className="w-72 border-sidebar-border bg-sidebar p-0 text-sidebar-foreground">
            <SheetHeader className="border-b border-sidebar-border/50 px-4 py-3">
              <SheetTitle className="font-display text-xl font-semibold tracking-tight">Aris Proxy</SheetTitle>
            </SheetHeader>
            <div className="flex-1 overflow-y-auto py-3">
              <SidebarNav items={navItems} onNavigate={closeMobileSidebar} />
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