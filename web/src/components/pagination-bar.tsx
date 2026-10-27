"use client";

import { useRef } from "react";
import type { PageInfo } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Pagination, PaginationContent, PaginationItem } from "@/components/ui/pagination";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { ChevronLeft, ChevronRight, ListFilter } from "lucide-react";
import { cn, formatInteger } from "@/lib/utils";
import { useT } from "@/lib/i18n";

const PAGE_SIZES = [20, 50, 100, 200, 500];

interface PaginationBarProps {
  pageInfo: PageInfo;
  onChange: (page: number, pageSize: number) => void;
  totalLabel?: string;
  className?: string;
}

export function PaginationBar({
  pageInfo,
  onChange,
  totalLabel: rawTotalLabel,
  className = "",
}: PaginationBarProps) {
  const totalPages = Math.max(1, Math.ceil(pageInfo.total / pageInfo.pageSize));
  const inputRef = useRef<HTMLInputElement>(null);
  const t = useT();
  const totalLabel = rawTotalLabel ?? t("pagination.items");

  if (pageInfo.total <= 0) return null;

  const resolvePage = () => {
    const raw = inputRef.current?.value ?? String(pageInfo.page);
    let page = parseInt(raw, 10);
    if (Number.isNaN(page)) page = 1;
    return Math.max(1, Math.min(page, totalPages));
  };

  const handlePageInputKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter") {
      const page = resolvePage();
      if (page !== pageInfo.page) onChange(page, pageInfo.pageSize);
    }
  };

  const handlePageInputBlur = () => {
    const page = resolvePage();
    if (page !== pageInfo.page) onChange(page, pageInfo.pageSize);
  };

  return (
    <div className={cn("mt-4 flex flex-wrap items-center gap-4 md:justify-between", className)}>
      <div className="hidden items-center gap-3 md:flex">
        <DropdownMenu>
          <DropdownMenuTrigger
            render={<Button variant="outline" size="sm" className="min-w-[7.5rem]" />}
          >
            <ListFilter data-icon="inline-start" />
            {pageInfo.pageSize} {t("pagination.per_page")}
          </DropdownMenuTrigger>
          <DropdownMenuContent align="start">
            <DropdownMenuGroup>
              <DropdownMenuRadioGroup
                value={String(pageInfo.pageSize)}
                onValueChange={(value) => onChange(1, Number(value))}
              >
                {PAGE_SIZES.map((size) => (
                  <DropdownMenuRadioItem key={size} value={String(size)}>
                    {size} {t("pagination.per_page")}
                  </DropdownMenuRadioItem>
                ))}
              </DropdownMenuRadioGroup>
            </DropdownMenuGroup>
          </DropdownMenuContent>
        </DropdownMenu>
        <p className="hidden whitespace-nowrap text-sm tabular-nums text-muted-foreground md:block">
          {t("pagination.total_format")
            .replace("{count}", formatInteger(pageInfo.total))
            .replace("{label}", totalLabel)}
        </p>
      </div>

      {/* 移动端左侧工具组不渲染（hidden md:flex），此时靠 mx-auto 居中；md 起交给
       * 容器的 justify-between 右贴边。 */}
      <Pagination className="mx-auto w-auto md:mx-0">
        <PaginationContent className="gap-2">
          <PaginationItem>
            <Button
              variant="outline"
              size="sm"
              disabled={pageInfo.page <= 1}
              onClick={() => onChange(pageInfo.page - 1, pageInfo.pageSize)}
            >
              <ChevronLeft />
            </Button>
          </PaginationItem>
          <PaginationItem>
            <div className="flex items-center gap-1.5 text-sm">
              <span className="whitespace-nowrap text-muted-foreground">
                {t("pagination.page")}
              </span>
              <Input
                key={`page-${pageInfo.page}`}
                ref={inputRef}
                type="number"
                min={1}
                max={totalPages}
                defaultValue={pageInfo.page}
                className="h-8 w-14 rounded-md bg-transparent px-2 text-center tabular-nums"
                onKeyDown={handlePageInputKeyDown}
                onBlur={handlePageInputBlur}
              />
              <span className="text-muted-foreground">/ {totalPages}</span>
            </div>
          </PaginationItem>
          <PaginationItem>
            <Button
              variant="outline"
              size="sm"
              disabled={pageInfo.page >= totalPages}
              onClick={() => onChange(pageInfo.page + 1, pageInfo.pageSize)}
            >
              <ChevronRight />
            </Button>
          </PaginationItem>
        </PaginationContent>
      </Pagination>
    </div>
  );
}
