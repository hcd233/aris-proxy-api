"use client";

import * as React from "react";
import { ChevronDown, Check } from "lucide-react";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { cn } from "@/lib/utils";

export interface MultiSelectPillProps {
  label: string;
  options: string[];
  value: string[];
  onChange: (next: string[]) => void;
  searchable?: boolean;
  emptyText?: string;
  className?: string;
  /** display label for option value (default = value itself) */
  formatOption?: (value: string) => string;
}

export function MultiSelectPill({
  label,
  options,
  value,
  onChange,
  searchable,
  emptyText = "No options in current range",
  className,
  formatOption,
}: MultiSelectPillProps) {
  const [open, setOpen] = React.useState(false);
  const [query, setQuery] = React.useState("");

  const showSearch = searchable ?? options.length > 8;
  const active = value.length > 0;

  const filtered = React.useMemo(() => {
    if (!query) return options;
    const q = query.toLowerCase();
    return options.filter((o) => o.toLowerCase().includes(q));
  }, [options, query]);

  const toggle = (v: string) => {
    if (value.includes(v)) {
      onChange(value.filter((x) => x !== v));
    } else {
      onChange([...value, v]);
    }
  };

  const clearLocal = () => onChange([]);

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger
        render={
          <button
            type="button"
            className={cn(
              "inline-flex h-8 items-center gap-1.5 rounded-full border px-3 text-sm transition-colors",
              active
                ? "border-ring/40 bg-accent/60 text-foreground hover:bg-accent"
                : "border-input bg-transparent text-muted-foreground hover:text-foreground hover:border-ring",
              className,
            )}
          >
            <span>
              {label}
              {active ? ` · ${value.length}` : ""}
            </span>
            <ChevronDown className="size-3.5" />
          </button>
        }
      />
      <PopoverContent
        align="start"
        className="w-auto min-w-[200px] max-w-[280px] p-1"
      >
        {showSearch && (
          <div className="px-1 pb-1">
            <input
              type="text"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Filter\u2026"
              className="h-7 w-full rounded-md border border-input bg-transparent px-2 text-xs placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring/40 focus-visible:outline-none"
            />
          </div>
        )}

        <div className="max-h-64 overflow-y-auto">
          {filtered.length === 0 ? (
            <div className="py-4 text-center text-xs text-muted-foreground">
              {emptyText}
            </div>
          ) : (
            filtered.map((opt) => {
              const checked = value.includes(opt);
              return (
                <button
                  key={opt}
                  type="button"
                  onClick={() => toggle(opt)}
                  className="flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-left text-sm hover:bg-accent hover:text-accent-foreground"
                >
                  <span
                    aria-hidden
                    className={cn(
                      "flex size-4 shrink-0 items-center justify-center rounded border",
                      checked
                        ? "border-primary bg-primary text-primary-foreground"
                        : "border-muted-foreground/30",
                    )}
                  >
                    {checked && <Check className="size-3" />}
                  </span>
                  <span className="truncate">
                    {formatOption ? formatOption(opt) : opt}
                  </span>
                </button>
              );
            })
          )}
        </div>

        {active && (
          <>
            <div className="-mx-1 my-1 h-px bg-border" />
            <button
              type="button"
              onClick={clearLocal}
              className="w-full rounded-sm px-2 py-1.5 text-left text-xs text-muted-foreground hover:bg-accent hover:text-foreground"
            >
              Clear selection
            </button>
          </>
        )}
      </PopoverContent>
    </Popover>
  );
}
