"use client";

import type { ReactNode } from "react";

interface ListEmptyStateProps {
  /** 空态图标（自带 className，如 "mb-3 size-10 text-muted-foreground/40"） */
  icon: ReactNode;
  message: ReactNode;
  /** 次要提示行（可选） */
  hint?: ReactNode;
}

/**
 * 列表页统一的空态：居中图标 + 提示文案（可附次要提示行）。
 */
export function ListEmptyState({ icon, message, hint }: ListEmptyStateProps) {
  return (
    <div className="flex flex-col items-center justify-center py-12 text-center">
      {icon}
      <p className="text-sm text-muted-foreground">{message}</p>
      {hint && <p className="mt-1 text-xs text-muted-foreground/70">{hint}</p>}
    </div>
  );
}
