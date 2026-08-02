"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";

export default function AuditRedirectPage() {
  const router = useRouter();
  useEffect(() => {
    // next/navigation 自动加 basePath(/web) 前缀，不要手动拼
    router.replace("/audit/model/");
  }, [router]);
  return null;
}
