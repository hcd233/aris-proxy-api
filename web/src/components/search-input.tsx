"use client";

import { Input } from "@/components/ui/input";
import { Search, X } from "lucide-react";
import { cn } from "@/lib/utils";

interface SearchInputProps {
  value: string;
  onChange: (value: string) => void;
  /** 回车触发（通常是列表查询） */
  onSearch?: () => void;
  placeholder?: string;
  /** 显示清除按钮（需配合 onClear 使用） */
  clearable?: boolean;
  onClear?: () => void;
  /** 附加到外层相对定位容器 */
  className?: string;
}

/**
 * 列表页统一的搜索输入框：左侧放大镜图标 + pl-9 + Enter 触发 + 可选清除按钮。
 */
export function SearchInput({
  value,
  onChange,
  onSearch,
  placeholder,
  clearable,
  onClear,
  className,
}: SearchInputProps) {
  return (
    <div className={cn("relative w-full md:max-w-sm", className)}>
      <Search className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
      <Input
        placeholder={placeholder}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter") onSearch?.();
        }}
        className={cn("pl-9", clearable && "pr-8")}
      />
      {clearable && value && (
        <button
          type="button"
          onClick={onClear}
          aria-label="clear search"
          className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
        >
          <X className="size-4" />
        </button>
      )}
    </div>
  );
}
