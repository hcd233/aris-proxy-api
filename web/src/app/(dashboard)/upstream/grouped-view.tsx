"use client";

import { Fragment } from "react";
import { usePersistentState } from "@/hooks/use-persistent-state";
import { Button } from "@/components/ui/button";
import { CopyButton } from "@/components/ui/copy-button";
import { Badge } from "@/components/ui/badge";
import { TooltipRoot, TooltipTrigger, TooltipContent } from "@/components/ui/tooltip";
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
import { Plus, Pencil, ChevronRight, Coins, Info, Lock } from "lucide-react";
import { useT } from "@/lib/i18n";
import { cn } from "@/lib/utils";
import type { UpstreamGroupItem, UpstreamModelItem, UpstreamEndpointItem } from "@/lib/types";
import {
  CapabilityBadges,
  ConfigMissingBadges,
  ModelActionsCell,
  ModelAliasCell,
  ModelIdCell,
  OwnerCell,
  PricingInline,
  SpecBadges,
  UpstreamModelCell,
} from "./shared";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";
import { oauthBadgeKey, quotaPercent } from "./oauth-form";

/** 组头的连通详情悬浮层：URL 可完整换行，不占列表列宽；订阅端点附账号/额度与运维动作 */
function EndpointDetailPopover({
  endpoint,
  onReauthOAuth,
  onProbeOAuthQuota,
}: {
  endpoint: UpstreamEndpointItem;
  onReauthOAuth: (ep: UpstreamEndpointItem) => void;
  onProbeOAuthQuota: (ep: UpstreamEndpointItem) => void;
}) {
  const t = useT();

  // copyable=false 的字段（掩码凭据、创建日期）复制无意义，渲染为纯文本
  const field = (label: string, value: string, copyable = true) => (
    <div className="min-w-0 rounded-md border border-border bg-background px-2 py-1.5">
      <p className="text-[10px] uppercase tracking-[0.08em] text-muted-foreground">{label}</p>
      {value ? (
        copyable ? (
          <div className="mt-0.5 flex items-start gap-1">
            <span className="min-w-0 flex-1 font-mono text-[11px] break-all text-foreground">
              {value}
            </span>
            <CopyButton value={value} variant="icon" className="-mt-1 -mr-1" />
          </div>
        ) : (
          <p className="mt-0.5 font-mono text-[11px] break-all text-foreground">{value}</p>
        )
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
          {field(t("endpoints.api_key"), endpoint.maskedAPIKey, false)}
          {endpoint.oauth && (
            <div className="min-w-0 rounded-md border border-border bg-background px-2 py-1.5">
              <p className="text-[10px] uppercase tracking-[0.08em] text-muted-foreground">
                {t("oauth.account")}
              </p>
              <p className="mt-0.5 text-[11px] break-all text-foreground">
                {endpoint.oauth.accountEmail || "—"}
                {endpoint.oauth.planType ? ` · ${endpoint.oauth.planType}` : ""}
              </p>
              <p className="mt-0.5 text-[11px]">
                <span
                  className={
                    endpoint.oauth.status === "active"
                      ? "text-muted-foreground"
                      : "font-medium text-destructive"
                  }
                >
                  {endpoint.oauth.status === "active"
                    ? t("oauth.status_active")
                    : t("oauth.status_reauth")}
                </span>
              </p>
              {(endpoint.oauth.quota?.windows ?? []).map((w) => (
                <div key={w.key} className="mt-1.5">
                  <div className="flex items-center justify-between text-[10px] text-muted-foreground">
                    <span>{t(`oauth.window_${w.key}`)}</span>
                    <span>{quotaPercent(w.usedPercent)}%</span>
                  </div>
                  <div className="mt-0.5 h-1 w-full rounded bg-muted">
                    <div
                      className="h-1 rounded bg-primary"
                      style={{ width: `${quotaPercent(w.usedPercent)}%` }}
                    />
                  </div>
                </div>
              ))}
              <div className="mt-2 flex gap-1.5">
                <Button
                  size="sm"
                  variant="outline"
                  className="h-6 px-2 text-[11px]"
                  onClick={() => onReauthOAuth(endpoint)}
                >
                  {t("oauth.reauth_action")}
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  className="h-6 px-2 text-[11px]"
                  onClick={() => onProbeOAuthQuota(endpoint)}
                >
                  {t("oauth.probe_action")}
                </Button>
              </div>
            </div>
          )}
          {field(
            t("upstream.created_at"),
            endpoint.createdAt ? new Date(endpoint.createdAt).toLocaleDateString() : "",
            false,
          )}
        </div>
      </PopoverContent>
    </Popover>
  );
}

/** 组头协议徽标：图标本身不够自明（两种 OpenAI 协议同图标），悬停展示协议名 */
function ProtocolBadge({ protocol, label }: { protocol: string; label: string }) {
  return (
    <TooltipRoot>
      <TooltipTrigger
        render={
          <Badge
            variant="secondary"
            className="cursor-default gap-1 px-1 py-0 text-[10px] font-normal"
          >
            <ProviderIcon protocol={protocol} size={12} />
          </Badge>
        }
      />
      <TooltipContent side="top" align="start">
        {label}
      </TooltipContent>
    </TooltipRoot>
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
  onReauthOAuth: (ep: UpstreamEndpointItem) => void;
  onProbeOAuthQuota: (ep: UpstreamEndpointItem) => void;
  onAddModel: (ep: UpstreamEndpointItem) => void;
  onEditModel: (m: UpstreamModelItem, ep: UpstreamEndpointItem) => void;
  onPricingModel: (m: UpstreamModelItem) => void;
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
  onReauthOAuth,
  onProbeOAuthQuota,
  onAddModel,
  onEditModel,
  onPricingModel,
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
            <Collapsible
              key={ep.id}
              open={open}
              onOpenChange={() => toggleCollapse(ep.id)}
              className="rounded-lg border border-border bg-card p-4"
            >
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0 space-y-1.5">
                  <CollapsibleTrigger
                    className="flex items-center gap-1 text-muted-foreground hover:text-foreground"
                    aria-label={open ? t("upstream.collapse_group") : t("upstream.expand_group")}
                  >
                    <ChevronRight className="size-3.5 in-data-panel-open:rotate-90" />
                    <OwnerCell user={ep.user} />
                  </CollapsibleTrigger>
                  <TooltipRoot>
                    <TooltipTrigger
                      render={<p className="line-clamp-1 text-sm font-medium">{ep.name}</p>}
                    />
                    <TooltipContent side="top" align="start" className="max-w-xs break-all">
                      {ep.name}
                    </TooltipContent>
                  </TooltipRoot>
                  <div className="flex flex-wrap items-center gap-1.5">
                    {oauthBadgeKey(ep.authType) && (
                      <Badge variant="secondary" className="px-1.5 py-0 text-[10px] font-normal">
                        {t(oauthBadgeKey(ep.authType) ?? "")}
                      </Badge>
                    )}
                    {ep.oauth?.status === "reauth_required" && (
                      <Badge variant="destructive" className="px-1.5 py-0 text-[10px] font-normal">
                        {t("oauth.status_reauth")}
                      </Badge>
                    )}
                    <EndpointDetailPopover
                      endpoint={ep}
                      onReauthOAuth={onReauthOAuth}
                      onProbeOAuthQuota={onProbeOAuthQuota}
                    />
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
                  {/* demo 只读账户写入口统一锁定（DeleteButton 的 locked 模式） */}
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    disabled={isDemo}
                    onClick={() => onEditEndpoint(ep)}
                    className="text-muted-foreground hover:text-foreground"
                  >
                    {isDemo ? <Lock className="size-3.5" /> : <Pencil className="size-3.5" />}
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
                    disabled={isDemo}
                    onClick={() => onAddModel(ep)}
                  >
                    {isDemo ? <Lock className="size-3" /> : <Plus className="size-3" />}
                    {t("upstream.add_model")}
                  </Button>
                </div>
              </div>
              <CollapsibleContent>
                <div className="mt-3 divide-y divide-border">
                  {group.models.length === 0 ? (
                    <p className="py-2 text-xs text-muted-foreground">
                      {t("upstream.no_models_in_group")}
                    </p>
                  ) : (
                    group.models.map((m) => (
                      <div key={m.id} className="flex items-center justify-between gap-2 py-2">
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
                            <TooltipContent side="top" align="start" className="max-w-xs break-all">
                              {m.upstreamModel}
                            </TooltipContent>
                          </TooltipRoot>
                          <div className="mt-1 flex items-center gap-1.5">
                            <SpecBadges
                              contextLength={m.contextLength}
                              maxOutputTokens={m.maxOutputTokens}
                            />
                            <CapabilityBadges capabilities={m.capabilities} />
                            <PricingInline pricing={m.pricing} />
                            <ConfigMissingBadges missing={m.configMissing} />
                          </div>
                        </div>
                        <div className="flex shrink-0 flex-col items-end gap-1.5">
                          <Switch
                            size="sm"
                            checked={m.enabled}
                            disabled={togglePending || isDemo}
                            onCheckedChange={() => onToggleEnabled(m)}
                            aria-label={m.enabled ? t("models.enabled") : t("models.disabled")}
                          />
                          <div className="flex items-center gap-1">
                            <Button
                              variant="ghost"
                              size="icon-sm"
                              disabled={isDemo}
                              onClick={() => onEditModel(m, ep)}
                              className="text-muted-foreground hover:text-foreground"
                            >
                              {isDemo ? (
                                <Lock className="size-3.5" />
                              ) : (
                                <Pencil className="size-3.5" />
                              )}
                            </Button>
                            <Button
                              variant="ghost"
                              size="icon-sm"
                              disabled={isDemo}
                              onClick={() => onPricingModel(m)}
                              aria-label={t("upstream.pricing.title")}
                              className="text-muted-foreground hover:text-foreground"
                            >
                              {isDemo ? (
                                <Lock className="size-3.5" />
                              ) : (
                                <Coins className="size-3.5" />
                              )}
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
              </CollapsibleContent>
            </Collapsible>
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
          <TableHead>{t("upstream.col_id")}</TableHead>
          <TableHead>{t("upstream.col_upstream")}</TableHead>
          <TableHead>{t("upstream.col_spec")}</TableHead>
          <TableHead>{t("upstream.col_capabilities")}</TableHead>
          <TableHead>{t("upstream.col_pricing")}</TableHead>
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
                <TableCell colSpan={9} className="border-l-[3px] border-l-primary pl-3">
                  <div className="flex min-w-0 items-center gap-2.5 py-0.5">
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon-xs"
                      aria-expanded={open}
                      aria-label={open ? t("upstream.collapse_group") : t("upstream.expand_group")}
                      onClick={() => toggleCollapse(ep.id)}
                    >
                      <ChevronRight className={cn("transition-transform", open && "rotate-90")} />
                    </Button>
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
                          <ProtocolBadge
                            protocol="openai-chat-completion"
                            label={t("endpoints.openai_chat_label")}
                          />
                        )}
                        {ep.supportOpenAIResponse && (
                          <ProtocolBadge
                            protocol="openai-response"
                            label={t("endpoints.openai_response_label")}
                          />
                        )}
                        {ep.supportAnthropicMessage && (
                          <ProtocolBadge
                            protocol="anthropic-message"
                            label={t("endpoints.anthropic_messages_label")}
                          />
                        )}
                        {ep.supportOpenAIDecision && (
                          <ProtocolBadge
                            protocol="openai-decision"
                            label={t("endpoints.openai_decision_label")}
                          />
                        )}
                      </span>
                      <EndpointDetailPopover
                        endpoint={ep}
                        onReauthOAuth={onReauthOAuth}
                        onProbeOAuthQuota={onProbeOAuthQuota}
                      />
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
                        disabled={isDemo}
                        onClick={() => onEditEndpoint(ep)}
                        aria-label={t("common.edit")}
                        className="text-muted-foreground hover:text-foreground"
                      >
                        {isDemo ? <Lock className="size-3.5" /> : <Pencil className="size-3.5" />}
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
                        disabled={isDemo}
                        onClick={() => onAddModel(ep)}
                      >
                        {isDemo ? <Lock className="size-3" /> : <Plus className="size-3" />}
                        {t("upstream.add_model")}
                      </Button>
                    </span>
                  </div>
                </TableCell>
              </TableRow>

              {open && group.models.length === 0 && (
                <TableRow>
                  <TableCell colSpan={9} className="pl-8 text-xs text-muted-foreground">
                    {t("upstream.no_models_in_group")}
                  </TableCell>
                </TableRow>
              )}

              {open &&
                group.models.map((m) => (
                  <TableRow key={m.id} className="hover:bg-muted/40">
                    {/* 虚线树枝 + 缩进，让归属关系不依赖背景色；停用行只降权内容列，操作列保持可点的视觉 */}
                    <ModelAliasCell
                      alias={m.alias}
                      enabled={m.enabled}
                      onCopyAlias={onCopyAlias}
                      className="border-l border-dashed border-border pl-8"
                    />
                    <ModelIdCell modelId={m.modelId} alias={m.alias} enabled={m.enabled} />
                    <UpstreamModelCell upstreamModel={m.upstreamModel} enabled={m.enabled} />
                    <TableCell className={cn(!m.enabled && "opacity-45")}>
                      <SpecBadges
                        contextLength={m.contextLength}
                        maxOutputTokens={m.maxOutputTokens}
                      />
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
                        disabled={togglePending || isDemo}
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
                      onEdit={() => onEditModel(m, ep)}
                      onPricing={() => onPricingModel(m)}
                      onDelete={() => onDeleteModel(m)}
                    />
                  </TableRow>
                ))}
            </Fragment>
          );
        })}
      </TableBody>
    </Table>
  );
}
