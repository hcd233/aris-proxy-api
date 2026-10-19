"use client";

import { Button } from "@/components/ui/button";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { TooltipRoot, TooltipTrigger, TooltipContent } from "@/components/ui/tooltip";
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
import { Pencil, ArrowUp, ArrowDown, ArrowUpDown, Coins, Layers, Lock } from "lucide-react";
import { useT } from "@/lib/i18n";
import { cn } from "@/lib/utils";
import type { ModelListSortField, ModelListItem, UpstreamUser } from "@/lib/types";
import {
  CapabilityBadges,
  ConfigMissingBadges,
  ModelActionsCell,
  ModelIdentityCell,
  PricingInline,
  SpecBadges,
} from "./shared";

/**
 * 端点列：端点名 + 该行模型归属用户的头像。
 * 平铺视图端点退化为属性列；头像与回退字母均取归属用户（对齐 shared.tsx 的 OwnerCell），
 * 端点名只作展示文本与无归属时的回退。
 */
function EndpointOwnerCell({ name, user }: { name: string; user?: UpstreamUser }) {
  return (
    <div className="flex min-w-0 items-center gap-1.5">
      <Avatar size="sm">
        {user?.avatar && <AvatarImage src={user.avatar} alt={user.name} />}
        <AvatarFallback className="text-[10px]">
          {(user?.name || name).charAt(0).toUpperCase() || "?"}
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
  onPricingModel: (m: ModelListItem) => void;
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
  className,
}: {
  label: string;
  field: ModelListSortField;
  sortField: ModelListSortField;
  sort: "asc" | "desc";
  onSort: (f: ModelListSortField) => void;
  /** 自适应列宽预算：与普通表头一样用 w-* 声明 */
  className?: string;
}) {
  const t = useT();
  const active = sortField === field;
  return (
    <TableHead className={className}>
      <Button
        type="button"
        variant="ghost"
        size="xs"
        className="-ml-2 min-w-0 uppercase tracking-[0.08em]"
        aria-label={`${label} ${sort === "asc" ? t("upstream.sort_asc") : t("upstream.sort_desc")}`}
        onClick={() => onSort(field)}
      >
        {label}
        {active ? (
          sort === "asc" ? (
            <ArrowUp data-icon="inline-end" />
          ) : (
            <ArrowDown data-icon="inline-end" />
          )
        ) : (
          <ArrowUpDown data-icon="inline-end" className="opacity-40" />
        )}
      </Button>
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
  onPricingModel,
  onDeleteModel,
  onCopyAlias,
  deletingModelID,
}: FlatViewProps) {
  const t = useT();

  if (loading) return <TableSkeleton />;

  if (items.length === 0) {
    return <ListEmptyState icon={Layers} message={t("upstream.empty")} />;
  }

  if (isMobile) {
    return (
      <div className="space-y-3">
        {items.map((m) => (
          <div key={m.id} className="rounded-lg border border-border bg-card p-3">
            <div className="flex items-start justify-between gap-2">
              <div className={cn("min-w-0", !m.enabled && "opacity-45")}>
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
                  disabled={isDemo}
                  onCheckedChange={() => onToggleEnabled(m)}
                  aria-label={m.enabled ? t("models.enabled") : t("models.disabled")}
                />
                <div className="flex items-center gap-1">
                  {/* demo 只读账户写入口统一锁定（DeleteButton 的 locked 模式） */}
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    disabled={isDemo}
                    onClick={() => onEditModel(m)}
                    className="text-muted-foreground hover:text-foreground"
                  >
                    {isDemo ? <Lock className="size-3.5" /> : <Pencil className="size-3.5" />}
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    disabled={isDemo}
                    onClick={() => onPricingModel(m)}
                    aria-label={t("upstream.pricing.title")}
                    className="text-muted-foreground hover:text-foreground"
                  >
                    {isDemo ? <Lock className="size-3.5" /> : <Coins className="size-3.5" />}
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
            <div
              className={cn("mt-2 flex flex-wrap items-center gap-1.5", !m.enabled && "opacity-45")}
            >
              {m.endpoint && <EndpointOwnerCell name={m.endpoint.name} user={m.user} />}
              <SpecBadges contextLength={m.contextLength} maxOutputTokens={m.maxOutputTokens} />
              <CapabilityBadges capabilities={m.capabilities} />
              <PricingInline pricing={m.pricing} />
              <ConfigMissingBadges missing={m.configMissing} />
            </div>
          </div>
        ))}
      </div>
    );
  }

  return (
    <Table fit>
      <TableHeader>
        <TableRow>
          {/* 主列不声明宽度：固定列宽预算之和之外的空间全归它（端点名并入次行） */}
          <SortableHead
            label={t("upstream.col_model")}
            field="alias"
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
            className="w-[10rem]"
          />
          <TableHead className="w-[9rem]">{t("upstream.col_capabilities")}</TableHead>
          <TableHead className="w-[7rem]">{t("upstream.col_pricing")}</TableHead>
          <SortableHead
            label={t("upstream.col_status")}
            field="enabled"
            sortField={sortField}
            sort={sort}
            onSort={onSort}
            className="w-[3.5rem]"
          />
          <SortableHead
            label={t("upstream.col_created")}
            field="created_at"
            sortField={sortField}
            sort={sort}
            onSort={onSort}
            className="w-[5.5rem]"
          />
          <TableHead className="w-[7.5rem] text-right">{t("upstream.col_actions")}</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {items.map((m) => (
          <TableRow key={m.id} className="hover:bg-muted/40">
            {/* 停用行只降权内容列，操作列保持可点的视觉 */}
            <ModelIdentityCell
              alias={m.alias}
              modelId={m.modelId}
              upstreamModel={m.upstreamModel}
              endpointName={m.endpoint?.name}
              enabled={m.enabled}
              onCopyAlias={onCopyAlias}
            />
            <TableCell className={cn(!m.enabled && "opacity-45")}>
              <SpecBadges contextLength={m.contextLength} maxOutputTokens={m.maxOutputTokens} />
            </TableCell>
            <TableCell className={cn(!m.enabled && "opacity-45")}>
              <CapabilityBadges capabilities={m.capabilities} />
            </TableCell>
            <TableCell className={cn(!m.enabled && "opacity-45")}>
              {/* 与兄弟列同形裹一层 flex：价格与徽标才能同行且有固定间距 */}
              <div className="flex items-center gap-1.5">
                <PricingInline pricing={m.pricing} />
                <ConfigMissingBadges missing={m.configMissing} />
              </div>
            </TableCell>
            <TableCell>
              <Switch
                size="sm"
                checked={m.enabled}
                disabled={isDemo}
                onCheckedChange={() => onToggleEnabled(m)}
                aria-label={m.enabled ? t("models.enabled") : t("models.disabled")}
              />
            </TableCell>
            <TableCell className="text-muted-foreground">
              {new Date(m.createdAt).toLocaleDateString()}
            </TableCell>
            <ModelActionsCell
              isDemo={isDemo}
              deleting={deletingModelID === m.id}
              onEdit={() => onEditModel(m)}
              onPricing={() => onPricingModel(m)}
              onDelete={() => onDeleteModel(m)}
            />
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
