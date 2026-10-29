import type { HTMLAttributes, KeyboardEvent, MouseEvent } from "react";

/**
 * 表格列分级与行详情的纯逻辑（spec 2026-10-29-mobile-tablet-adapt §5.1）。
 * 组件见 components/ui/table.tsx（priority）与 components/row-detail-drawer.tsx。
 */

export type ColumnPriority = "md" | "lg";

/** 列在窄屏隐藏：md 档平板（≥768px）起显示，lg 档桌面（≥1024px）起显示；不传则常驻 */
export function priorityClass(priority?: ColumnPriority): string {
  if (priority === "md") return "hidden md:table-cell";
  if (priority === "lg") return "hidden lg:table-cell";
  return "";
}

const INTERACTIVE =
  "button,a,input,label,select,textarea,[role=switch],[role=checkbox],[role=menuitem],[data-row-detail-ignore]";

/** 点击是否应打开行详情：落在行内交互元素（开关、按钮、链接、显式忽略区）上时不打开 */
export function shouldOpenRowDetail(target: EventTarget | null, row: Element): boolean {
  const hit = (target as Element | null)?.closest?.(INTERACTIVE) ?? null;
  return hit === null || hit === row;
}

/** 给 TableRow 展开的属性：启用时整行可点、可 Tab 聚焦、Enter/空格打开；未启用返回空对象 */
export function rowDetailProps(
  enabled: boolean,
  onOpen: () => void,
  label: string,
): HTMLAttributes<HTMLTableRowElement> & { "data-row-detail"?: string } {
  if (!enabled) return {};
  return {
    role: "button",
    tabIndex: 0,
    "aria-label": label,
    // 样式由 TableRow 按 data-row-detail 提供，这里不传 className，避免覆盖调用方的行样式
    "data-row-detail": "",
    onClick: (e: MouseEvent<HTMLTableRowElement>) => {
      if (shouldOpenRowDetail(e.target, e.currentTarget)) onOpen();
    },
    onKeyDown: (e: KeyboardEvent<HTMLTableRowElement>) => {
      if ((e.key === "Enter" || e.key === " ") && e.target === e.currentTarget) {
        e.preventDefault();
        onOpen();
      }
    },
  };
}
