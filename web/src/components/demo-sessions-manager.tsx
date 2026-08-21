"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Check, MessageSquare, Plus, Trash2 } from "lucide-react";
import { api } from "@/lib/api-client";
import { showErrorToast } from "@/lib/api-error-handler";
import { useI18n, useT } from "@/lib/i18n";
import type { DemoSession, PageInfo, SessionSummary } from "@/lib/types";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { ListEmptyState } from "@/components/list-empty-state";
import { TableSkeleton } from "@/components/table-skeleton";
import { PaginationBar } from "@/components/pagination-bar";
import { DeleteConfirmDialog } from "@/components/delete-confirm-dialog";
import { ProviderIcon } from "@/components/provider-icon";
import { usePersistentState } from "@/hooks/use-persistent-state";
import { useIsMobile } from "@/hooks/use-mobile";
import { toast } from "sonner";
import { cn, formatDateTime } from "@/lib/utils";
import { FilterBar } from "@/components/filter-bar/filter-bar";
import { useFilterBar } from "@/components/filter-bar/use-filter-bar";
import type { FacetDef, FilterBarQueryParams } from "@/components/filter-bar/types";
import { TimeRangePicker } from "@/components/ui/time-range-picker";
import type { TimeRangeKey } from "@/lib/time-range";
import { computeRange } from "@/lib/time-range";

/** 两个列表共用的行结构（DemoSession 与 SessionSummary 均可赋值） */
interface SessionRow {
  id: number;
  summary?: string;
  score?: number;
  messageCount: number;
  toolCount: number;
  createdAt?: string;
  modelIds?: string[];
}

/** 勾选框（对齐 sessions/trigger 的 role="checkbox" 自绘模式） */
function SelectCheckbox({ checked, onToggle }: { checked: boolean; onToggle: () => void }) {
  return (
    <div
      role="checkbox"
      aria-checked={checked}
      tabIndex={0}
      onClick={onToggle}
      onKeyDown={(e) => {
        if (e.key === " " || e.key === "Enter") onToggle();
      }}
      className={cn(
        "flex size-4 cursor-pointer items-center justify-center rounded border transition-colors",
        checked
          ? "border-primary bg-primary text-primary-foreground"
          : "border-muted-foreground/30 hover:border-muted-foreground",
      )}
    >
      {checked && <Check className="size-3" />}
    </div>
  );
}

function SessionListTable({
  items,
  selectedIds,
  onToggle,
  onToggleAll,
  emptyMessage,
  isMobile,
}: {
  items: SessionRow[];
  selectedIds: Set<number>;
  onToggle: (id: number) => void;
  onToggleAll: () => void;
  emptyMessage: string;
  isMobile: boolean;
}) {
  const t = useT();

  if (items.length === 0) {
    return (
      <ListEmptyState
        icon={<MessageSquare className="mb-3 size-10 text-muted-foreground/40" />}
        message={emptyMessage}
      />
    );
  }

  const summary = (item: SessionRow) =>
    item.summary || t("sessions.untitled_session").replace("{id}", String(item.id));

  if (isMobile) {
    return (
      <div className="space-y-3">
        {items.map((item) => {
          const isSelected = selectedIds.has(item.id);
          return (
            <div key={item.id} className="rounded-lg border border-border bg-card p-4">
              <div className="flex items-start gap-2">
                <SelectCheckbox checked={isSelected} onToggle={() => onToggle(item.id)} />
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-medium">{summary(item)}</p>
                  <div className="mt-2 flex items-center gap-3 text-xs text-muted-foreground">
                    <span>
                      {t("common.id")}: {item.id}
                    </span>
                    <span>
                      {t("sessions.messages")}: {item.messageCount}
                    </span>
                    <span>
                      {t("sessions.tools")}: {item.toolCount}
                    </span>
                    {item.score != null && (
                      <span>
                        {t("sessions.score")}: ★{item.score}
                      </span>
                    )}
                  </div>
                  <div className="mt-1.5 flex flex-wrap items-center gap-3 text-xs text-muted-foreground">
                    {item.modelIds && item.modelIds.length > 0 && (
                      <div className="flex items-center gap-1">
                        {item.modelIds.map((m) => (
                          <ProviderIcon key={m} protocol={m} size={12} />
                        ))}
                      </div>
                    )}
                    {item.createdAt && <span>{formatDateTime(item.createdAt)}</span>}
                  </div>
                </div>
              </div>
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
          <TableHead className="w-10">
            <SelectCheckbox checked={selectedIds.size === items.length} onToggle={onToggleAll} />
          </TableHead>
          <TableHead>{t("common.id")}</TableHead>
          <TableHead className="w-[160px] whitespace-nowrap">{t("sessions.time")}</TableHead>
          <TableHead>{t("sessions.summary")}</TableHead>
          <TableHead className="w-[160px] text-center">{t("sessions.score")}</TableHead>
          <TableHead className="w-24">{t("sessions.messages")}</TableHead>
          <TableHead className="w-24">{t("sessions.tools")}</TableHead>
          <TableHead className="w-[140px]">{t("sessions.models")}</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {items.map((item) => {
          const isSelected = selectedIds.has(item.id);
          return (
            <TableRow key={item.id}>
              <TableCell className="w-10">
                <SelectCheckbox checked={isSelected} onToggle={() => onToggle(item.id)} />
              </TableCell>
              <TableCell className="font-mono text-xs">{item.id}</TableCell>
              <TableCell className="whitespace-nowrap text-muted-foreground">
                {item.createdAt ? formatDateTime(item.createdAt) : "—"}
              </TableCell>
              <TableCell className="max-w-[200px] truncate">
                {summary(item)}
              </TableCell>
              <TableCell className="w-[160px]">
                <div className="flex justify-center">
                  {item.score != null ? (
                    <span className="text-sm">
                      {"★".repeat(item.score)}
                      <span className="text-muted-foreground/30">{"★".repeat(5 - item.score)}</span>
                    </span>
                  ) : (
                    <span className="text-muted-foreground">—</span>
                  )}
                </div>
              </TableCell>
              <TableCell>{item.messageCount}</TableCell>
              <TableCell>{item.toolCount}</TableCell>
              <TableCell>
                <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
                  {item.modelIds && item.modelIds.length > 0 ? (
                    item.modelIds.map((m) => (
                      <span
                        key={m}
                        className="inline-flex items-center gap-1 text-xs text-muted-foreground"
                      >
                        <ProviderIcon protocol={m} size={14} />
                        {m}
                      </span>
                    ))
                  ) : (
                    <span className="text-muted-foreground">—</span>
                  )}
                </div>
              </TableCell>
            </TableRow>
          );
        })}
      </TableBody>
    </Table>
  );
}

/** Demo sessions 管理：上半已选列表 + 下半会话选择器 */
export function DemoSessionsManager() {
  const t = useT();
  const isMobile = useIsMobile();

  // ─── 已选列表（demo 账户可见的 sessions） ─────────────────────────────────
  const [selectedSessions, setSelectedSessions] = useState<DemoSession[]>([]);
  const [selectedPage, setSelectedPage] = usePersistentState(
    "dashboard.demo.selected.page",
    1,
  );
  const [selectedPageSize, setSelectedPageSize] = usePersistentState(
    "dashboard.demo.selected.pageSize",
    100,
  );
  const [selectedPageInfo, setSelectedPageInfo] = useState<PageInfo>({
    page: selectedPage,
    pageSize: selectedPageSize,
    total: 0,
  });
  const [selectedLoading, setSelectedLoading] = useState(true);
  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set());
  const [removing, setRemoving] = useState(false);
  const [removeConfirmOpen, setRemoveConfirmOpen] = useState(false);

  const fetchSelected = useCallback(
    async (page: number, pageSize: number) => {
      setSelectedLoading(true);
      try {
        const rsp = await api.listDemoSessions(page, pageSize);
        setSelectedSessions(rsp.sessions ?? []);
        if (rsp.pageInfo) {
          setSelectedPageInfo(rsp.pageInfo);
          setSelectedPage(rsp.pageInfo.page);
          setSelectedPageSize(rsp.pageInfo.pageSize);
        }
        setSelectedIds(new Set());
      } catch (err) {
        showErrorToast(err, { title: t("demo.sessions_load_error") });
      } finally {
        setSelectedLoading(false);
      }
    },
    [setSelectedPage, setSelectedPageSize, t],
  );

  /* eslint-disable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps -- Initial fetch on mount with persisted pagination */
  useEffect(() => {
    fetchSelected(selectedPage, selectedPageSize);
  }, [fetchSelected]);
  /* eslint-enable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps */

  // ─── 选择器（候选 sessions） ─────────────────────────────────────────────
  const { locale } = useI18n();
  const [candidates, setCandidates] = useState<SessionSummary[]>([]);
  const [candidatePage, setCandidatePage] = usePersistentState(
    "dashboard.demo.candidates.page",
    1,
  );
  const [candidatePageSize, setCandidatePageSize] = usePersistentState(
    "dashboard.demo.candidates.pageSize",
    20,
  );
  const [candidatePageInfo, setCandidatePageInfo] = useState<PageInfo>({
    page: candidatePage,
    pageSize: candidatePageSize,
    total: 0,
  });
  const [candidateLoading, setCandidateLoading] = useState(true);
  const [candidateTimeRange, setCandidateTimeRange] = usePersistentState<TimeRangeKey>(
    "dashboard.demo.candidates.timeRange",
    "30d",
  );
  const [candidateCustomStart, setCandidateCustomStart] = usePersistentState(
    "dashboard.demo.candidates.customStart",
    "",
  );
  const [candidateCustomEnd, setCandidateCustomEnd] = usePersistentState(
    "dashboard.demo.candidates.customEnd",
    "",
  );
  const [candidateIds, setCandidateIds] = useState<Set<number>>(new Set());
  const [adding, setAdding] = useState(false);

  interface CandidatesQuery {
    page: number;
    pageSize: number;
    qp: FilterBarQueryParams;
    range?: { key: TimeRangeKey; cs: string; ce: string };
  }

  const fetchCandidates = useCallback(
    async (q: CandidatesQuery) => {
      setCandidateLoading(true);
      try {
        const { startTime, endTime } = computeRange(
          q.range?.key ?? candidateTimeRange,
          q.range?.cs ?? candidateCustomStart,
          q.range?.ce ?? candidateCustomEnd,
        );
        const rsp = await api.listSessions({
          page: q.page,
          pageSize: q.pageSize,
          sort: "desc",
          sortField: "created_at",
          startTime,
          endTime,
          keyword: q.qp.freeText || undefined,
          filter: q.qp.filter,
        });
        setCandidates(rsp.sessions ?? []);
        if (rsp.pageInfo) {
          setCandidatePageInfo(rsp.pageInfo);
          setCandidatePage(rsp.pageInfo.page);
          setCandidatePageSize(rsp.pageInfo.pageSize);
        }
        setCandidateIds(new Set());
      } catch (err) {
        showErrorToast(err, { title: t("demo.candidates_load_error") });
      } finally {
        setCandidateLoading(false);
      }
    },
    [
      candidateTimeRange,
      candidateCustomStart,
      candidateCustomEnd,
      setCandidatePage,
      setCandidatePageSize,
      t,
    ],
  );

  const fetchOptionsFor = useCallback(
    (field: "score" | "model" | "messageCount") => async () => {
      const { startTime, endTime } = computeRange(
        candidateTimeRange,
        candidateCustomStart,
        candidateCustomEnd,
      );
      const rsp = await api.listSessionOptions({ field, startTime, endTime });
      return rsp.items ?? [];
    },
    [candidateTimeRange, candidateCustomStart, candidateCustomEnd],
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
    persistKey: "dashboard.demo.candidates",
    facets,
    freeTextPlaceholder: t("demo.search_placeholder"),
    legacyKeys: {
      keyword: "dashboard.demo.candidates.keyword",
    },
    legacyFreeTextKey: "dashboard.demo.candidates.searchInput",
    optionsCacheKey: `${candidateTimeRange}:${candidateCustomStart}:${candidateCustomEnd}`,
  });
  const { queryParams } = filterBar;

  /* eslint-disable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps -- Initial fetch on mount with persisted pagination and filters */
  useEffect(() => {
    fetchCandidates({ page: candidatePage, pageSize: candidatePageSize, qp: queryParams });
  }, [queryParams]);
  /* eslint-enable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps */

  const toggleSelectedId = useCallback((id: number) => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }, []);

  const toggleSelectedAll = useCallback(() => {
    if (selectedIds.size === selectedSessions.length) {
      setSelectedIds(new Set());
    } else {
      setSelectedIds(new Set(selectedSessions.map((s) => s.id)));
    }
  }, [selectedIds.size, selectedSessions]);

  const toggleCandidateId = useCallback((id: number) => {
    setCandidateIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }, []);

  const toggleCandidateAll = useCallback(() => {
    if (candidateIds.size === candidates.length) {
      setCandidateIds(new Set());
    } else {
      setCandidateIds(new Set(candidates.map((s) => s.id)));
    }
  }, [candidateIds.size, candidates]);

  const handleAdd = useCallback(async () => {
    if (candidateIds.size === 0 || adding) return;
    setAdding(true);
    try {
      const ids = Array.from(candidateIds);
      await api.addDemoSessions({ sessionIds: ids });
      toast.success(t("demo.add_success").replace("{count}", String(ids.length)));
      setCandidateIds(new Set());
      await fetchSelected(selectedPage, selectedPageSize);
    } catch (err) {
      showErrorToast(err, { title: t("demo.add_error") });
    } finally {
      setAdding(false);
    }
  }, [adding, candidateIds, fetchSelected, selectedPage, selectedPageSize, t]);

  const handleRemove = useCallback(async () => {
    if (selectedIds.size === 0) return;
    setRemoving(true);
    try {
      const ids = Array.from(selectedIds);
      await api.removeDemoSessions(ids);
      toast.success(t("demo.remove_success").replace("{count}", String(ids.length)));
      setSelectedIds(new Set());
      await fetchSelected(selectedPage, selectedPageSize);
    } catch (err) {
      showErrorToast(err, { title: t("demo.remove_error") });
    } finally {
      setRemoving(false);
      setRemoveConfirmOpen(false);
    }
  }, [fetchSelected, selectedIds, selectedPage, selectedPageSize, t]);

  return (
    <div className="space-y-8">
      <Card>
        <CardHeader>
          <CardTitle className="font-display">{t("demo.selected_title")}</CardTitle>
          <CardDescription>{t("demo.selected_desc")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center justify-between gap-3">
            {selectedIds.size > 0 ? (
              <Button
                variant="destructive"
                size="sm"
                onClick={() => setRemoveConfirmOpen(true)}
                className="gap-1.5"
              >
                <Trash2 className="size-3.5" />
                {t("demo.remove_selected")} {selectedIds.size}
              </Button>
            ) : (
              <span />
            )}
          </div>

          {selectedLoading ? (
            <TableSkeleton rows={3} />
          ) : (
            <SessionListTable
              items={selectedSessions}
              selectedIds={selectedIds}
              onToggle={toggleSelectedId}
              onToggleAll={toggleSelectedAll}
              emptyMessage={t("demo.no_selected")}
              isMobile={isMobile}
            />
          )}

          <PaginationBar
            pageInfo={selectedPageInfo}
            onChange={(page, pageSize) => fetchSelected(page, pageSize)}
            totalLabel={t("pagination.sessions")}
          />
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="font-display">{t("demo.selector_title")}</CardTitle>
          <CardDescription>{t("demo.selector_desc")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex flex-col gap-3 md:flex-row md:items-center">
            <TimeRangePicker
              value={candidateTimeRange}
              customStart={candidateCustomStart}
              customEnd={candidateCustomEnd}
              onChange={(key, cs, ce) => {
                setCandidateTimeRange(key);
                setCandidateCustomStart(cs);
                setCandidateCustomEnd(ce);
                fetchCandidates({
                  page: 1,
                  pageSize: candidatePageSize,
                  qp: queryParams,
                  range: { key, cs, ce },
                });
              }}
            />
            <FilterBar
              {...filterBar}
              facets={facets}
              placeholder={t("demo.search_placeholder")}
            />
            {candidateIds.size > 0 && (
              <Button
                size="sm"
                disabled={adding}
                onClick={handleAdd}
                className="gap-1.5 md:ml-auto"
              >
                <Plus className="size-3.5" />
                {t("demo.add_selected")} {candidateIds.size}
              </Button>
            )}
          </div>
          {filterBar.tokens.length > 0 && (
            <p className="-mt-2 mb-3 text-xs text-muted-foreground">
              {t("filter_bar.applied_count").replace("{count}", String(filterBar.tokens.length))}
            </p>
          )}

          {candidateLoading ? (
            <TableSkeleton rows={3} />
          ) : (
            <SessionListTable
              items={candidates}
              selectedIds={candidateIds}
              onToggle={toggleCandidateId}
              onToggleAll={toggleCandidateAll}
              emptyMessage={t("demo.no_candidates")}
              isMobile={isMobile}
            />
          )}

          <PaginationBar
            pageInfo={candidatePageInfo}
            onChange={(page, pageSize) =>
              fetchCandidates({ page, pageSize, qp: queryParams })
            }
            totalLabel={t("pagination.sessions")}
          />
        </CardContent>
      </Card>

      <DeleteConfirmDialog
        open={removeConfirmOpen}
        onOpenChange={setRemoveConfirmOpen}
        title={t("demo.remove_confirm_title")}
        description={t("demo.remove_confirm_desc").replace("{count}", String(selectedIds.size))}
        confirmLabel={`${t("demo.remove_selected")} ${selectedIds.size}`}
        loadingLabel={t("common.processing")}
        loading={removing}
        onConfirm={handleRemove}
      />
    </div>
  );
}
