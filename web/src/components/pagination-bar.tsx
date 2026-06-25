"use client";

import { useRef } from "react";
import type { PageInfo } from "@/lib/types";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { ChevronLeft, ChevronRight, Check, ListFilter } from "lucide-react";
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
    <div className={`mt-4 flex flex-wrap items-center justify-between gap-4 ${className}`}>
      <div className="hidden items-center gap-3 md:flex">
        <DropdownMenu>
          <DropdownMenuTrigger
            render={<Button variant="outline" size="sm" className="gap-1.5" />}
          >
            <ListFilter size={14} />
            {pageInfo.pageSize} {t("pagination.per_page")}
          </DropdownMenuTrigger>
          <DropdownMenuContent align="start">
            {PAGE_SIZES.map((size) => (
              <DropdownMenuItem key={size} onClick={() => onChange(1, size)}>
                {size === pageInfo.pageSize && <Check className="size-4" />}
                <span className={size === pageInfo.pageSize ? "ml-0" : "ml-6"}>
                  {size} {t("pagination.per_page")}
                </span>
              </DropdownMenuItem>
            ))}
          </DropdownMenuContent>
        </DropdownMenu>
        <p className="hidden text-sm text-muted-foreground md:block">
          {pageInfo.total} {totalLabel} {t("pagination.total")}
        </p>
      </div>

      <div className="flex items-center gap-2">
        <Button
          variant="outline"
          size="sm"
          disabled={pageInfo.page <= 1}
          onClick={() => onChange(pageInfo.page - 1, pageInfo.pageSize)}
        >
          <ChevronLeft className="size-4" />
        </Button>
        <div className="flex items-center gap-1.5 text-sm">
          <span className="text-muted-foreground">{t("pagination.page")}</span>
          <input
            key={`page-${pageInfo.page}`}
            ref={inputRef}
            type="number"
            min={1}
            max={totalPages}
            defaultValue={pageInfo.page}
            className="h-8 w-14 rounded-md border border-input bg-transparent px-2 py-1 text-center text-sm tabular-nums focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring/40 focus-visible:outline-none dark:bg-input/30"
            onKeyDown={handlePageInputKeyDown}
            onBlur={handlePageInputBlur}
          />
          <span className="text-muted-foreground">/ {totalPages}</span>
        </div>
        <Button
          variant="outline"
          size="sm"
          disabled={pageInfo.page >= totalPages}
          onClick={() => onChange(pageInfo.page + 1, pageInfo.pageSize)}
        >
          <ChevronRight className="size-4" />
        </Button>
      </div>
    </div>
  );
}
