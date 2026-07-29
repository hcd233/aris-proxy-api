"use client";

import { Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

type DeleteButtonProps = Omit<
  React.ComponentProps<typeof Button>,
  "children" | "variant" | "size"
> & {
  label: string;
};

/**
 * 管理列表（表格/卡片行内）的统一删除按钮：
 * destructive 实心小按钮 + 垃圾桶图标 + 文字标签。
 */
export function DeleteButton({ label, ...props }: DeleteButtonProps) {
  return (
    <Button variant="destructive" size="xs" {...props}>
      <Trash2 />
      {label}
    </Button>
  );
}

/**
 * 信息流/详情页的统一删除按钮：
 * ghost 图标按钮，hover 时图标变红。
 */
export function DeleteIconButton({
  className,
  iconClassName,
  ...props
}: Omit<React.ComponentProps<typeof Button>, "children" | "variant" | "size"> & {
  iconClassName?: string;
}) {
  return (
    <Button
      variant="ghost"
      size="icon-sm"
      className={cn("text-muted-foreground hover:text-destructive", className)}
      {...props}
    >
      <Trash2 className={cn("size-4", iconClassName)} />
    </Button>
  );
}
