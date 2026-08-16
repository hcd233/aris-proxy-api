"use client";

import { Suspense } from "react";
import { useSearchParams } from "next/navigation";
import TraceDetailClient from "@/components/trace-detail/trace-detail-client";
import { PermissionGuard } from "@/components/permission-guard";

function TraceDetailContent() {
  const searchParams = useSearchParams();
  const traceId = Number(searchParams.get("id"));
  return <TraceDetailClient traceId={traceId} />;
}

export default function TraceDetailPage() {
  return (
    <PermissionGuard>
      <Suspense fallback={null}>
        <TraceDetailContent />
      </Suspense>
    </PermissionGuard>
  );
}
