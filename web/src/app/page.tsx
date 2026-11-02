"use client";

import Link from "next/link";
import { DASHBOARD_HOME_PATH } from "@/lib/nav";

export default function PortalPage() {
  return (
    <main className="page-surface flex min-h-dvh items-center justify-center bg-background">
      <Link href={DASHBOARD_HOME_PATH} className="font-display text-2xl text-foreground">
        Aris Proxy
      </Link>
    </main>
  );
}
