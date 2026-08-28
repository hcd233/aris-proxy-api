"use client";

import { Fragment } from "react";
import { usePersistentState } from "@/hooks/use-persistent-state";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
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
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
  PopoverHeader,
  PopoverTitle,
  PopoverDescription,
} from "@/components/ui/popover";
import { DeleteButton } from "@/components/delete-button";
import { Switch } from "@/components/ui/switch";
import { ProviderIcon } from "@/components/provider-icon";
import { Plus, Pencil, ChevronDown, ChevronRight, Info, Copy } from "lucide-react";
import { useT } from "@/lib/i18n";
import { copyTextToClipboard } from "@/lib/clipboard";
import { toast } from "sonner";
import { cn } from "@/lib/utils";
import type { UpstreamGroupItem, UpstreamModelItem, UpstreamEndpointItem } from "@/lib/types";
import { CapabilityBadges, OwnerCell, SpecBadges } from "./shared";

/** 组头的连通详情悬浮层：URL 可完整换行，不占列表列宽 */
function EndpointDetailPopover({ endpoint }: { endpoint: UpstreamEndpointItem }) {
  const t = useT();

  const copy = (text: string) => {
    if (!text) return;
    void copyTextToClipboard(text).then((ok) =>
      ok ? toast.success(t("common.copied_to_clipboard")) : toast.error(t("common.copy_failed")),
    );
  };

  const field = (label: string, value: string) => (
    <div className="min-w-0 rounded-md border border-border bg-background px-2 py-1.5">
      <p className="text-[10px] uppercase tracking-[0.08em] text-muted-foreground">{label}</p>
      {value ? (
        <button
          type="button"
          className="mt-0.5 flex w-full items-start gap-1 text-left font-mono text-[11px] break-all text-foreground hover:text-primary"
          onClick={() => copy(value)}
        >
          <Copy className="mt-0.5 size-3 shrink-0 text-muted-foreground" />
          <span className="min-w-0">{value}</span>
        </button>
      ) : (
        <p className="mt-0.5 text-[11px] text-muted-foreground">—</p>
      )}
    </div>
  );

  return (
    <Popover>
      <PopoverTrigger
        render={
          <Badge
            variant="outline"
            className="shrink-0 cursor-pointer gap-1 border-dashed px-1.5 py-0 text-[10px] font-normal text-muted-foreground hover:text-foreground"
          >
            <Info className="size-3" />
            {t("upstream.connection_details")}
          </Badge>
        }
      />
      <PopoverContent align="start" sideOffset={8} className="w-80 p-2.5">
        <PopoverHeader className="px-0.5 pb-1.5">
          <PopoverTitle className="text-xs">{t("upstream.connection_details")}</PopoverTitle>
          <PopoverDescription className="text-[11px]">
            {t("upstream.connection_desc")}
          </PopoverDescription>
        </PopoverHeader>
        <div className="grid grid-cols-1 gap-1.5">
          {field("OpenAI", endpoint.openaiBaseURL)}
          {field("Anthropic", endpoint.anthropicBaseURL)}
          {field("API Key", endpoint.maskedAPIKey)}
          {field(
            t("upstream.created_at"),
            endpoint.createdAt ? new Date(endpoint.createdAt).toLocaleDateString() : "",
          )}
        </div>
      </PopoverContent>
    </Popover>
  );
}

export interface GroupedViewProps {
  groups: UpstreamGroupItem[];
  isMobile: boolean;
  isDemo: boolean;
  togglePending: boolean;
  onToggleEnabled: (m: UpstreamModelItem) => void;
  onEditEndpoint: (ep: UpstreamEndpointItem) => void;
  onDeleteEndpoint: (ep: UpstreamEndpointItem) => void;
  onAddModel: (ep: UpstreamEndpointItem) => void;
  onEditModel: (m: UpstreamModelItem, ep: UpstreamEndpointItem) => void;
  onDeleteModel: (m: UpstreamModelItem) => void;
  onCopyAlias: (alias: string) => void;
  deletingEndpointID?: number;
  deletingModelID?: number;
}

/**
 * 分组视图：端点为组、模型为行的缩进树表。
 *
 * 相对原实现的变化见 spec「缺陷 #1–#8」：补真表头（原为 colSpan=9 空表头）、
 * 删除占位空列、组头加左侧色条、模型行画虚线树枝、停用行整行降权、
 * 截断徽标显示真实规模、组可折叠、端点连通详情收进悬浮层。
 */
export function GroupedView({
  groups,
  isMobile,
  isDemo,
  togglePending,
  onToggleEnabled,
  onEditEndpoint,
  onDeleteEndpoint,
  onAddModel,
  onEditModel,
  onDeleteModel,
  onCopyAlias,
  deletingEndpointID,
  deletingModelID,
}: GroupedViewProps) {
  const t = useT();
  const [collapsed, setCollapsed] = usePersistentState<number[]>(
    "dashboard.upstream.collapsed",
    [],
  );

  const isCollapsed = (id: number) => collapsed.includes(id);
  const toggleCollapse = (id: number) =>
    setCollapsed((prev) => (prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]));

  if (isMobile) {
    return (
      <div className="space-y-4">
        {groups.map((group) => {
          const ep = group.endpoint;
          const open = !isCollapsed(ep.id);
          return (
            <div key={ep.id} className="rounded-lg border border-border bg-card p-4">
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0 space-y-1.5">
                  <button
                    type="button"
                    className="flex items-center gap-1 text-muted-foreground hover:text-foreground"
                    aria-expanded={open}
                    aria-label={open ? t("upstream.collapse_group") : t("upstream.expand_group")}
                    onClick={() => toggleCollapse(ep.id)}
                  >
                    {open ? (
                      <ChevronDown className="size-3.5" />
                    ) : (
                      <ChevronRight className="size-3.5" />
                    )}
                    <OwnerCell user={ep.user} />
                  </button>
                  <TooltipRoot>
                    <TooltipTrigger
                      render={<p className="line-clamp-1 text-sm font-medium">{ep.name}</p>}
                    />
                    <TooltipContent side="top" align="start" className="max-w-xs break-all">
                      {ep.name}
                    </TooltipContent>
                  </TooltipRoot>
                  <div className="flex flex-wrap items-center gap-1.5">
                    <EndpointDetailPopover endpoint={ep} />
                    <span className="text-xs text-muted-foreground">
                      {t("upstream.model_count").replace("{count}", String(group.modelCount))}
                    </span>
                    {group.truncated && (
                      <Badge variant="outline" className="text-[10px] font-normal">
                        {t("upstream.truncated_detail")
                          .replace("{total}", String(group.totalModelCount ?? group.modelCount))
                          .replace("{shown}", String(group.modelCount))}
                      </Badge>
                    )}
                  </div>
                </div>
                <div className="flex shrink-0 items-center gap-1">
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    onClick={() => onEditEndpoint(ep)}
                    className="text-muted-foreground hover:text-foreground"
                  >
                    <Pencil className="size-3.5" />
                  </Button>
                  <DeleteButton
                    label={t("common.delete")}
                    locked={isDemo}
                    disabled={deletingEndpointID === ep.id}
                    onClick={() => onDeleteEndpoint(ep)}
                  />
                  <Button
                    size="sm"
                    variant="outline"
                    className="h-7 gap-1 px-2 text-xs"
                    onClick={() => onAddModel(ep)}
                  >
                    <Plus className="size-3" />
                    {t("upstream.add_model")}
                  </Button>
                </div>
              </div>
              {open && (
                <div className="mt-3 divide-y divide-border">
                  {group.models.length === 0 ? (
                    <p className="py-2 text-xs text-muted-foreground">
                      {t("upstream.no_models_in_group")}
                    </p>
                  ) : (
                    group.models.map((m) => (
                      <div
                        key={m.id}
                        className={cn(
                          "flex items-center justify-between gap-2 py-2",
                          !m.enabled && "opacity-45",
                        )}
                      >
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
                              <TooltipContent
                                side="top"
                                align="start"
                                className="max-w-xs break-all"
                              >
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
                            <TooltipContent
                              side="top"
                              align="start"
                              className="max-w-xs break-all"
                            >
                              {m.upstreamModel}
                            </TooltipContent>
                          </TooltipRoot>
                          <div className="mt-1 flex items-center gap-1.5">
                            <SpecBadges
                              contextLength={m.contextLength}
                              maxOutputTokens={m.maxOutputTokens}
                            />
                            <CapabilityBadges capabilities={m.capabilities} />
                          </div>
                        </div>
                        <div className="flex shrink-0 flex-col items-end gap-1.5">
                          <Switch
                            size="sm"
                            checked={m.enabled}
                            disabled={togglePending}
                            onCheckedChange={() => onToggleEnabled(m)}
                            aria-label={m.enabled ? t("models.enabled") : t("models.disabled")}
                          />
                          <div className="flex items-center gap-1">
                            <Button
                              variant="ghost"
                              size="icon-sm"
                              onClick={() => onEditModel(m, ep)}
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
                    ))
                  )}
                </div>
              )}
            </div>
          );
        })}
      </div>
    );
  }

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>{t("upstream.col_model")}</TableHead>
          <TableHead>{t("upstream.col_upstream")}</TableHead>
          <TableHead>{t("upstream.col_spec")}</TableHead>
          <TableHead>{t("upstream.col_status")}</TableHead>
          <TableHead>{t("upstream.col_created")}</TableHead>
          <TableHead className="text-right">{t("upstream.col_actions")}</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {groups.map((group) => {
          const ep = group.endpoint;
          const open = !isCollapsed(ep.id);
          return (
            <Fragment key={ep.id}>
              {/* 组头：白底 + 左侧主色条，与模型行区分层级 */}
              <TableRow className="bg-card hover:bg-card">
                <TableCell colSpan={6} className="border-l-[3px] border-l-primary pl-3">
                  <div className="flex min-w-0 items-center gap-2.5 py-0.5">
                    <button
                      type="button"
                      className="shrink-0 text-muted-foreground hover:text-foreground"
                      aria-expanded={open}
                      aria-label={
                        open ? t("upstream.collapse_group") : t("upstream.expand_group")
                      }
                      onClick={() => toggleCollapse(ep.id)}
                    >
                      {open ? (
                        <ChevronDown className="size-4" />
                      ) : (
                        <ChevronRight className="size-4" />
                      )}
                    </button>
                    <OwnerCell user={ep.user} />
                    <span className="flex min-w-0 items-center gap-2">
                      <TooltipRoot>
                        <TooltipTrigger
                          render={
                            <span className="max-w-[16ch] truncate font-medium">{ep.name}</span>
                          }
                        />
                        <TooltipContent side="top" align="start" className="max-w-xs break-all">
                          {ep.name}
                        </TooltipContent>
                      </TooltipRoot>
                      <span className="flex shrink-0 items-center gap-1">
                        {ep.supportOpenAIChatCompletion && (
                          <Badge
                            variant="secondary"
                            className="gap-1 px-1 py-0 text-[10px] font-normal"
                          >
                            <ProviderIcon protocol="openai-chat-completion" size={12} />
                          </Badge>
                        )}
                        {ep.supportOpenAIResponse && (
                          <Badge
                            variant="secondary"
                            className="gap-1 px-1 py-0 text-[10px] font-normal"
                          >
                            <ProviderIcon protocol="openai-response" size={12} />
                          </Badge>
                        )}
                        {ep.supportAnthropicMessage && (
                          <Badge
                            variant="secondary"
                            className="gap-1 px-1 py-0 text-[10px] font-normal"
                          >
                            <ProviderIcon protocol="anthropic-message" size={12} />
                          </Badge>
                        )}
                      </span>
                      <EndpointDetailPopover endpoint={ep} />
                      {group.truncated && (
                        <Badge variant="outline" className="shrink-0 text-[10px] font-normal">
                          {t("upstream.truncated_detail")
                            .replace("{total}", String(group.totalModelCount ?? group.modelCount))
                            .replace("{shown}", String(group.modelCount))}
                        </Badge>
                      )}
                    </span>
                    <span className="ml-auto flex items-center gap-1.5">
                      <span className="font-mono text-xs tabular-nums text-muted-foreground">
                        {t("upstream.model_count").replace("{count}", String(group.modelCount))}
                      </span>
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        onClick={() => onEditEndpoint(ep)}
                        aria-label={t("common.edit")}
                        className="text-muted-foreground hover:text-foreground"
                      >
                        <Pencil className="size-3.5" />
                      </Button>
                      <DeleteButton
                        label={t("common.delete")}
                        locked={isDemo}
                        disabled={deletingEndpointID === ep.id}
                        onClick={() => onDeleteEndpoint(ep)}
                      />
                      <Button
                        size="sm"
                        variant="outline"
                        className="h-7 gap-1 px-2 text-xs"
                        onClick={() => onAddModel(ep)}
                      >
                        <Plus className="size-3" />
                        {t("upstream.add_model")}
                      </Button>
                    </span>
                  </div>
                </TableCell>
              </TableRow>

              {open && group.models.length === 0 && (
                <TableRow>
                  <TableCell colSpan={6} className="pl-8 text-xs text-muted-foreground">
                    {t("upstream.no_models_in_group")}
                  </TableCell>
                </TableRow>
              )}

              {open &&
                group.models.map((m) => (
                  <TableRow
                    key={m.id}
                    className={cn("hover:bg-muted/40", !m.enabled && "opacity-45")}
                  >
                    {/* 虚线树枝 + 缩进，让归属关系不依赖背景色 */}
                    <TableCell className="border-l border-dashed border-border pl-8">
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
                            <TooltipContent
                              side="top"
                              align="start"
                              className="max-w-xs break-all"
                            >
                              {m.modelId}
                            </TooltipContent>
                          </TooltipRoot>
                        )}
                      </div>
                    </TableCell>
                    <TableCell className="font-mono text-xs">
                      <TooltipRoot>
                        <TooltipTrigger
                          render={
                            <span className="block max-w-[20ch] truncate">{m.upstreamModel}</span>
                          }
                        />
                        <TooltipContent side="top" align="start" className="max-w-xs break-all">
                          {m.upstreamModel}
                        </TooltipContent>
                      </TooltipRoot>
                    </TableCell>
                    <TableCell>
                      <div className="flex items-center gap-1.5">
                        <SpecBadges
                          contextLength={m.contextLength}
                          maxOutputTokens={m.maxOutputTokens}
                        />
                        <CapabilityBadges capabilities={m.capabilities} />
                      </div>
                    </TableCell>
                    <TableCell>
                      <Switch
                        size="sm"
                        checked={m.enabled}
                        disabled={togglePending}
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
                          onClick={() => onEditModel(m, ep)}
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
            </Fragment>
          );
        })}
      </TableBody>
    </Table>
  );
}
