"use client";

import * as React from "react";

import { cn } from "@/lib/utils";

/**
 * 表格容器。
 *
 * `fit` 打开「列宽自适应」：改用 `table-layout: fixed`，列宽由表头 `w-*` 声明决定，
 * 未声明宽度的列自动吃掉剩余空间——约定只留一列不声明（主名列），其余列都声明宽度。
 * 单元格内容溢出时裁剪而不是把表格撑宽，列多的表格因此不必左右滑动即可看全。
 * 兜底：容器保留 `overflow-x-auto`，只有窄到列宽预算放不下时才出现横向滚动。
 *
 * 注意：声明宽度之和超过容器宽度时 fixed 布局会溢出（表现为横向滚动），
 * 新增列前先核对预算；内容需要省略号提示时按项目约定给单元格配 Tooltip。
 */
function Table({
  className,
  fit = false,
  ...props
}: React.ComponentProps<"table"> & { fit?: boolean }) {
  return (
    <div
      data-slot="table-container"
      data-fit={fit ? "" : undefined}
      className={cn(
        "relative w-full overflow-x-auto",
        fit &&
          "[&_[data-slot=table-cell]]:overflow-hidden [&_[data-slot=table-cell]]:text-ellipsis",
      )}
    >
      <table
        data-slot="table"
        className={cn("w-full caption-bottom text-sm", fit && "table-fixed", className)}
        {...props}
      />
    </div>
  );
}

function TableHeader({ className, ...props }: React.ComponentProps<"thead">) {
  return (
    <thead
      data-slot="table-header"
      className={cn("border-b border-border bg-secondary/50", className)}
      {...props}
    />
  );
}

function TableBody({ className, ...props }: React.ComponentProps<"tbody">) {
  return (
    <tbody
      data-slot="table-body"
      className={cn("[&_tr:last-child]:border-0", className)}
      {...props}
    />
  );
}

function TableFooter({ className, ...props }: React.ComponentProps<"tfoot">) {
  return (
    <tfoot
      data-slot="table-footer"
      className={cn("border-t bg-muted/50 font-medium [&>tr]:last:border-b-0", className)}
      {...props}
    />
  );
}

function TableRow({ className, ...props }: React.ComponentProps<"tr">) {
  return (
    <tr
      data-slot="table-row"
      className={cn(
        "border-b transition-colors hover:bg-muted/60 has-aria-expanded:bg-muted/60 data-[state=selected]:bg-muted",
        className,
      )}
      {...props}
    />
  );
}

function TableHead({ className, ...props }: React.ComponentProps<"th">) {
  return (
    <th
      data-slot="table-head"
      className={cn(
        "h-11 px-3 text-left align-middle text-xs font-semibold uppercase tracking-[0.08em] whitespace-nowrap text-muted-foreground [&:has([role=checkbox])]:pr-0",
        className,
      )}
      {...props}
    />
  );
}

function TableCell({ className, ...props }: React.ComponentProps<"td">) {
  return (
    <td
      data-slot="table-cell"
      className={cn("p-2 align-middle whitespace-nowrap [&:has([role=checkbox])]:pr-0", className)}
      {...props}
    />
  );
}

function TableCaption({ className, ...props }: React.ComponentProps<"caption">) {
  return (
    <caption
      data-slot="table-caption"
      className={cn("mt-4 text-sm text-muted-foreground", className)}
      {...props}
    />
  );
}

export { Table, TableHeader, TableBody, TableFooter, TableHead, TableRow, TableCell, TableCaption };
