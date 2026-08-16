"use client";

import { useCallback, useEffect, useState } from "react";
import { usePersistentState } from "@/hooks/use-persistent-state";
import { api } from "@/lib/api-client";
import { showErrorToast } from "@/lib/api-error-handler";
import type { SessionSummary, PageInfo } from "@/lib/types";
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
import { Badge } from "@/components/ui/badge";
import { MessageSquare, Check, ArrowUp, ArrowDown, Trash2, Lock, X } from "lucide-react";
import { useT } from "@/lib/i18n";
import { useAuth } from "@/lib/auth-context";
import { PermissionGuard } from "@/components/permission-guard";
import { useIsMobile } from "@/hooks/use-mobile";
import { formatDateTime, truncateText } from "@/lib/utils";
import { ScoreDots } from "@/components/session-detail/score-dots";
import { PageHeader } from "@/components/page-header";
import { SearchInput } from "@/components/search-input";
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
import { MultiSelectPill } from "@/components/ui/multi-select-pill";
import { ProviderIcon } from "@/components/provider-icon";

type SortDir = "asc" | "desc";

const SORTABLE_COLUMNS: Record<string, string> = {
  createdAt: "created_at",
  messageCount: "message_count",
  toolCount: "tool_count",
};

export default function SessionsPage() {
  const t = useT();
  const isMobile = useIsMobile();
  const { isDemo } = useAuth();
  const [sessions, setSessions] = useState<SessionSummary[]>([]);
  const [persistedPage, setPersistedPage] = usePersistentState("dashboard.sessions.page", 1);
  const [persistedPageSize, setPersistedPageSize] = usePersistentState(
    "dashboard.sessions.pageSize",
    20,
  );
  const [pageInfo, setPageInfo] = useState<PageInfo>({
    page: persistedPage,
    pageSize: persistedPageSize,
    total: 0,
  });
  const [loading, setLoading] = useState(true);
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
  const [keyword, setKeyword] = usePersistentState("dashboard.sessions.keyword", "");
  const [searchInput, setSearchInput] = usePersistentState("dashboard.sessions.searchInput", "");
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [batchDeleting, setBatchDeleting] = useState(false);
  const [batchDeleteConfirmOpen, setBatchDeleteConfirmOpen] = useState(false);
  const [filterScore, setFilterScore] = usePersistentState<string[]>(
    "dashboard.sessions.filterScore",
    [],
  );
  const [filterModel, setFilterModel] = usePersistentState<string[]>(
    "dashboard.sessions.filterModel",
    [],
  );
  const [filterMessageCount, setFilterMessageCount] = usePersistentState<string[]>(
    "dashboard.sessions.filterMessageCount",
    [],
  );
  const [scoreOptions, setScoreOptions] = useState<string[]>([]);
  const [modelOptions, setModelOptions] = useState<string[]>([]);
  const [messageCountOptions, setMessageCountOptions] = useState<string[]>([]);

  const fetchScoreOptions = useCallback(async (range: TimeRangeKey, cs: string, ce: string) => {
    const { startTime, endTime } = computeRange(range, cs, ce);
    try {
      const rsp = await api.listSessionOptions({ field: "score", startTime, endTime });
      if (!rsp.error && rsp.items) setScoreOptions(rsp.items);
    } catch (err) {
      console.error("Failed to load score options:", err);
    }
  }, []);

  const fetchModelOptions = useCallback(async (range: TimeRangeKey, cs: string, ce: string) => {
    const { startTime, endTime } = computeRange(range, cs, ce);
    try {
      const rsp = await api.listSessionOptions({ field: "model", startTime, endTime });
      if (!rsp.error && rsp.items) setModelOptions(rsp.items);
    } catch (err) {
      console.error("Failed to load model options:", err);
    }
  }, []);

  const fetchMessageCountOptions = useCallback(
    async (range: TimeRangeKey, cs: string, ce: string) => {
      const { startTime, endTime } = computeRange(range, cs, ce);
      try {
        const rsp = await api.listSessionOptions({ field: "messageCount", startTime, endTime });
        if (!rsp.error && rsp.items) setMessageCountOptions(rsp.items);
      } catch (err) {
        console.error("Failed to load message count options:", err);
      }
    },
    [],
  );

  /* eslint-disable react-hooks/set-state-in-effect -- Re-fetch filter options when the time range changes */
  useEffect(() => {
    fetchScoreOptions(timeRange, customStart, customEnd);
    fetchModelOptions(timeRange, customStart, customEnd);
    fetchMessageCountOptions(timeRange, customStart, customEnd);
  }, [
    timeRange,
    customStart,
    customEnd,
    fetchScoreOptions,
    fetchModelOptions,
    fetchMessageCountOptions,
  ]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const buildSessionFilter = (
    scores: string[],
    models: string[],
    msgCounts: string[],
  ): string | undefined => {
    const parts: string[] = [];
    if (scores.length > 0) parts.push(`score:${scores.join("|")}`);
    if (models.length > 0) parts.push(`model:${models.join("|")}`);
    if (msgCounts.length > 0) parts.push(`messageCount:${msgCounts.join("|")}`);
    return parts.length > 0 ? parts.join(" ") : undefined;
  };

  const fetchSessions = useCallback(
    async (
      page: number,
      pageSize: number,
      range: TimeRangeKey,
      cs: string,
      ce: string,
      sortState: { field: string; dir: SortDir },
      kw: string,
      score: string[],
      models: string[],
      msgCounts: string[],
      silent?: boolean,
    ) => {
      if (!silent) setLoading(true);
      try {
        const { startTime, endTime } = computeRange(range, cs, ce);
        const rsp = await api.listSessions({
          page,
          pageSize,
          sort: sortState.dir,
          sortField: sortState.field,
          startTime,
          endTime,
          keyword: kw || undefined,
          filter: buildSessionFilter(score, models, msgCounts),
        });
        setSessions(rsp.sessions ?? []);
        if (rsp.pageInfo) {
          setPageInfo(rsp.pageInfo);
          setPersistedPage(rsp.pageInfo.page);
          setPersistedPageSize(rsp.pageInfo.pageSize);
        }
      } catch {
        // handled silently
      } finally {
        setLoading(false);
      }
    },
    [setPersistedPage, setPersistedPageSize],
  );

  /* eslint-disable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps -- Initial data fetch on mount with persisted filters */
  useEffect(() => {
    fetchSessions(
      persistedPage,
      persistedPageSize,
      timeRange,
      customStart,
      customEnd,
      sort,
      keyword,
      filterScore,
      filterModel,
      filterMessageCount,
    );
  }, [fetchSessions]);
  /* eslint-enable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps */

  const refresh = (page: number, pageSize?: number) =>
    fetchSessions(
      page,
      pageSize ?? pageInfo.pageSize,
      timeRange,
      customStart,
      customEnd,
      sort,
      keyword,
      filterScore,
      filterModel,
      filterMessageCount,
    );

  const handleSort = (field: string) => {
    const newSort: { field: string; dir: SortDir } =
      sort.field === field
        ? { field, dir: sort.dir === "asc" ? "desc" : "asc" }
        : { field, dir: "desc" };
    setSort(newSort);
    fetchSessions(
      1,
      pageInfo.pageSize,
      timeRange,
      customStart,
      customEnd,
      newSort,
      keyword,
      filterScore,
      filterModel,
      filterMessageCount,
    );
  };

  const handleSearch = () => {
    const kw = searchInput.trim();
    setKeyword(kw);
    setSelected(new Set());
    fetchSessions(
      1,
      pageInfo.pageSize,
      timeRange,
      customStart,
      customEnd,
      sort,
      kw,
      filterScore,
      filterModel,
      filterMessageCount,
    );
  };

  const renderSortIcon = (field: string) => {
    if (sort.field !== field) return null;
    return sort.dir === "asc" ? <ArrowUp className="size-3" /> : <ArrowDown className="size-3" />;
  };

  const deleteConfirm = useDeleteConfirm<SessionSummary>({
    onConfirm: async (s) => {
      await api.deleteSession(s.id);
      toast.success(t("sessions.delete_success"));
      fetchSessions(
        pageInfo.page,
        pageInfo.pageSize,
        timeRange,
        customStart,
        customEnd,
        sort,
        keyword,
        filterScore,
        filterModel,
        filterMessageCount,
        true,
      );
    },
    onError: (err) => showErrorToast(err, { title: t("sessions.delete_error") }),
  });

  const toggleSelect = (id: number, e: React.MouseEvent) => {
    e.stopPropagation();
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
      fetchSessions(
        1,
        pageInfo.pageSize,
        timeRange,
        customStart,
        customEnd,
        sort,
        keyword,
        filterScore,
        filterModel,
        filterMessageCount,
        true,
      );
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
        <CardContent>
          {/* Filters — always visible */}
          <div className="mb-4 flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
            <div className="flex flex-wrap items-center gap-2">
              <TimeRangePicker
                value={timeRange}
                customStart={customStart}
                customEnd={customEnd}
                onChange={(key, cs, ce) => {
                  setTimeRange(key);
                  setCustomStart(cs);
                  setCustomEnd(ce);
                  fetchSessions(
                    1,
                    pageInfo.pageSize,
                    key,
                    cs,
                    ce,
                    sort,
                    keyword,
                    filterScore,
                    filterModel,
                    filterMessageCount,
                  );
                }}
              />
              <MultiSelectPill
                label={t("sessions.filter_score")}
                options={scoreOptions}
                value={filterScore}
                onChange={(v) => {
                  setFilterScore(v);
                  fetchSessions(
                    1,
                    pageInfo.pageSize,
                    timeRange,
                    customStart,
                    customEnd,
                    sort,
                    keyword,
                    v,
                    filterModel,
                    filterMessageCount,
                  );
                }}
              />
              <MultiSelectPill
                label={t("sessions.filter_model")}
                options={modelOptions}
                value={filterModel}
                onChange={(v) => {
                  setFilterModel(v);
                  fetchSessions(
                    1,
                    pageInfo.pageSize,
                    timeRange,
                    customStart,
                    customEnd,
                    sort,
                    keyword,
                    filterScore,
                    v,
                    filterMessageCount,
                  );
                }}
              />
              <MultiSelectPill
                label={t("sessions.filter_message_count")}
                options={messageCountOptions}
                value={filterMessageCount}
                onChange={(v) => {
                  setFilterMessageCount(v);
                  fetchSessions(
                    1,
                    pageInfo.pageSize,
                    timeRange,
                    customStart,
                    customEnd,
                    sort,
                    keyword,
                    filterScore,
                    filterModel,
                    v,
                  );
                }}
              />
              {(filterScore.length > 0 ||
                filterModel.length > 0 ||
                filterMessageCount.length > 0) && (
                <Button
                  variant="ghost"
                  size="sm"
                  className="gap-1 text-muted-foreground"
                  onClick={() => {
                    setFilterScore([]);
                    setFilterModel([]);
                    setFilterMessageCount([]);
                    fetchSessions(
                      1,
                      pageInfo.pageSize,
                      timeRange,
                      customStart,
                      customEnd,
                      sort,
                      keyword,
                      [],
                      [],
                      [],
                    );
                  }}
                >
                  <X className="size-3.5" />
                  {t("common.clear")}
                </Button>
              )}
            </div>
            <div className="flex items-center gap-2">
              <SearchInput
                placeholder={t("sessions.search_placeholder")}
                value={searchInput}
                onChange={setSearchInput}
                onSearch={handleSearch}
                clearable
                onClear={() => {
                  setSearchInput("");
                  setKeyword("");
                  fetchSessions(
                    1,
                    pageInfo.pageSize,
                    timeRange,
                    customStart,
                    customEnd,
                    sort,
                    "",
                    filterScore,
                    filterModel,
                    filterMessageCount,
                  );
                }}
              />
              {selected.size > 0 && (
                <Button
                  variant="destructive"
                  size="sm"
                  disabled={isDemo()}
                  onClick={() => setBatchDeleteConfirmOpen(true)}
                  className="gap-1.5"
                >
                  {isDemo() ? <Lock className="size-3.5" /> : <Trash2 className="size-3.5" />}
                  {t("common.delete")} {selected.size}
                </Button>
              )}
            </div>
          </div>

          {loading ? (
            <TableSkeleton rows={5} rowClassName="h-10" />
          ) : sessions.length === 0 ? (
            <ListEmptyState
              icon={<MessageSquare className="mb-3 size-10 text-muted-foreground/50" />}
              message={t("sessions.no_sessions")}
            />
          ) : (
            <>
              {isMobile ? (
                <div className="space-y-3">
                  {sessions.map((s) => {
                    const isSelected = selected.has(s.id);
                    return (
                      <div
                        key={s.id}
                        className="cursor-pointer rounded-lg border border-border bg-card p-4 transition-colors hover:bg-secondary/50"
                        onClick={() => {
                          window.location.href = `/web/sessions/detail/?id=${s.id}`;
                        }}
                      >
                        <div className="flex items-start justify-between gap-3">
                          <div className="flex items-center gap-2 min-w-0 flex-1">
                            <div
                              role="checkbox"
                              aria-checked={isSelected}
                              tabIndex={0}
                              onClick={(e) => toggleSelect(s.id, e)}
                              onKeyDown={(e) => {
                                if (e.key === " " || e.key === "Enter")
                                  toggleSelect(s.id, e as unknown as React.MouseEvent);
                              }}
                              className={`mt-0.5 flex size-4 shrink-0 cursor-pointer items-center justify-center rounded border transition-colors ${
                                isSelected
                                  ? "border-primary bg-primary text-primary-foreground"
                                  : "border-muted-foreground/30 hover:border-muted-foreground"
                              }`}
                            >
                              {isSelected && <Check className="size-3" />}
                            </div>
                            <div className="min-w-0 flex-1">
                              <p className="truncate text-sm font-medium">
                                {s.summary ||
                                  t("sessions.untitled_session").replace("{id}", String(s.id))}
                              </p>
                            </div>
                          </div>
                          <div className="flex items-center gap-2 shrink-0">
                            {!isDemo() && (
                            <ScoreDots
                              score={s.score}
                              scoring={scoring === s.id}
                              onScore={(v) => handleScoreSession(s.id, v)}
                              onClear={() => handleDeleteScore(s.id)}
                              size={isMobile ? 20 : 16}
                            />
                            )}
                            <Badge variant="secondary" className="text-xs">
                              {t("sessions.msg_count").replace(
                                "{count}",
                                String(s.messageCount ?? 0),
                              )}
                            </Badge>
                            <DeleteIconButton
                              locked={isDemo()}
                              disabled={deleteConfirm.loading && deleteConfirm.target?.id === s.id}
                              onClick={(e) => {
                                e.stopPropagation();
                                deleteConfirm.openDelete(s);
                              }}
                              aria-label={t("sessions.delete_aria")}
                            />
                          </div>
                        </div>
                        <div className="mt-2 flex flex-wrap items-center gap-3 text-xs text-muted-foreground">
                          <span>
                            {t("common.id")}: {s.id}
                          </span>
                          <span>
                            {t("sessions.tool_count").replace("{count}", String(s.toolCount ?? 0))}
                          </span>
                          {s.modelIds && s.modelIds.length > 0 && (
                            <div className="flex items-center gap-1">
                              {s.modelIds.map((m) => (
                                <ProviderIcon key={m} protocol={m} size={12} />
                              ))}
                            </div>
                          )}
                          <span>{formatDateTime(s.createdAt)}</span>
                        </div>
                      </div>
                    );
                  })}
                </div>
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead className="w-10">
                        <div
                          role="checkbox"
                          aria-checked={selected.size === sessions.length}
                          tabIndex={0}
                          onClick={toggleSelectAll}
                          onKeyDown={(e) => {
                            if (e.key === " " || e.key === "Enter") toggleSelectAll();
                          }}
                          className={`flex size-4 cursor-pointer items-center justify-center rounded border transition-colors ${
                            selected.size === sessions.length
                              ? "border-primary bg-primary text-primary-foreground"
                              : "border-muted-foreground/30 hover:border-muted-foreground"
                          }`}
                        >
                          {selected.size === sessions.length && <Check className="size-3" />}
                        </div>
                      </TableHead>
                      <TableHead>{t("common.id")}</TableHead>
                      <TableHead
                        className="cursor-pointer select-none whitespace-nowrap"
                        onClick={() => handleSort(SORTABLE_COLUMNS.createdAt)}
                      >
                        <span className="inline-flex items-center gap-1">
                          {t("sessions.time")} {renderSortIcon(SORTABLE_COLUMNS.createdAt)}
                        </span>
                      </TableHead>
                      <TableHead>{t("sessions.summary")}</TableHead>
                      <TableHead className="w-[160px] text-center">{t("sessions.score")}</TableHead>
                      <TableHead
                        className="cursor-pointer select-none whitespace-nowrap"
                        onClick={() => handleSort(SORTABLE_COLUMNS.messageCount)}
                      >
                        <span className="inline-flex items-center gap-1">
                          {t("sessions.messages")} {renderSortIcon(SORTABLE_COLUMNS.messageCount)}
                        </span>
                      </TableHead>
                      <TableHead
                        className="cursor-pointer select-none whitespace-nowrap"
                        onClick={() => handleSort(SORTABLE_COLUMNS.toolCount)}
                      >
                        <span className="inline-flex items-center gap-1">
                          {t("sessions.tools")} {renderSortIcon(SORTABLE_COLUMNS.toolCount)}
                        </span>
                      </TableHead>
                      <TableHead className="w-[140px]">{t("sessions.models")}</TableHead>
                      <TableHead className="w-16 sr-only">{t("common.actions")}</TableHead>
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
                            window.location.href = `/web/sessions/detail/?id=${s.id}`;
                          }}
                        >
                          <TableCell className="w-10">
                            <div
                              role="checkbox"
                              aria-checked={isSelected}
                              tabIndex={0}
                              onClick={(e) => toggleSelect(s.id, e)}
                              onKeyDown={(e) => {
                                if (e.key === " " || e.key === "Enter")
                                  toggleSelect(s.id, e as unknown as React.MouseEvent);
                              }}
                              className={`flex size-4 cursor-pointer items-center justify-center rounded border transition-colors ${
                                isSelected
                                  ? "border-primary bg-primary text-primary-foreground"
                                  : "border-muted-foreground/30 hover:border-muted-foreground"
                              }`}
                            >
                              {isSelected && <Check className="size-3" />}
                            </div>
                          </TableCell>
                          <TableCell className="font-mono text-xs">{s.id}</TableCell>
                          <TableCell className="text-muted-foreground">
                            {formatDateTime(s.createdAt)}
                          </TableCell>
                          <TableCell className="max-w-[200px] truncate">
                            {s.summary || "—"}
                          </TableCell>
                          <TableCell className="w-[160px]">
                            <div className="flex justify-center">
                              <ScoreDots
                                score={s.score}
                                scoring={scoring === s.id}
                                onScore={(v) => handleScoreSession(s.id, v)}
                                onClear={() => handleDeleteScore(s.id)}
                                size={16}
                              />
                            </div>
                          </TableCell>
                          <TableCell>{s.messageCount ?? 0}</TableCell>
                          <TableCell>{s.toolCount ?? 0}</TableCell>
                          <TableCell>
                            <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
                              {s.modelIds && s.modelIds.length > 0 ? (
                                s.modelIds.map((m) => (
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
                          <TableCell className="w-16">
                            <div className="flex justify-center">
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
