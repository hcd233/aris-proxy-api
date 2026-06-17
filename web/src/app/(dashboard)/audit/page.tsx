"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";

export default function AuditRedirectPage() {
  const router = useRouter();
  useEffect(() => {
    router.replace("/web/model-call-audit/");
  }, [router]);
  return null;
}
