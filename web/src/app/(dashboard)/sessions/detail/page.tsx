"use client";

import { Suspense } from "react";
import { useSearchParams } from "next/navigation";
import SessionDetailClient from "@/components/session-detail/session-detail-client";

function SessionDetailContent() {
  const searchParams = useSearchParams();
  const sessionId = Number(searchParams.get("id"));
  return <SessionDetailClient sessionId={sessionId} />;
}

export default function SessionDetailPage() {
  return (
    <Suspense fallback={null}>
      <SessionDetailContent />
    </Suspense>
  );
}
