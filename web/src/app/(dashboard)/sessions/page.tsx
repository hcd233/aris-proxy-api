"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { usePersistentState } from "@/hooks/use-persistent-state";
import { usePaginatedList } from "@/hooks/use-paginated-list";
import { api } from "@/lib/api-client";
import { showErrorToast } from "@/lib/api-error-handler";
import type { SessionSummary } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { TooltipRoot, TooltipTrigger, TooltipContent } from "@/components/ui/tooltip";
import { Badge } from "@/components/ui/badge";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Item,
  ItemActions,
  ItemContent,
  ItemFooter,
  ItemGroup,
  ItemMedia,
  ItemTitle,
} from "@/components/ui/item";
import { MessageSquare, ArrowUp, ArrowDown, Trash2, Lock } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { useAuth } from "@/lib/auth-context";
import { PermissionGuard } from "@/components/permission-guard";
import { useIsMobile } from "@/hooks/use-mobile";
import { cn, formatDateTime, truncateText } from "@/lib/utils";
import { ScoreDots } from "@/components/session-detail/score-dots";
import { PageHeader } from "@/components/page-header";
import { ListEmptyState } from "@/components/list-empty-state";
import { TableSkeleton } from "@/components/table-skeleton";
import { PaginationBar } from "@/components/pagination-bar";
import { TimeRangePicker } from "@/components/ui/time-range-picker";
import type { TimeRangeKey } from "@/lib/time-range";
import { computeRange } from "@/lib/time-range";
import { DeleteIconButton } from "@/components/delete-button";
import { DeleteConfirmDialog } from "@/components/delete-confirm-dialog";
import { useDeleteConfirm } from "@/hooks/use-delete-confirm";
import { toast } from "sonner";
import { ProviderIcon } from "@/components/provider-icon";
import { DemoAddButton } from "@/components/demo-add-button";
import { useDemoWhitelist } from "@/hooks/use-demo-whitelist";
import { FilterBar } from "@/components/filter-bar/filter-bar";
import { useFilterBar } from "@/components/filter-bar/use-filter-bar";
import type { FacetDef, FilterBarQueryParams } from "@/components/filter-bar/types";
import { TruncatedText } from "@/components/truncated-text";

type SortDir = "asc" | "desc";

/* Table-Fit 列宽预算（1280 视口容器实测 902px，1512 视口 1118px）：
 * 「摘要」是唯一不声明宽度的主列，吃剩余空间；其余列声明宽度合计必须留够它。
 * 常驻列合计 668px → 902 下摘要 ~150px，仍不够，于是把两个低价值列按容器宽度
 * 渐进显示（Progressive Columns）：窄容器先收起评分与工具数，摘要稳定在 ~200px。
 * 阈值是表格可用宽度，不是视口宽度。 */
const SHOW_SCORE_FROM = "hidden @min-[64rem]:table-cell";
const SHOW_TOOL_FROM = "hidden @min-[58rem]:table-cell";
const SHOW_AGENT_FROM = "hidden @min-[52rem]:table-cell";

const SORTABLE_COLUMNS: Record<string, string> = {
  createdAt: "created_at",
  messageCount: "message_count",
  toolCount: "tool_count",
};

export default function SessionsPage() {
  const { t, locale } = useI18n();
  const router = useRouter();
  const isMobile = useIsMobile();
  const { isDemo } = useAuth();
  const { loginEnabled, pending, isInDemo, toggle } = useDemoWhitelist();
  const [timeRange, setTimeRange] = usePersistentState<TimeRangeKey>(
    "dashboard.sessions.timeRange",
    "30d",
  );
  const [customStart, setCustomStart] = usePersistentState("dashboard.sessions.customStart", "");
  const [customEnd, setCustomEnd] = usePersistentState("dashboard.sessions.customEnd", "");
  const [sort, setSort] = useState<{ field: string; dir: SortDir }>({
    field: "created_at",
    dir: "desc",
  });
  const [scoring, setScoring] = useState<number | null>(null);
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [batchDeleting, setBatchDeleting] = useState(false);
  const [batchDeleteConfirmOpen, setBatchDeleteConfirmOpen] = useState(false);

  const fetchOptionsFor = useCallback(
    (field: "score" | "model" | "messageCount" | "harness") => async () => {
      const { startTime, endTime } = computeRange(timeRange, customStart, customEnd);
      const rsp = await api.listSessionOptions({ field, startTime, endTime });
      return rsp.items ?? [];
    },
    [timeRange, customStart, customEnd],
  );

  const facets = useMemo<FacetDef[]>(
    () => [
      {
        key: "score",
        label: t("sessions.filter_score"),
        options: fetchOptionsFor("score"),
        formatValue: (v) => (v === "unscored" ? t("sessions.filter_unscored") : `★${v}`),
      },
      { key: "model", label: t("sessions.filter_model"), options: fetchOptionsFor("model") },
      {
        key: "harness",
        label: t("sessions.filter_harness"),
        options: fetchOptionsFor("harness"),
        single: true,
      },
      {
        key: "messageCount",
        label: t("sessions.filter_message_count"),
        options: fetchOptionsFor("messageCount"),
      },
    ],
    // locale 必须在依赖里：t 引用已稳定（见 lib/i18n.tsx），翻译文本刷新只能靠 locale 驱动重算
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [locale, fetchOptionsFor],
  );

  const filterBar = useFilterBar({
    persistKey: "dashboard.sessions",
    facets,
    freeTextPlaceholder: t("sessions.search_placeholder"),
    legacyKeys: {
      score: "dashboard.sessions.filterScore",
      model: "dashboard.sessions.filterModel",
      messageCount: "dashboard.sessions.filterMessageCount",
      _draft: "dashboard.sessions.searchInput",
    },
    legacyFreeTextKey: "dashboard.sessions.keyword",
    optionsCacheKey: `${timeRange}:${customStart}:${customEnd}`,
  });
  const { queryParams } = filterBar;

  interface SessionsQuery {
    page: number;
    pageSize: number;
    range: TimeRangeKey;
    cs: string;
    ce: string;
    sortState: { field: string; dir: SortDir };
    qp: FilterBarQueryParams;
    silent?: boolean;
  }

  // 加载失败静默（不传 errorTitle）：列表保留上一次结果
  const {
    items: sessions,
    setItems: setSessions,
    pageInfo,
    loading,
    load,
  } = usePaginatedList({
    persistKey: "dashboard.sessions",
    fetchPage: async (page, pageSize, q: Omit<SessionsQuery, "page" | "pageSize" | "silent">) => {
      const { startTime, endTime } = computeRange(q.range, q.cs, q.ce);
      const rsp = await api.listSessions({
        page,
        pageSize,
        sort: q.sortState.dir,
        sortField: q.sortState.field,
        startTime,
        endTime,
        keyword: q.qp.freeText || undefined,
        filter: q.qp.filter,
      });
      return { items: rsp.sessions, pageInfo: rsp.pageInfo };
    },
  });
  const fetchSessions = ({ page, pageSize, silent, ...q }: SessionsQuery) =>
    load(page, pageSize, q, { silent });

  const currentQuery = (): Omit<SessionsQuery, "page" | "pageSize"> => ({
    range: timeRange,
    cs: customStart,
    ce: customEnd,
    sortState: sort,
    qp: queryParams,
  });

  /* eslint-disable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps -- token 变化回到第 1 页查询；挂载时以持久化筛选发起首次查询 */
  useEffect(() => {
    setSelected(new Set());
    fetchSessions({ page: 1, pageSize: pageInfo.pageSize, ...currentQuery() });
  }, [queryParams]);
  /* eslint-enable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps */

  const refresh = (page: number, pageSize?: number) =>
    fetchSessions({ page, pageSize: pageSize ?? pageInfo.pageSize, ...currentQuery() });

  const handleSort = (field: string) => {
    const newSort: { field: string; dir: SortDir } =
      sort.field === field
        ? { field, dir: sort.dir === "asc" ? "desc" : "asc" }
        : { field, dir: "desc" };
    setSort(newSort);
    fetchSessions({ page: 1, pageSize: pageInfo.pageSize, ...currentQuery(), sortState: newSort });
  };

  const renderSortIcon = (field: string) => {
    if (sort.field !== field) return null;
    return sort.dir === "asc" ? <ArrowUp className="size-3" /> : <ArrowDown className="size-3" />;
  };

  const deleteConfirm = useDeleteConfirm<SessionSummary>({
    onConfirm: async (s) => {
      await api.deleteSession(s.id);
      toast.success(t("sessions.delete_success"));
      fetchSessions({
        page: pageInfo.page,
        pageSize: pageInfo.pageSize,
        ...currentQuery(),
        silent: true,
      });
    },
    onError: (err) => showErrorToast(err, { title: t("sessions.delete_error") }),
  });

  const toggleSelect = (id: number) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  };

  const toggleSelectAll = () => {
    if (selected.size === sessions.length) {
      setSelected(new Set());
    } else {
      setSelected(new Set(sessions.map((s) => s.id)));
    }
  };

  const handleBatchDelete = async () => {
    if (selected.size === 0) return;
    setBatchDeleting(true);
    try {
      const ids = Array.from(selected);
      const rsp = await api.batchDeleteSessions(ids);
      const failed = rsp.failures?.length ?? 0;
      if (failed > 0) {
        toast.warning(
          t("sessions.batch_delete_warning")
            .replace("{deleted}", String(rsp.deletedCount))
            .replace("{failed}", String(failed)),
        );
      } else {
        toast.success(
          t("sessions.batch_delete_success").replace("{count}", String(rsp.deletedCount)),
        );
      }
      setSelected(new Set());
      fetchSessions({ page: 1, pageSize: pageInfo.pageSize, ...currentQuery(), silent: true });
    } catch (err) {
      showErrorToast(err, { title: t("sessions.batch_delete_error") });
    } finally {
      setBatchDeleting(false);
      setBatchDeleteConfirmOpen(false);
    }
  };

  const handleScoreSession = async (sessionId: number, score: number) => {
    if (scoring !== null) return;
    setScoring(sessionId);
    try {
      await api.scoreSession({ sessionId, score });
      setSessions((prev) => prev.map((s) => (s.id === sessionId ? { ...s, score } : s)));
      toast.success(t("sessions.scored"));
    } catch (err) {
      showErrorToast(err, { title: t("sessions.score_error") });
    } finally {
      setScoring(null);
    }
  };

  const handleDeleteScore = async (sessionId: number) => {
    if (scoring !== null) return;
    setScoring(sessionId);
    try {
      await api.deleteScoreSession(sessionId);
      setSessions((prev) => prev.map((s) => (s.id === sessionId ? { ...s, score: undefined } : s)));
      toast.success(t("sessions.score_removed"));
    } catch (err) {
      showErrorToast(err, { title: t("sessions.score_remove_error") });
    } finally {
      setScoring(null);
    }
  };

  return (
    <PermissionGuard module="sessions">
      <div className="space-y-8">
        <PageHeader title={t("sessions.title")} description={t("sessions.subtitle")} />

        <Card>
          <CardHeader>
            <CardTitle className="font-display">{t("sessions.all_sessions")}</CardTitle>
          </CardHeader>
          <CardContent className="@container">
            {/* Filters — faceted bar */}
            <div className="mb-4 flex flex-col gap-3 md:flex-row md:items-center">
              <TimeRangePicker
                value={timeRange}
                customStart={customStart}
                customEnd={customEnd}
                onChange={(key, cs, ce) => {
                  setTimeRange(key);
                  setCustomStart(cs);
                  setCustomEnd(ce);
                  fetchSessions({
                    page: 1,
                    pageSize: pageInfo.pageSize,
                    range: key,
                    cs,
                    ce,
                    sortState: sort,
                    qp: queryParams,
                  });
                }}
              />
              <FilterBar
                {...filterBar}
                facets={facets}
                placeholder={t("sessions.search_placeholder")}
              />
              {selected.size > 0 && (
                <Button
                  variant="destructive"
                  size="sm"
                  disabled={isDemo()}
                  onClick={() => setBatchDeleteConfirmOpen(true)}
                  className="gap-1.5 md:ml-auto"
                >
                  {isDemo() ? <Lock className="size-3.5" /> : <Trash2 className="size-3.5" />}
                  {t("common.delete")} {selected.size}
                </Button>
              )}
            </div>
            {filterBar.tokens.length > 0 && (
              <p className="-mt-2 mb-3 text-xs text-muted-foreground">
                {t("filter_bar.applied_count").replace("{count}", String(filterBar.tokens.length))}
              </p>
            )}

            {loading ? (
              <TableSkeleton rows={5} rowClassName="h-10" />
            ) : sessions.length === 0 ? (
              filterBar.tokens.length > 0 ? (
                <ListEmptyState
                  icon={MessageSquare}
                  message={t("sessions.no_match")}
                  action={
                    <Button variant="outline" size="sm" onClick={filterBar.clearTokens}>
                      {t("filter_bar.clear_filters")}
                    </Button>
                  }
                />
              ) : (
                <ListEmptyState
                  icon={MessageSquare}
                  message={t("sessions.no_sessions")}
                  hint={t("sessions.empty_hint")}
                  action={
                    <Button
                      variant="outline"
                      size="sm"
                      nativeButton={false}
                      render={<Link href="/apikeys/" />}
                    >
                      {t("sessions.empty_action")}
                    </Button>
                  }
                />
              )
            ) : (
              <>
                {isMobile ? (
                  <ItemGroup className="gap-3">
                    {sessions.map((s) => {
                      const isSelected = selected.has(s.id);
                      return (
                        <Item
                          key={s.id}
                          variant="outline"
                          role="listitem"
                          className="cursor-pointer hover:bg-muted"
                          onClick={() => {
                            router.push(`/sessions/detail/?id=${s.id}`);
                          }}
                        >
                          <ItemMedia>
                            <Checkbox
                              checked={isSelected}
                              onCheckedChange={() => toggleSelect(s.id)}
                              onClick={(e) => e.stopPropagation()}
                              aria-label={t("sessions.select_aria").replace("{id}", String(s.id))}
                            />
                          </ItemMedia>
                          <ItemContent className="min-w-0">
                            <TruncatedText render={<ItemTitle />} className="w-full">
                              {s.summary ||
                                t("sessions.untitled_session").replace("{id}", String(s.id))}
                            </TruncatedText>
                          </ItemContent>
                          <ItemActions>
                            {!isDemo() && (
                              <ScoreDots
                                score={s.score}
                                scoring={scoring === s.id}
                                onScore={(v) => handleScoreSession(s.id, v)}
                                onClear={() => handleDeleteScore(s.id)}
                                size={12}
                              />
                            )}
                            <Badge variant="secondary" className="text-xs">
                              {t("sessions.msg_count").replace(
                                "{count}",
                                String(s.messageCount ?? 0),
                              )}
                            </Badge>
                            <DemoAddButton
                              sessionId={s.id}
                              inDemo={isInDemo(s.id)}
                              pending={pending}
                              loginEnabled={loginEnabled}
                              onToggle={toggle}
                            />
                            <DeleteIconButton
                              locked={isDemo()}
                              disabled={deleteConfirm.loading && deleteConfirm.target?.id === s.id}
                              onClick={(e) => {
                                e.stopPropagation();
                                deleteConfirm.openDelete(s);
                              }}
                              aria-label={t("sessions.delete_aria")}
                            />
                          </ItemActions>
                          <ItemFooter className="flex-wrap justify-start gap-3 text-xs text-muted-foreground">
                            <span>
                              {t("common.id")}: {s.id}
                            </span>
                            <Badge variant="outline" className="text-xs">
                              {s.agentHarness || "Other"}
                            </Badge>
                            <span>
                              {t("sessions.tool_count").replace(
                                "{count}",
                                String(s.toolCount ?? 0),
                              )}
                            </span>
                            {s.modelIds && s.modelIds.length > 0 && (
                              <div className="flex items-center gap-1">
                                {s.modelIds.map((m) => (
                                  <ProviderIcon key={m} protocol={m} size={12} />
                                ))}
                              </div>
                            )}
                            <span>{formatDateTime(s.createdAt)}</span>
                          </ItemFooter>
                        </Item>
                      );
                    })}
                  </ItemGroup>
                ) : (
                  <Table fit>
                    <TableHeader>
                      <TableRow>
                        <TableHead className="w-10">
                          <Checkbox
                            checked={sessions.length > 0 && selected.size === sessions.length}
                            onCheckedChange={toggleSelectAll}
                            aria-label={t("sessions.select_all_aria")}
                          />
                        </TableHead>
                        <TableHead className="w-[4rem]">{t("common.id")}</TableHead>
                        <TableHead
                          className="w-[10.5rem] cursor-pointer select-none whitespace-nowrap"
                          onClick={() => handleSort(SORTABLE_COLUMNS.createdAt)}
                        >
                          <span className="inline-flex items-center gap-1">
                            {t("sessions.time")} {renderSortIcon(SORTABLE_COLUMNS.createdAt)}
                          </span>
                        </TableHead>
                        {/* 摘要不声明宽度：fixed 布局把剩余空间全给这一列 */}
                        <TableHead>{t("sessions.summary")}</TableHead>
                        <TableHead className={cn("w-[120px] text-center", SHOW_SCORE_FROM)}>
                          {t("sessions.score")}
                        </TableHead>
                        <TableHead
                          className="w-[3.5rem] cursor-pointer select-none whitespace-nowrap"
                          onClick={() => handleSort(SORTABLE_COLUMNS.messageCount)}
                        >
                          <span className="inline-flex items-center gap-1">
                            {t("sessions.messages")} {renderSortIcon(SORTABLE_COLUMNS.messageCount)}
                          </span>
                        </TableHead>
                        <TableHead
                          className={cn(
                            "w-[3.5rem] cursor-pointer select-none whitespace-nowrap",
                            SHOW_TOOL_FROM,
                          )}
                          onClick={() => handleSort(SORTABLE_COLUMNS.toolCount)}
                        >
                          <span className="inline-flex items-center gap-1">
                            {t("sessions.tools")} {renderSortIcon(SORTABLE_COLUMNS.toolCount)}
                          </span>
                        </TableHead>
                        <TableHead className={cn("w-[120px]", SHOW_AGENT_FROM)}>
                          {t("sessions.harness")}
                        </TableHead>
                        <TableHead className="w-[140px]">{t("sessions.models")}</TableHead>
                        <TableHead className="w-[104px]">
                          {/* sr-only 只能包文字：直接写在 th 上会把它变成绝对定位，
                              该列宽度因此不计入 fixed 布局的声明总和，剩下一截空白既不归
                              主列也不归任何列（实测 902px 容器丢 53px） */}
                          <span className="sr-only">{t("common.actions")}</span>
                        </TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {sessions.map((s) => {
                        const isSelected = selected.has(s.id);
                        return (
                          <TableRow
                            key={s.id}
                            className="cursor-pointer"
                            onClick={() => {
                              router.push(`/sessions/detail/?id=${s.id}`);
                            }}
                          >
                            <TableCell className="w-10">
                              <Checkbox
                                checked={isSelected}
                                onCheckedChange={() => toggleSelect(s.id)}
                                onClick={(e) => e.stopPropagation()}
                                aria-label={t("sessions.select_aria").replace("{id}", String(s.id))}
                              />
                            </TableCell>
                            <TableCell className="font-mono text-xs">{s.id}</TableCell>
                            <TableCell className="text-muted-foreground">
                              {formatDateTime(s.createdAt)}
                            </TableCell>
                            <TableCell>
                              <TruncatedText className="block text-left">
                                {s.summary || "—"}
                              </TruncatedText>
                            </TableCell>
                            <TableCell className={cn("w-[120px]", SHOW_SCORE_FROM)}>
                              <div className="flex justify-center">
                                <ScoreDots
                                  score={s.score}
                                  scoring={scoring === s.id}
                                  onScore={(v) => handleScoreSession(s.id, v)}
                                  onClear={() => handleDeleteScore(s.id)}
                                  size={12}
                                />
                              </div>
                            </TableCell>
                            <TableCell className="tabular-nums">{s.messageCount ?? 0}</TableCell>
                            <TableCell className={cn("tabular-nums", SHOW_TOOL_FROM)}>
                              {s.toolCount ?? 0}
                            </TableCell>
                            <TableCell className={SHOW_AGENT_FROM}>
                              <Badge
                                variant={
                                  s.agentHarness && s.agentHarness !== "Other"
                                    ? "secondary"
                                    : "outline"
                                }
                                className="text-xs"
                              >
                                {s.agentHarness || "Other"}
                              </Badge>
                            </TableCell>
                            <TableCell>
                              <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
                                {s.modelIds && s.modelIds.length > 0 ? (
                                  s.modelIds.map((m) => (
                                    // 模型名很长：max-w-full + truncate 收在固定列宽内，完整名走 Tooltip
                                    <TooltipRoot key={m}>
                                      <TooltipTrigger
                                        render={
                                          <span className="inline-flex max-w-full min-w-0 items-center gap-1 text-xs text-muted-foreground">
                                            <ProviderIcon
                                              protocol={m}
                                              size={14}
                                              className="shrink-0"
                                            />
                                            <span className="truncate">{m}</span>
                                          </span>
                                        }
                                      />
                                      <TooltipContent
                                        side="top"
                                        align="start"
                                        className="max-w-xs break-all"
                                      >
                                        {m}
                                      </TooltipContent>
                                    </TooltipRoot>
                                  ))
                                ) : (
                                  <span className="text-muted-foreground">—</span>
                                )}
                              </div>
                            </TableCell>
                            <TableCell className="w-[104px]">
                              <div className="flex items-center justify-center gap-1">
                                <DemoAddButton
                                  sessionId={s.id}
                                  inDemo={isInDemo(s.id)}
                                  pending={pending}
                                  loginEnabled={loginEnabled}
                                  onToggle={toggle}
                                />
                                <DeleteIconButton
                                  locked={isDemo()}
                                  disabled={
                                    deleteConfirm.loading && deleteConfirm.target?.id === s.id
                                  }
                                  onClick={(e) => {
                                    e.stopPropagation();
                                    deleteConfirm.openDelete(s);
                                  }}
                                  aria-label={t("sessions.delete_aria")}
                                />
                              </div>
                            </TableCell>
                          </TableRow>
                        );
                      })}
                    </TableBody>
                  </Table>
                )}

                <PaginationBar
                  pageInfo={pageInfo}
                  onChange={(page, pageSize) => refresh(page, pageSize)}
                  totalLabel={t("pagination.sessions")}
                />
              </>
            )}
          </CardContent>
        </Card>

        <DeleteConfirmDialog
          {...deleteConfirm.dialogProps}
          title={t("sessions.delete_dialog_title")}
          description={t("sessions.delete_dialog_desc").replace(
            "{name}",
            deleteConfirm.target
              ? truncateText(
                  (
                    deleteConfirm.target.summary ||
                    t("sessions.untitled_session").replace("{id}", String(deleteConfirm.target.id))
                  )
                    .replace(/\s+/g, " ")
                    .trim(),
                  60,
                )
              : "",
          )}
          confirmLabel={t("common.delete")}
          loadingLabel={t("common.deleting")}
        />

        <DeleteConfirmDialog
          open={batchDeleteConfirmOpen}
          onOpenChange={setBatchDeleteConfirmOpen}
          title={t("sessions.batch_delete_title")}
          description={t("sessions.batch_delete_desc").replace("{count}", String(selected.size))}
          confirmLabel={`${t("common.delete")} ${selected.size}`}
          loadingLabel={t("common.deleting")}
          loading={batchDeleting}
          onConfirm={handleBatchDelete}
        />
      </div>
    </PermissionGuard>
  );
}
