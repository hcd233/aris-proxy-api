"use client";

import type { ReactNode } from "react";
import {
  Drawer,
  DrawerContent,
  DrawerFooter,
  DrawerHeader,
  DrawerTitle,
} from "@/components/ui/drawer";

export interface RowDetailField {
  label: ReactNode;
  value: ReactNode;
}

interface RowDetailDrawerProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: ReactNode;
  fields: RowDetailField[];
  /** 行内操作（启用开关、删除、复制等）在手机上移到这里 */
  actions?: ReactNode;
}

/**
 * 表格行详情：窄屏下表格只留关键列，点行从底部弹出，列出该行全部字段（不截断）。
 * 配合 lib/row-detail.ts 的 rowDetailProps 与 TableHead/TableCell 的 priority 使用。
 */
export function RowDetailDrawer({
  open,
  onOpenChange,
  title,
  fields,
  actions,
}: RowDetailDrawerProps) {
  return (
    <Drawer open={open} onOpenChange={onOpenChange} showSwipeHandle>
      <DrawerContent>
        <DrawerHeader className="text-left!">
          <DrawerTitle className="break-all">{title}</DrawerTitle>
        </DrawerHeader>
        <dl className="grid min-h-0 grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-3 overflow-y-auto px-4 py-4 text-sm">
          {fields.map((f, i) => (
            <div key={i} className="contents">
              <dt className="text-muted-foreground">{f.label}</dt>
              <dd className="min-w-0 break-all whitespace-normal">{f.value}</dd>
            </div>
          ))}
        </dl>
        {actions && (
          <DrawerFooter className="flex-row flex-wrap pb-[calc(env(safe-area-inset-bottom)+1rem)]">
            {actions}
          </DrawerFooter>
        )}
        {!actions && <div className="pb-[env(safe-area-inset-bottom)]" />}
      </DrawerContent>
    </Drawer>
  );
}
