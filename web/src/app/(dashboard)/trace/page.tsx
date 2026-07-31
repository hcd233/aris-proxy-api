"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api-client";
import { showErrorToast } from "@/lib/api-error-handler";
import type { PageInfo, TraceSummary } from "@/lib/types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
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
import { Check, Radar, Search, Trash2, X } from "lucide-react";
import { PaginationBar } from "@/components/pagination-bar";
import { toast } from "sonner";
import { usePersistentState } from "@/hooks/use-persistent-state";
import { useIsMobile } from "@/hooks/use-mobile";
import { useT } from "@/lib/i18n";
import TraceInstallPopover from "@/components/trace-install-popover";
import { DeleteIconButton } from "@/components/delete-button";
import { DeleteConfirmDialog } from "@/components/delete-confirm-dialog";

function statusBadge(status: string, t: (k: string, f?: string) => string) {
  if (status === "active") {
    return <Badge variant="secondary">{t("trace.status_active")}</Badge>;
  }
  if (status === "done") {
    return <Badge variant="outline">{t("trace.status_done")}</Badge>;
  }
  return <Badge variant="outline">{status}</Badge>;
}

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

export default function TracePage() {
  const [traces, setTraces] = useState<TraceSummary[]>([]);
  const [persistedPage, setPersistedPage] = usePersistentState("dashboard.trace.page", 1);
  const [persistedPageSize, setPersistedPageSize] = usePersistentState("dashboard.trace.pageSize", 20);
  const [pageInfo, setPageInfo] = useState<PageInfo>({
    page: persistedPage,
    pageSize: persistedPageSize,
    total: 0,
  });
  const [loading, setLoading] = useState(true);
  const [keyword, setKeyword] = usePersistentState("dashboard.trace.keyword", "");
  const [searchInput, setSearchInput] = usePersistentState("dashboard.trace.searchInput", "");
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [deleteTarget, setDeleteTarget] = useState<TraceSummary | null>(null);
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);
  const [batchDeleteConfirmOpen, setBatchDeleteConfirmOpen] = useState(false);
  const [deleting, setDeleting] = useState<number | null>(null);
  const [batchDeleting, setBatchDeleting] = useState(false);
  const t = useT();
  const isMobile = useIsMobile();

  const fetchTraces = useCallback(
    async (page: number, pageSize: number, kw: string, silent?: boolean) => {
      if (!silent) setLoading(true);
      try {
        const safeSize = pageSize > 0 ? pageSize : 20;
        const rsp = await api.listTraces(page, safeSize, kw || undefined);
        setTraces(rsp.traces ?? []);
        if (rsp.pageInfo) {
          setPageInfo(rsp.pageInfo);
          setPersistedPage(rsp.pageInfo.page);
          setPersistedPageSize(rsp.pageInfo.pageSize);
        }
      } catch (err) {
        showErrorToast(err, { title: t("trace.load_error") });
      } finally {
        setLoading(false);
      }
    },
    [setPersistedPage, setPersistedPageSize, t]
  );

  /* eslint-disable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps -- Initial data fetch on mount with persisted filters */
  useEffect(() => {
    fetchTraces(persistedPage, persistedPageSize, keyword);
  }, [fetchTraces]);
  /* eslint-enable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps */

  const handleSearch = useCallback(() => {
    const kw = searchInput.trim();
    setKeyword(kw);
    fetchTraces(1, pageInfo.pageSize, kw);
  }, [fetchTraces, pageInfo.pageSize, searchInput, setKeyword]);

  const openDeleteConfirm = (tr: TraceSummary, e: React.MouseEvent) => {
    e.stopPropagation();
    setDeleteTarget(tr);
    setDeleteConfirmOpen(true);
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;
    setDeleting(deleteTarget.id);
    try {
      await api.deleteTrace(deleteTarget.id);
      toast.success(t("trace.delete_success"));
      setSelected(new Set());
      fetchTraces(pageInfo.page, pageInfo.pageSize, keyword, true);
    } catch (err) {
      showErrorToast(err, { title: t("trace.delete_error") });
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
    if (selected.size === traces.length) {
      setSelected(new Set());
    } else {
      setSelected(new Set(traces.map((tr) => tr.id)));
    }
  };

  const handleBatchDelete = async () => {
    if (selected.size === 0) return;
    setBatchDeleting(true);
    try {
      const ids = Array.from(selected);
      const rsp = await api.batchDeleteTraces(ids);
      const failed = rsp.failures?.length ?? 0;
      if (failed > 0) {
        toast.warning(
          t("trace.batch_delete_warning")
            .replace("{deleted}", String(rsp.deletedCount))
            .replace("{failed}", String(failed))
        );
      } else {
        toast.success(
          t("trace.batch_delete_success").replace("{count}", String(rsp.deletedCount))
        );
      }
      setSelected(new Set());
      fetchTraces(1, pageInfo.pageSize, keyword, true);
    } catch (err) {
      showErrorToast(err, { title: t("trace.batch_delete_error") });
    } finally {
      setBatchDeleting(false);
      setBatchDeleteConfirmOpen(false);
    }
  };

  return (
    <div className="space-y-8">
      <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
        <div>
          <h1 className="font-display text-2xl md:text-3xl font-semibold tracking-tight text-foreground">
            {t("trace.title")}
          </h1>
          <p className="mt-1.5 text-sm text-muted-foreground">{t("trace.subtitle")}</p>
        </div>
        <div className="flex items-center gap-2">
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
          <TraceInstallPopover />
        </div>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="font-display">{t("trace.all_traces")}</CardTitle>
        </CardHeader>
        <CardContent>
          {/* Search — always visible */}
          <div className="mb-4 flex flex-col gap-3 md:flex-row md:items-center md:justify-end">
            <div className="relative w-full md:max-w-sm">
              <Search className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                placeholder={t("trace.search_placeholder")}
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
                  onClick={() => { setSearchInput(""); setKeyword(""); fetchTraces(1, pageInfo.pageSize, ""); }}
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                >
                  <X className="size-4" />
                </button>
              )}
            </div>
          </div>

          {loading ? (
            <div className="space-y-3">
              {Array.from({ length: 5 }).map((_, i) => (
                <Skeleton key={i} className="h-10 w-full" />
              ))}
            </div>
          ) : traces.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-12 text-center">
              <Radar className="mb-3 size-10 text-muted-foreground/50" />
              <p className="text-sm text-muted-foreground">{t("trace.no_traces")}</p>
            </div>
          ) : (
            <>
              {isMobile ? (
                <div className="space-y-3">
                  {traces.map((tr) => (
                    <div
                      key={tr.id}
                      className="cursor-pointer rounded-lg border border-border bg-card p-4 transition-colors hover:bg-secondary/50"
                      onClick={() => {
                        window.location.href = `/web/trace/detail/?id=${tr.id}`;
                      }}
                    >
                      <div className="flex items-start justify-between gap-3">
                        <div className="min-w-0 flex-1">
                          <p className="truncate font-mono text-sm font-medium">{tr.sessionId}</p>
                        </div>
                        <div className="flex shrink-0 items-center gap-2">
                          {statusBadge(tr.status, t)}
                          <DeleteIconButton
                            aria-label={t("trace.delete_aria")}
                            title={t("trace.delete_aria")}
                            disabled={deleting === tr.id}
                            onClick={(e) => openDeleteConfirm(tr, e as unknown as React.MouseEvent)}
                          />
                        </div>
                      </div>
                      <div className="mt-2 flex flex-wrap items-center gap-3 text-xs text-muted-foreground">
                        <span>ID: {tr.id}</span>
                        <span>{tr.agent}</span>
                        <span>{tr.model}</span>
                        <span>{tr.source}</span>
                        <span>{formatDateTime(tr.createdAt)}</span>
                      </div>
                    </div>
                  ))}
                </div>
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead className="w-10">
                        <div
                          role="checkbox"
                          tabIndex={0}
                          aria-checked={selected.size === traces.length && traces.length > 0}
                          onClick={toggleSelectAll}
                          onKeyDown={(e) => {
                            if (e.key === " " || e.key === "Enter") toggleSelectAll();
                          }}
                          className="flex size-4 cursor-pointer items-center justify-center rounded-sm border border-border transition-colors"
                        >
                          {selected.size === traces.length && traces.length > 0 && (
                            <Check className="size-3" />
                          )}
                        </div>
                      </TableHead>
                      <TableHead className="w-16">{t("common.id")}</TableHead>
                      <TableHead>{t("trace.session_id")}</TableHead>
                      <TableHead>{t("trace.agent")}</TableHead>
                      <TableHead>{t("trace.api_key")}</TableHead>
                      <TableHead>{t("trace.model")}</TableHead>
                      <TableHead>{t("trace.source")}</TableHead>
                      <TableHead className="w-24">{t("trace.status")}</TableHead>
                      <TableHead className="w-40">{t("trace.created_at")}</TableHead>
                      <TableHead className="w-12" />
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {traces.map((tr) => (
                      <TableRow
                        key={tr.id}
                        className="cursor-pointer"
                        onClick={() => {
                          window.location.href = `/web/trace/detail/?id=${tr.id}`;
                        }}
                      >
                        <TableCell onClick={(e) => e.stopPropagation()}>
                          <div
                            role="checkbox"
                            tabIndex={0}
                            aria-checked={selected.has(tr.id)}
                            onClick={(e) => toggleSelect(tr.id, e as unknown as React.MouseEvent)}
                            onKeyDown={(e) => {
                              if (e.key === " " || e.key === "Enter") {
                                toggleSelect(tr.id, e as unknown as React.MouseEvent);
                              }
                            }}
                            className="flex size-4 cursor-pointer items-center justify-center rounded-sm border border-border transition-colors"
                          >
                            {selected.has(tr.id) && <Check className="size-3" />}
                          </div>
                        </TableCell>
                        <TableCell className="font-mono text-xs">{tr.id}</TableCell>
                        <TableCell className="font-mono">{tr.sessionId}</TableCell>
                        <TableCell>{tr.agent}</TableCell>
                        <TableCell>{tr.apiKeyName}</TableCell>
                        <TableCell>{tr.model}</TableCell>
                        <TableCell>{tr.source}</TableCell>
                        <TableCell>{statusBadge(tr.status, t)}</TableCell>
                        <TableCell className="text-muted-foreground">{formatDateTime(tr.createdAt)}</TableCell>
                        <TableCell onClick={(e) => e.stopPropagation()}>
                          <DeleteIconButton
                            aria-label={t("trace.delete_aria")}
                            title={t("trace.delete_aria")}
                            disabled={deleting === tr.id}
                            onClick={(e) => openDeleteConfirm(tr, e as unknown as React.MouseEvent)}
                          />
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}
              <PaginationBar
                pageInfo={pageInfo}
                onChange={(page, pageSize) => fetchTraces(page, pageSize, keyword)}
                totalLabel={t("trace.traces")}
              />
            </>
          )}
        </CardContent>
      </Card>

      <DeleteConfirmDialog
        open={deleteConfirmOpen}
        onOpenChange={setDeleteConfirmOpen}
        title={t("trace.delete_dialog_title")}
        description={t("trace.delete_dialog_desc").replace(
          "{name}",
          deleteTarget?.sessionId ?? String(deleteTarget?.id ?? "")
        )}
        confirmLabel={t("common.delete")}
        loadingLabel={t("common.deleting")}
        loading={deleting !== null}
        onConfirm={handleDelete}
      />

      <DeleteConfirmDialog
        open={batchDeleteConfirmOpen}
        onOpenChange={setBatchDeleteConfirmOpen}
        title={t("trace.batch_delete_title")}
        description={t("trace.batch_delete_desc").replace("{count}", String(selected.size))}
        confirmLabel={`${t("common.delete")} ${selected.size}`}
        loadingLabel={t("common.deleting")}
        loading={batchDeleting}
        onConfirm={handleBatchDelete}
      />

    </div>
  );
}
