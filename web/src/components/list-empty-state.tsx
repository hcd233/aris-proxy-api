"use client";

import type { ReactNode } from "react";
import type { LucideIcon } from "lucide-react";
import {
  Empty,
  EmptyContent,
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
  /** 下一步动作（可选）：空态不该是死胡同——真没数据时给创建 / 前往入口，被筛掉时给「清除筛选」 */
  action?: ReactNode;
}

/**
 * 列表页统一的空态：基于 shadcn Empty 组合（图标 + 标题 + 可选描述 + 可选下一步动作）。
 */
export function ListEmptyState({ icon: Icon, message, hint, action }: ListEmptyStateProps) {
  return (
    <Empty>
      <EmptyHeader>
        <EmptyMedia variant="icon">
          <Icon />
        </EmptyMedia>
        <EmptyTitle>{message}</EmptyTitle>
        {hint && <EmptyDescription>{hint}</EmptyDescription>}
      </EmptyHeader>
      {action && <EmptyContent>{action}</EmptyContent>}
    </Empty>
  );
}
