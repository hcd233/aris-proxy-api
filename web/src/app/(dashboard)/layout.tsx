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
}: {
  items: NavItem[];
  onNavigate?: () => void;
}) {
  const pathname = usePathname();
  const { isAdmin } = useAuth();

  const visibleItems = items.filter(
    (item) => !item.adminOnly || isAdmin()
  );

  return (
    <nav className="flex flex-col gap-1 px-2">
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
            className={`flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors ${
              isActive
                ? "bg-primary text-primary-foreground"
                : "text-muted-foreground hover:bg-accent hover:text-accent-foreground"
            }`}
          >
            {item.icon}
            {item.label}
          </Link>
        );
      })}
    </nav>
  );
}

function UserBar() {
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
    <div className="flex items-center gap-3">
      <Avatar size="sm">
        {user.avatar && <AvatarImage src={user.avatar} alt={user.name ?? ""} />}
        <AvatarFallback>{initials}</AvatarFallback>
      </Avatar>
      <div className="hidden min-w-0 flex-1 md:block">
        <p className="truncate text-sm font-medium leading-none">
          {user.name ?? user.email ?? "User"}
        </p>
        <div className="mt-1 flex items-center gap-1.5">
          <Badge variant="secondary" className="text-[10px] px-1.5 py-0">
            {user.permission}
          </Badge>
        </div>
      </div>
      <Button variant="ghost" size="icon-sm" onClick={logout} title="Logout">
        <LogOut className="size-4" />
      </Button>
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
      <div className="flex h-screen overflow-hidden bg-background">
        {/* Desktop sidebar */}
        <aside
          className={`hidden md:flex flex-col border-r border-border bg-card transition-[width] duration-200 ${
            collapsed ? "w-16" : "w-56"
          }`}
        >
          <div className="flex h-14 items-center justify-between border-b px-3">
            {!collapsed && (
              <span className="text-base font-semibold tracking-tight">
                Aris Proxy
              </span>
            )}
            <Button
              variant="ghost"
              size="icon-sm"
              onClick={toggleCollapsed}
              className={collapsed ? "mx-auto" : ""}
            >
              <Menu className="size-4" />
            </Button>
          </div>
          <div className="flex-1 overflow-y-auto py-3">
            <SidebarNav
              items={collapsed ? navItems.map((n) => ({ ...n, label: "" })) : navItems}
            />
          </div>
          <Separator />
          <div className="p-3">
            <UserBar />
          </div>
        </aside>

        {/* Mobile sidebar via Sheet */}
        <Sheet open={sidebarOpen} onOpenChange={setSidebarOpen}>
          <SheetContent side="left" className="w-64 p-0">
            <SheetHeader className="border-b px-4 py-3">
              <SheetTitle>Aris Proxy</SheetTitle>
            </SheetHeader>
            <div className="flex-1 overflow-y-auto py-3">
              <SidebarNav items={navItems} onNavigate={closeMobileSidebar} />
            </div>
            <Separator />
            <div className="p-3">
              <UserBar />
            </div>
          </SheetContent>
        </Sheet>

        {/* Main content */}
        <div className="flex flex-1 flex-col overflow-hidden">
          {/* Mobile top bar */}
          <header className="flex h-14 items-center gap-3 border-b px-4 md:hidden">
            <SheetTrigger
              render={
                <Button
                  variant="ghost"
                  size="icon-sm"
                  onClick={() => setSidebarOpen(true)}
                />
              }
            >
              <Menu className="size-5" />
            </SheetTrigger>
            <span className="text-base font-semibold">Aris Proxy</span>
          </header>

          <main className="flex-1 overflow-y-auto p-4 md:p-6 lg:p-8">
            {children}
          </main>
        </div>
      </div>
    </PermissionGuard>
  );
}