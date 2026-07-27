"use client";

import { Suspense } from "react";
import { useSearchParams } from "next/navigation";
import TraceDetailClient from "@/components/trace-detail/trace-detail-client";

function TraceDetailContent() {
  const searchParams = useSearchParams();
  const traceId = Number(searchParams.get("id"));
  return <TraceDetailClient traceId={traceId} />;
}

export default function TraceDetailPage() {
  return (
    <Suspense fallback={null}>
      <TraceDetailContent />
    </Suspense>
  );
}
