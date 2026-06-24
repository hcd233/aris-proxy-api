"use client";

import { useCallback, useEffect, useState } from "react";
import { usePersistentState } from "@/hooks/use-persistent-state";
import { api } from "@/lib/api-client";
import type { SessionSummary, PageInfo } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import {
  MessageSquare,
  Check,
  ArrowUp,
  ArrowDown,
  Trash2,
  AlertTriangle,
  Search,
  X,
} from "lucide-react";
import { useT } from "@/lib/i18n";
import { useIsMobile } from "@/hooks/use-mobile";
import { ScoreDots } from "@/components/session-detail/score-dots";
import { PaginationBar } from "@/components/pagination-bar";
import { TimeRangePicker } from "@/components/ui/time-range-picker";
import type { TimeRangeKey } from "@/lib/time-range";
import { computeRange } from "@/lib/time-range";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { toast } from "sonner";
import { MultiSelectPill } from "@/components/ui/multi-select-pill";
import { ProviderIcon } from "@/components/provider-icon";

type SortDir = "asc" | "desc";

const SORTABLE_COLUMNS: Record<string, string> = {
  createdAt: "created_at",
  messageCount: "message_count",
  toolCount: "tool_count",
};

function formatDateTime(dateStr: string): string {
  const d = new Date(dateStr);
  const year = d.getFullYear();
  const month = d.getMonth() + 1;
  const day = d.getDate();
  const hours = String(d.getHours()).padStart(2, "0");
  const minutes = String(d.getMinutes()).padStart(2, "0");
  const seconds = String(d.getSeconds()).padStart(2, "0");
  return `${year}/${month}/${day} ${hours}:${minutes}:${seconds}`;
}

export default function SessionsPage() {
  const t = useT();
  const isMobile = useIsMobile();
  const [sessions, setSessions] = useState<SessionSummary[]>([]);
  const [persistedPage, setPersistedPage] = usePersistentState("dashboard.sessions.page", 1);
  const [persistedPageSize, setPersistedPageSize] = usePersistentState("dashboard.sessions.pageSize", 20);
  const [pageInfo, setPageInfo] = useState<PageInfo>({
    page: persistedPage,
    pageSize: persistedPageSize,
    total: 0,
  });
  const [loading, setLoading] = useState(true);
  const [timeRange, setTimeRange] = usePersistentState<TimeRangeKey>("dashboard.sessions.timeRange", "30d");
  const [customStart, setCustomStart] = usePersistentState("dashboard.sessions.customStart", "");
  const [customEnd, setCustomEnd] = usePersistentState("dashboard.sessions.customEnd", "");
  const [sort, setSort] = useState<{ field: string; dir: SortDir }>({ field: "created_at", dir: "desc" });
  const [deleting, setDeleting] = useState<number | null>(null);
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<{ id: number; summary: string } | null>(null);
  const [scoring, setScoring] = useState<number | null>(null);
  const [keyword, setKeyword] = useState("");
  const [searchInput, setSearchInput] = useState("");
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [batchDeleting, setBatchDeleting] = useState(false);
  const [batchDeleteConfirmOpen, setBatchDeleteConfirmOpen] = useState(false);
  const [filterScore, setFilterScore] = useState<string[]>([]);
  const [filterModel, setFilterModel] = useState<string[]>([]);
  const [scoreOptions, setScoreOptions] = useState<string[]>([]);
  const [modelOptions, setModelOptions] = useState<string[]>([]);

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

  /* eslint-disable react-hooks/set-state-in-effect -- Re-fetch filter options when the time range changes */
  useEffect(() => {
    fetchScoreOptions(timeRange, customStart, customEnd);
    fetchModelOptions(timeRange, customStart, customEnd);
  }, [timeRange, customStart, customEnd, fetchScoreOptions, fetchModelOptions]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const buildSessionFilter = (scores: string[], models: string[]): string | undefined => {
    const parts: string[] = [];
    if (scores.length > 0) parts.push(`score:${scores.join("|")}`);
    if (models.length > 0) parts.push(`model:${models.join("|")}`);
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
          filter: buildSessionFilter(score, models),
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

  /* eslint-disable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps -- Initial data fetch on mount */
  useEffect(() => {
    fetchSessions(persistedPage, persistedPageSize, "30d", "", "", { field: "created_at", dir: "desc" }, "", [], []);
  }, [fetchSessions]);
  /* eslint-enable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps */

  const refresh = (page: number, pageSize?: number) =>
    fetchSessions(page, pageSize ?? pageInfo.pageSize, timeRange, customStart, customEnd, sort, keyword, filterScore, filterModel);

  const handleSort = (field: string) => {
    const newSort: { field: string; dir: SortDir } =
      sort.field === field
        ? { field, dir: sort.dir === "asc" ? "desc" : "asc" }
        : { field, dir: "desc" };
    setSort(newSort);
    fetchSessions(1, pageInfo.pageSize, timeRange, customStart, customEnd, newSort, keyword, filterScore, filterModel);
  };

  const handleSearch = () => {
    const kw = searchInput.trim();
    setKeyword(kw);
    setSelected(new Set());
    fetchSessions(1, pageInfo.pageSize, timeRange, customStart, customEnd, sort, kw, filterScore, filterModel);
  };

  const renderSortIcon = (field: string) => {
    if (sort.field !== field) return null;
    return sort.dir === "asc" ? <ArrowUp className="size-3" /> : <ArrowDown className="size-3" />;
  };

  const openDeleteConfirm = (s: SessionSummary, e: React.MouseEvent) => {
    e.stopPropagation();
    setDeleteTarget({ id: s.id, summary: s.summary || `Session #${s.id}` });
    setDeleteConfirmOpen(true);
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;
    setDeleting(deleteTarget.id);
    try {
      await api.deleteSession(deleteTarget.id);
      toast.success("Session deleted");
      fetchSessions(pageInfo.page, pageInfo.pageSize, timeRange, customStart, customEnd, sort, keyword, filterScore, filterModel, true);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to delete session");
    } finally {
      setDeleting(null);
      setDeleteConfirmOpen(false);
      setDeleteTarget(null);
    }
  };

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
        toast.warning(`${rsp.deletedCount} deleted, ${failed} failed`);
      } else {
        toast.success(`${rsp.deletedCount} sessions deleted`);
      }
      setSelected(new Set());
      fetchSessions(1, pageInfo.pageSize, timeRange, customStart, customEnd, sort, keyword, filterScore, filterModel, true);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to batch delete");
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
      setSessions((prev) =>
        prev.map((s) => (s.id === sessionId ? { ...s, score } : s)),
      );
      toast.success("Scored");
    } catch {
      toast.error("Failed to score");
    } finally {
      setScoring(null);
    }
  };

  const handleDeleteScore = async (sessionId: number) => {
    if (scoring !== null) return;
    setScoring(sessionId);
    try {
      await api.deleteScoreSession(sessionId);
      setSessions((prev) =>
        prev.map((s) => (s.id === sessionId ? { ...s, score: undefined } : s)),
      );
      toast.success("Score removed");
    } catch {
      toast.error("Failed to remove score");
    } finally {
      setScoring(null);
    }
  };

  return (
    <div className="space-y-8">
      <div>
        <h1 className="font-display text-2xl md:text-3xl font-semibold tracking-tight text-foreground">{t("sessions.title")}</h1>
        <p className="mt-1.5 text-sm text-muted-foreground">
          {t("sessions.subtitle")}
        </p>
      </div>

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
                  fetchSessions(1, pageInfo.pageSize, key, cs, ce, sort, keyword, filterScore, filterModel);
                }}
              />
              <MultiSelectPill
                label={t("sessions.filter_score")}
                options={scoreOptions}
                value={filterScore}
                onChange={(v) => {
                  setFilterScore(v);
                  fetchSessions(1, pageInfo.pageSize, timeRange, customStart, customEnd, sort, keyword, v, filterModel);
                }}
              />
              <MultiSelectPill
                label={t("sessions.filter_model")}
                options={modelOptions}
                value={filterModel}
                onChange={(v) => {
                  setFilterModel(v);
                  fetchSessions(1, pageInfo.pageSize, timeRange, customStart, customEnd, sort, keyword, filterScore, v);
                }}
              />
              {(filterScore.length > 0 || filterModel.length > 0) && (
                <Button
                  variant="ghost"
                  size="sm"
                  className="gap-1 text-muted-foreground"
                  onClick={() => {
                    setFilterScore([]);
                    setFilterModel([]);
                    fetchSessions(1, pageInfo.pageSize, timeRange, customStart, customEnd, sort, keyword, [], []);
                  }}
                >
                  <X className="size-3.5" />
                  {t("common.clear")}
                </Button>
              )}
            </div>
            <div className="flex items-center gap-2">
              <div className="relative w-full md:max-w-sm">
                <Search className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  placeholder={t("sessions.search_placeholder")}
                  value={searchInput}
                  onChange={(e) => setSearchInput(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") handleSearch();
                  }}
                  className="pl-9 pr-8"
                />
                {searchInput && (
                  <button
                    type="button"
                    onClick={() => { setSearchInput(""); setKeyword(""); fetchSessions(1, pageInfo.pageSize, timeRange, customStart, customEnd, sort, "", filterScore, filterModel); }}
                    className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                  >
                    <X className="size-4" />
                  </button>
                )}
              </div>
              {selected.size > 0 && (
                <Button
                  variant="destructive"
                  size="sm"
                  onClick={() => setBatchDeleteConfirmOpen(true)}
                  className="gap-1.5"
                >
                  <Trash2 className="size-3.5" />
                  {t("common.delete")} {selected.size}
                </Button>
              )}
            </div>
          </div>

          {loading ? (
            <div className="space-y-3">
              {Array.from({ length: 5 }).map((_, i) => (
                <Skeleton key={i} className="h-10 w-full" />
              ))}
            </div>
          ) : sessions.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-12 text-center">
              <MessageSquare className="mb-3 size-10 text-muted-foreground/50" />
              <p className="text-sm text-muted-foreground">{t("sessions.no_sessions")}</p>
            </div>
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
                            onKeyDown={(e) => { if (e.key === " " || e.key === "Enter") toggleSelect(s.id, e as unknown as React.MouseEvent); }}
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
                              {s.summary || `Session #${s.id}`}
                            </p>
                          </div>
                        </div>
                        <div className="flex items-center gap-2 shrink-0">
                          <ScoreDots
                            score={s.score}
                            scoring={scoring === s.id}
                            onScore={(v) => handleScoreSession(s.id, v)}
                            onClear={() => handleDeleteScore(s.id)}
                            size={isMobile ? 20 : 16}
                          />
                          <Badge variant="secondary" className="text-xs">
                            {s.messageCount ?? 0} msgs
                          </Badge>
                          <Button
                            variant="ghost"
                            size="icon-sm"
                            disabled={deleting === s.id}
                            onClick={(e) => openDeleteConfirm(s, e)}
                            className="size-8 text-muted-foreground hover:text-destructive"
                            aria-label="Delete session"
                          >
                            <Trash2 className="size-4" />
                          </Button>
                        </div>
                      </div>
                      <div className="mt-2 flex flex-wrap items-center gap-3 text-xs text-muted-foreground">
                        <span>ID: {s.id}</span>
                        <span>{s.toolCount ?? 0} tools</span>
                        {s.models && s.models.length > 0 && (
                          <div className="flex items-center gap-1">
                            {s.models.map((m) => <ProviderIcon key={m} protocol={m} size={12} />)}
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
                        onKeyDown={(e) => { if (e.key === " " || e.key === "Enter") toggleSelectAll(); }}
                        className={`flex size-4 cursor-pointer items-center justify-center rounded border transition-colors ${
                          selected.size === sessions.length
                            ? "border-primary bg-primary text-primary-foreground"
                            : "border-muted-foreground/30 hover:border-muted-foreground"
                        }`}
                      >
                        {selected.size === sessions.length && <Check className="size-3" />}
                      </div>
                    </TableHead>
                    <TableHead>ID</TableHead>
                    <TableHead
                      className="cursor-pointer select-none whitespace-nowrap"
                      onClick={() => handleSort(SORTABLE_COLUMNS.createdAt)}
                    >
                      <span className="inline-flex items-center gap-1">Time {renderSortIcon(SORTABLE_COLUMNS.createdAt)}</span>
                    </TableHead>
                    <TableHead>Summary</TableHead>
                    <TableHead className="w-[160px] text-center">Score</TableHead>
                    <TableHead
                      className="cursor-pointer select-none whitespace-nowrap"
                      onClick={() => handleSort(SORTABLE_COLUMNS.messageCount)}
                    >
                      <span className="inline-flex items-center gap-1">Messages {renderSortIcon(SORTABLE_COLUMNS.messageCount)}</span>
                    </TableHead>
                    <TableHead
                      className="cursor-pointer select-none whitespace-nowrap"
                      onClick={() => handleSort(SORTABLE_COLUMNS.toolCount)}
                    >
                      <span className="inline-flex items-center gap-1">Tools {renderSortIcon(SORTABLE_COLUMNS.toolCount)}</span>
                    </TableHead>
                    <TableHead className="w-[140px]">Models</TableHead>
                    <TableHead className="w-16 sr-only">Actions</TableHead>
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
                            onKeyDown={(e) => { if (e.key === " " || e.key === "Enter") toggleSelect(s.id, e as unknown as React.MouseEvent); }}
                            className={`flex size-4 cursor-pointer items-center justify-center rounded border transition-colors ${
                              isSelected
                                ? "border-primary bg-primary text-primary-foreground"
                                : "border-muted-foreground/30 hover:border-muted-foreground"
                            }`}
                          >
                            {isSelected && <Check className="size-3" />}
                          </div>
                        </TableCell>
                        <TableCell className="font-mono text-xs">
                          {s.id}
                        </TableCell>
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
                            {s.models && s.models.length > 0 ? (
                              s.models.map((m) => (
                                <span key={m} className="inline-flex items-center gap-1 text-xs text-muted-foreground">
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
                            <Button
                              variant="ghost"
                              size="icon-sm"
                              disabled={deleting === s.id}
                              onClick={(e) => openDeleteConfirm(s, e)}
                              className="size-8 text-muted-foreground hover:text-destructive"
                              aria-label="Delete session"
                            >
                              <Trash2 className="size-4" />
                            </Button>
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
                totalLabel="sessions"
              />
            </>
          )}
          </CardContent>
        </Card>

        <AlertDialog open={deleteConfirmOpen} onOpenChange={setDeleteConfirmOpen}>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle className="flex items-center gap-2">
                <AlertTriangle className="size-5 text-destructive" />
                Are you sure?
              </AlertDialogTitle>
              <AlertDialogDescription>
                This will permanently delete session <strong>{deleteTarget?.summary}</strong> and all its messages. This action cannot be undone.
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>Cancel</AlertDialogCancel>
              <AlertDialogAction variant="destructive" onClick={handleDelete} disabled={deleting !== null}>
                {deleting !== null ? "Deleting..." : "Delete"}
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>

        <AlertDialog open={batchDeleteConfirmOpen} onOpenChange={setBatchDeleteConfirmOpen}>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle className="flex items-center gap-2">
                <AlertTriangle className="size-5 text-destructive" />
                Batch delete sessions?
              </AlertDialogTitle>
              <AlertDialogDescription>
                This will permanently delete <strong>{selected.size}</strong> session{selected.size !== 1 ? "s" : ""} and all their messages. This action cannot be undone.
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>Cancel</AlertDialogCancel>
              <AlertDialogAction variant="destructive" onClick={handleBatchDelete} disabled={batchDeleting}>
                {batchDeleting ? "Deleting..." : `Delete ${selected.size}`}
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </div>
  );
}
