"use client";

import type { ReactNode } from "react";
import type { LucideIcon } from "lucide-react";
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from "@/components/ui/empty";

interface ListEmptyStateProps {
  icon: LucideIcon;
  message: ReactNode;
  /** 次要提示行（可选） */
  hint?: ReactNode;
}

/**
 * 列表页统一的空态：基于 shadcn Empty 组合（图标 + 标题 + 可选描述）。
 */
export function ListEmptyState({ icon: Icon, message, hint }: ListEmptyStateProps) {
  return (
    <Empty>
      <EmptyHeader>
        <EmptyMedia variant="icon">
          <Icon />
        </EmptyMedia>
        <EmptyTitle>{message}</EmptyTitle>
        {hint && <EmptyDescription>{hint}</EmptyDescription>}
      </EmptyHeader>
    </Empty>
  );
}
