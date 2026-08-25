"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api-client";
import { showErrorToast } from "@/lib/api-error-handler";
import type { PageInfo, TraceSummary } from "@/lib/types";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import {
  TooltipProvider,
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
import { Check, Radar, Trash2 } from "lucide-react";
import { PaginationBar } from "@/components/pagination-bar";
import { toast } from "sonner";
import { usePersistentState } from "@/hooks/use-persistent-state";
import { useIsMobile } from "@/hooks/use-mobile";
import { useT } from "@/lib/i18n";
import TraceInstallPopover from "@/components/trace-install-popover";
import { DeleteIconButton } from "@/components/delete-button";
import { DeleteConfirmDialog } from "@/components/delete-confirm-dialog";
import { PageHeader } from "@/components/page-header";
import { ListEmptyState } from "@/components/list-empty-state";
import { TableSkeleton } from "@/components/table-skeleton";
import { useDeleteConfirm } from "@/hooks/use-delete-confirm";
import { formatDateTime } from "@/lib/utils";
import { PermissionGuard } from "@/components/permission-guard";
import { FilterBar } from "@/components/filter-bar/filter-bar";
import { useFilterBar } from "@/components/filter-bar/use-filter-bar";

export default function TracePage() {
  const [traces, setTraces] = useState<TraceSummary[]>([]);
  const [persistedPage, setPersistedPage] = usePersistentState("dashboard.trace.page", 1);
  const [persistedPageSize, setPersistedPageSize] = usePersistentState(
    "dashboard.trace.pageSize",
    20,
  );
  const [pageInfo, setPageInfo] = useState<PageInfo>({
    page: persistedPage,
    pageSize: persistedPageSize,
    total: 0,
  });
  const [loading, setLoading] = useState(true);
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [batchDeleteConfirmOpen, setBatchDeleteConfirmOpen] = useState(false);
  const [batchDeleting, setBatchDeleting] = useState(false);
  const t = useT();
  const isMobile = useIsMobile();

  const filterBar = useFilterBar({
    persistKey: "dashboard.trace",
    facets: [],
    freeTextPlaceholder: t("trace.search_placeholder"),
    legacyFreeTextKey: "dashboard.trace.keyword",
    legacyKeys: { _draft: "dashboard.trace.searchInput" },
  });
  const { queryParams } = filterBar;

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
    [setPersistedPage, setPersistedPageSize, t],
  );

  /* eslint-disable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps -- 关键词 token 变化回到第 1 页查询；挂载时以持久化关键词发起首次查询 */
  useEffect(() => {
    fetchTraces(1, pageInfo.pageSize, queryParams.freeText);
  }, [queryParams]);
  /* eslint-enable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps */

  const deleteConfirm = useDeleteConfirm<TraceSummary>({
    onConfirm: async (tr) => {
      await api.deleteTrace(tr.id);
      toast.success(t("trace.delete_success"));
      setSelected(new Set());
      fetchTraces(pageInfo.page, pageInfo.pageSize, queryParams.freeText, true);
    },
    onError: (err) => showErrorToast(err, { title: t("trace.delete_error") }),
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
            .replace("{failed}", String(failed)),
        );
      } else {
        toast.success(t("trace.batch_delete_success").replace("{count}", String(rsp.deletedCount)));
      }
      setSelected(new Set());
      fetchTraces(1, pageInfo.pageSize, queryParams.freeText, true);
    } catch (err) {
      showErrorToast(err, { title: t("trace.batch_delete_error") });
    } finally {
      setBatchDeleting(false);
      setBatchDeleteConfirmOpen(false);
    }
  };

  return (
    <PermissionGuard>
      <div className="space-y-8">
        <PageHeader
          title={t("trace.title")}
          description={t("trace.subtitle")}
          actions={
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
          }
        />

        <Card>
          <CardHeader>
            <CardTitle className="font-display">{t("trace.all_traces")}</CardTitle>
          </CardHeader>
          <CardContent>
            {/* Search — faceted bar */}
            <div className="mb-4 flex">
              <FilterBar {...filterBar} facets={[]} placeholder={t("trace.search_placeholder")} />
            </div>
            {filterBar.tokens.length > 0 && (
              <p className="-mt-2 mb-3 text-xs text-muted-foreground">
                {t("filter_bar.applied_count").replace("{count}", String(filterBar.tokens.length))}
              </p>
            )}

            {loading ? (
              <TableSkeleton rows={5} rowClassName="h-10" />
            ) : traces.length === 0 ? (
              <ListEmptyState
                icon={<Radar className="mb-3 size-10 text-muted-foreground/50" />}
                message={t("trace.no_traces")}
              />
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
                            <TooltipProvider>
                              <TooltipRoot>
                                <TooltipTrigger
                                  render={
                                    <p className="truncate font-mono text-sm font-medium">
                                      {tr.sessionId}
                                    </p>
                                  }
                                />
                                <TooltipContent side="top" className="max-w-xs break-all">
                                  {tr.sessionId}
                                </TooltipContent>
                              </TooltipRoot>
                            </TooltipProvider>
                          </div>
                          <div className="flex shrink-0 items-center gap-2">
                            <TooltipProvider>
                              <TooltipRoot>
                                <TooltipTrigger
                                  render={
                                    <DeleteIconButton
                                      aria-label={t("trace.delete_aria")}
                                      disabled={
                                        deleteConfirm.loading && deleteConfirm.target?.id === tr.id
                                      }
                                      onClick={(e) => {
                                        (e as unknown as React.MouseEvent).stopPropagation();
                                        deleteConfirm.openDelete(tr);
                                      }}
                                    />
                                  }
                                />
                                <TooltipContent side="top">{t("trace.delete_aria")}</TooltipContent>
                              </TooltipRoot>
                            </TooltipProvider>
                          </div>
                        </div>
                        <div className="mt-2 flex flex-wrap items-center gap-3 text-xs text-muted-foreground">
                          <span>ID: {tr.id}</span>
                          <span>{tr.agent}</span>
                          <span>{tr.model}</span>
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
                          <TableCell className="text-muted-foreground">
                            {formatDateTime(tr.createdAt)}
                          </TableCell>
                          <TableCell onClick={(e) => e.stopPropagation()}>
                            <TooltipProvider>
                              <TooltipRoot>
                                <TooltipTrigger
                                  render={
                                    <DeleteIconButton
                                      aria-label={t("trace.delete_aria")}
                                      disabled={
                                        deleteConfirm.loading && deleteConfirm.target?.id === tr.id
                                      }
                                      onClick={(e) => {
                                        (e as unknown as React.MouseEvent).stopPropagation();
                                        deleteConfirm.openDelete(tr);
                                      }}
                                    />
                                  }
                                />
                                <TooltipContent side="top">{t("trace.delete_aria")}</TooltipContent>
                              </TooltipRoot>
                            </TooltipProvider>
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                )}
                <PaginationBar
                  pageInfo={pageInfo}
                  onChange={(page, pageSize) => fetchTraces(page, pageSize, queryParams.freeText)}
                  totalLabel={t("trace.traces")}
                />
              </>
            )}
          </CardContent>
        </Card>

        <DeleteConfirmDialog
          {...deleteConfirm.dialogProps}
          title={t("trace.delete_dialog_title")}
          description={t("trace.delete_dialog_desc").replace(
            "{name}",
            deleteConfirm.target?.sessionId ?? String(deleteConfirm.target?.id ?? ""),
          )}
          confirmLabel={t("common.delete")}
          loadingLabel={t("common.deleting")}
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
    </PermissionGuard>
  );
}
