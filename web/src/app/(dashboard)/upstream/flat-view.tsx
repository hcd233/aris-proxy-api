"use client";

import { Button } from "@/components/ui/button";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import {
  TooltipRoot,
  TooltipTrigger,
  TooltipContent,
} from "@/components/ui/tooltip";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { DeleteButton } from "@/components/delete-button";
import { Switch } from "@/components/ui/switch";
import { ProviderIcon } from "@/components/provider-icon";
import { ListEmptyState } from "@/components/list-empty-state";
import { TableSkeleton } from "@/components/table-skeleton";
import { Pencil, ArrowUp, ArrowDown, ArrowUpDown, Layers } from "lucide-react";
import { useT } from "@/lib/i18n";
import { cn } from "@/lib/utils";
import type { ModelListSortField, ModelListItem, UpstreamUser } from "@/lib/types";
import { CapabilityBadges, SpecBadges } from "./shared";

/** 端点列：头像 + 名称。平铺视图里端点退化为普通属性列 */
function EndpointCell({ name, user }: { name: string; user?: UpstreamUser }) {
  return (
    <div className="flex min-w-0 items-center gap-1.5">
      <Avatar size="sm">
        {user?.avatar && <AvatarImage src={user.avatar} alt={name} />}
        <AvatarFallback className="text-[10px]">
          {name.charAt(0).toUpperCase() || "?"}
        </AvatarFallback>
      </Avatar>
      <TooltipRoot>
        <TooltipTrigger render={<span className="max-w-[14ch] truncate text-xs">{name}</span>} />
        <TooltipContent side="top" align="start" className="max-w-xs break-all">
          {name}
        </TooltipContent>
      </TooltipRoot>
    </div>
  );
}

export interface FlatViewProps {
  items: ModelListItem[];
  loading: boolean;
  isMobile: boolean;
  isDemo: boolean;
  sortField: ModelListSortField;
  sort: "asc" | "desc";
  onSort: (field: ModelListSortField) => void;
  onToggleEnabled: (m: ModelListItem) => void;
  onEditModel: (m: ModelListItem) => void;
  onDeleteModel: (m: ModelListItem) => void;
  onCopyAlias: (alias: string) => void;
  deletingModelID?: number;
}

/** 可排序列头：箭头指示当前排序列与方向 */
function SortableHead({
  label,
  field,
  sortField,
  sort,
  onSort,
}: {
  label: string;
  field: ModelListSortField;
  sortField: ModelListSortField;
  sort: "asc" | "desc";
  onSort: (f: ModelListSortField) => void;
}) {
  const t = useT();
  const active = sortField === field;
  return (
    <TableHead>
      <button
        type="button"
        className="inline-flex items-center gap-1 uppercase tracking-[0.08em] hover:text-foreground"
        aria-label={`${label} ${sort === "asc" ? t("upstream.sort_asc") : t("upstream.sort_desc")}`}
        onClick={() => onSort(field)}
      >
        {label}
        {active ? (
          sort === "asc" ? (
            <ArrowUp className="size-3" />
          ) : (
            <ArrowDown className="size-3" />
          )
        ) : (
          <ArrowUpDown className="size-3 opacity-40" />
        )}
      </button>
    </TableHead>
  );
}

/** 平铺视图：模型为行、端点为属性列，支持 SQL 级排序与真分页 */
export function FlatView({
  items,
  loading,
  isMobile,
  isDemo,
  sortField,
  sort,
  onSort,
  onToggleEnabled,
  onEditModel,
  onDeleteModel,
  onCopyAlias,
  deletingModelID,
}: FlatViewProps) {
  const t = useT();

  if (loading) return <TableSkeleton />;

  if (items.length === 0) {
    return (
      <ListEmptyState
        icon={<Layers className="mb-3 size-10 text-muted-foreground/40" />}
        message={t("upstream.empty")}
      />
    );
  }

  if (isMobile) {
    return (
      <div className="space-y-3">
        {items.map((m) => (
          <div
            key={m.id}
            className={cn(
              "rounded-lg border border-border bg-card p-3",
              !m.enabled && "opacity-45",
            )}
          >
            <div className="flex items-start justify-between gap-2">
              <div className="min-w-0">
                <p className="flex items-center gap-1.5 text-sm font-medium">
                  <ProviderIcon protocol={m.alias} size={14} className="shrink-0" />
                  <TooltipRoot>
                    <TooltipTrigger
                      render={
                        <span
                          className={cn(
                            "cursor-pointer truncate underline-offset-2 hover:underline",
                            !m.enabled && "line-through",
                          )}
                          onClick={() => onCopyAlias(m.alias)}
                        >
                          {m.alias}
                        </span>
                      }
                    />
                    <TooltipContent side="top" align="start" className="max-w-xs break-all">
                      {m.alias}
                    </TooltipContent>
                  </TooltipRoot>
                </p>
                <TooltipRoot>
                  <TooltipTrigger
                    render={
                      <p className="mt-0.5 truncate font-mono text-xs text-muted-foreground">
                        {m.upstreamModel}
                      </p>
                    }
                  />
                  <TooltipContent side="top" align="start" className="max-w-xs break-all">
                    {m.upstreamModel}
                  </TooltipContent>
                </TooltipRoot>
              </div>
              <div className="flex shrink-0 flex-col items-end gap-1.5">
                <Switch
                  size="sm"
                  checked={m.enabled}
                  onCheckedChange={() => onToggleEnabled(m)}
                  aria-label={m.enabled ? t("models.enabled") : t("models.disabled")}
                />
                <div className="flex items-center gap-1">
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    onClick={() => onEditModel(m)}
                    className="text-muted-foreground hover:text-foreground"
                  >
                    <Pencil className="size-3.5" />
                  </Button>
                  <DeleteButton
                    label={t("common.delete")}
                    locked={isDemo}
                    disabled={deletingModelID === m.id}
                    onClick={() => onDeleteModel(m)}
                  />
                </div>
              </div>
            </div>
            <div className="mt-2 flex flex-wrap items-center gap-1.5">
              {m.endpoint && <EndpointCell name={m.endpoint.name} user={m.user} />}
              <SpecBadges
                contextLength={m.contextLength}
                maxOutputTokens={m.maxOutputTokens}
              />
              <CapabilityBadges capabilities={m.capabilities} />
            </div>
          </div>
        ))}
      </div>
    );
  }

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <SortableHead
            label={t("upstream.col_model")}
            field="alias"
            sortField={sortField}
            sort={sort}
            onSort={onSort}
          />
          <TableHead>{t("upstream.col_upstream")}</TableHead>
          <SortableHead
            label={t("upstream.col_endpoint")}
            field="endpoint_id"
            sortField={sortField}
            sort={sort}
            onSort={onSort}
          />
          <SortableHead
            label={t("upstream.col_spec")}
            field="context_length"
            sortField={sortField}
            sort={sort}
            onSort={onSort}
          />
          <TableHead>{t("upstream.col_capabilities")}</TableHead>
          <SortableHead
            label={t("upstream.col_status")}
            field="enabled"
            sortField={sortField}
            sort={sort}
            onSort={onSort}
          />
          <SortableHead
            label={t("upstream.col_created")}
            field="created_at"
            sortField={sortField}
            sort={sort}
            onSort={onSort}
          />
          <TableHead className="text-right">{t("upstream.col_actions")}</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {items.map((m) => (
          <TableRow key={m.id} className={cn("hover:bg-muted/40", !m.enabled && "opacity-45")}>
            <TableCell>
              <div className="flex min-w-0 items-center gap-1.5">
                <ProviderIcon protocol={m.alias} size={14} className="shrink-0" />
                <TooltipRoot>
                  <TooltipTrigger
                    render={
                      <span
                        className={cn(
                          "max-w-[16ch] cursor-pointer truncate font-medium underline-offset-2 hover:underline",
                          !m.enabled && "line-through",
                        )}
                        onClick={() => onCopyAlias(m.alias)}
                      >
                        {m.alias}
                      </span>
                    }
                  />
                  <TooltipContent side="top" align="start" className="max-w-xs break-all">
                    {`${t("models.click_to_copy")}: ${m.alias}`}
                  </TooltipContent>
                </TooltipRoot>
                {m.modelId && m.modelId !== m.alias && (
                  <TooltipRoot>
                    <TooltipTrigger
                      render={
                        <span className="max-w-[10ch] shrink-0 truncate font-mono text-[10px] text-muted-foreground">
                          {`· id: ${m.modelId}`}
                        </span>
                      }
                    />
                    <TooltipContent side="top" align="start" className="max-w-xs break-all">
                      {m.modelId}
                    </TooltipContent>
                  </TooltipRoot>
                )}
              </div>
            </TableCell>
            <TableCell className="font-mono text-xs">
              <TooltipRoot>
                <TooltipTrigger
                  render={<span className="block max-w-[20ch] truncate">{m.upstreamModel}</span>}
                />
                <TooltipContent side="top" align="start" className="max-w-xs break-all">
                  {m.upstreamModel}
                </TooltipContent>
              </TooltipRoot>
            </TableCell>
            <TableCell>
              {m.endpoint ? (
                <EndpointCell name={m.endpoint.name} user={m.user} />
              ) : (
                <span className="text-muted-foreground">—</span>
              )}
            </TableCell>
            <TableCell>
              <SpecBadges
                contextLength={m.contextLength}
                maxOutputTokens={m.maxOutputTokens}
              />
            </TableCell>
            <TableCell>
              <CapabilityBadges capabilities={m.capabilities} />
            </TableCell>
            <TableCell>
              <Switch
                size="sm"
                checked={m.enabled}
                onCheckedChange={() => onToggleEnabled(m)}
                aria-label={m.enabled ? t("models.enabled") : t("models.disabled")}
              />
            </TableCell>
            <TableCell className="text-muted-foreground">
              {new Date(m.createdAt).toLocaleDateString()}
            </TableCell>
            <TableCell className="text-right">
              <div className="flex items-center justify-end gap-1">
                <Button
                  variant="ghost"
                  size="icon-sm"
                  onClick={() => onEditModel(m)}
                  aria-label={t("common.edit")}
                  className="text-muted-foreground hover:text-foreground"
                >
                  <Pencil className="size-3.5" />
                </Button>
                <DeleteButton
                  label={t("common.delete")}
                  locked={isDemo}
                  disabled={deletingModelID === m.id}
                  onClick={() => onDeleteModel(m)}
                />
              </div>
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}

