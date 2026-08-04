"use client";

import type { ReactNode } from "react";

interface PageHeaderProps {
  title: ReactNode;
  description?: ReactNode;
  /** 右侧操作区（按钮/弹窗触发器），可选 */
  actions?: ReactNode;
}

/**
 * Dashboard 页面的统一标题区：左侧 title + description，右侧可选操作区。
 * 所有列表页共用同一套布局（flex-col → md:flex-row），保证跨页面视觉一致。
 */
export function PageHeader({ title, description, actions }: PageHeaderProps) {
  return (
    <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
      <div>
        <h1 className="font-display text-2xl font-semibold tracking-tight text-foreground md:text-3xl">
          {title}
        </h1>
        {description && (
          <p className="mt-1.5 text-sm text-muted-foreground">{description}</p>
        )}
      </div>
      {actions && <div className="shrink-0">{actions}</div>}
    </div>
  );
}
