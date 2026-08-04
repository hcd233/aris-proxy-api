"use client";

import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";

interface TableSkeletonProps {
  /** 骨架行数 */
  rows?: number;
  /** 行高 class（默认 h-12） */
  rowClassName?: string;
}

/**
 * 列表页统一的 loading 骨架：rows 行全宽骨架条。
 */
export function TableSkeleton({ rows = 3, rowClassName }: TableSkeletonProps) {
  return (
    <div className="space-y-3">
      {Array.from({ length: rows }).map((_, i) => (
        <Skeleton key={i} className={cn("h-12 w-full", rowClassName)} />
      ))}
    </div>
  );
}
