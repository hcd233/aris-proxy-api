"use client";

/**
 * Shares management page — lists every active share link the current user
 * owns, lets them copy a link, jump to the source session, or revoke a share.
 */

import { useCallback, useEffect, useState } from "react";
import { usePersistentState } from "@/hooks/use-persistent-state";
import { Check, Copy, ExternalLink, Share2 } from "lucide-react";
import { toast } from "sonner";
import { api } from "@/lib/api-client";
import { showErrorToast } from "@/lib/api-error-handler";
import type { PageInfo, ShareItem } from "@/lib/types";
import { buildShareURL } from "@/components/share/share-dialog";
import { DeleteButton } from "@/components/delete-button";
import { DeleteConfirmDialog } from "@/components/delete-confirm-dialog";
import { PageHeader } from "@/components/page-header";
import { PermissionGuard } from "@/components/permission-guard";
import { ListEmptyState } from "@/components/list-empty-state";
import { TableSkeleton } from "@/components/table-skeleton";
import { useDeleteConfirm } from "@/hooks/use-delete-confirm";
import { useRequestSeq } from "@/hooks/use-request-seq";
import { formatDateTime, formatExpiry } from "@/lib/utils";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { PaginationBar } from "@/components/pagination-bar";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { TooltipRoot, TooltipTrigger, TooltipContent } from "@/components/ui/tooltip";
import { useT } from "@/lib/i18n";
import { copyTextToClipboard } from "@/lib/clipboard";
import { RowDetailDrawer } from "@/components/row-detail-drawer";
import { rowDetailProps } from "@/lib/row-detail";
import { useIsCompact } from "@/hooks/use-mobile";

export default function SharesPage() {
  const t = useT();
  const [shares, setShares] = useState<ShareItem[]>([]);
  const [persistedPage, setPersistedPage] = usePersistentState("dashboard.shares.page", 1);
  const [persistedPageSize, setPersistedPageSize] = usePersistentState(
    "dashboard.shares.pageSize",
    20,
  );
  const [pageInfo, setPageInfo] = useState<PageInfo>({
    page: persistedPage,
    pageSize: persistedPageSize,
    total: 0,
  });
  const [loading, setLoading] = useState(true);
  const [copiedID, setCopiedID] = useState<string | null>(null);
  // 窄屏（<1024px）表格只留关键列，点行看全部字段
  const isCompact = useIsCompact();
  const [detail, setDetail] = useState<ShareItem | null>(null);
  // Snapshot of "now" taken at fetch time so the expired-check during render
  // stays pure (react-hooks/purity forbids `Date.now()` inside render).
  const [refreshedAt, setRefreshedAt] = useState<number>(0);

  // 竞态守卫：翻页连点时慢响应不得覆盖新响应
  const requestSeq = useRequestSeq();
  const fetchShares = useCallback(
    async (page: number, pageSize: number) => {
      const seq = requestSeq.begin();
      setLoading(true);
      try {
        const rsp = await api.listShares(page, pageSize);
        if (!requestSeq.isCurrent(seq)) return;
        if (rsp.error) {
          showErrorToast(rsp.error, { title: t("common.error") });
          setShares([]);
          return;
        }
        setShares(rsp.shares ?? []);
        if (rsp.pageInfo) {
          setPageInfo(rsp.pageInfo);
          setPersistedPage(rsp.pageInfo.page);
          setPersistedPageSize(rsp.pageInfo.pageSize);
        }
        setRefreshedAt(Date.now());
      } catch (err) {
        if (!requestSeq.isCurrent(seq)) return;
        showErrorToast(err, { title: t("common.error") });
      } finally {
        if (requestSeq.isCurrent(seq)) setLoading(false);
      }
    },
    [setPersistedPage, setPersistedPageSize, t, requestSeq],
  );

  /* eslint-disable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps -- Data fetching requires setting state from async effects on mount */
  useEffect(() => {
    fetchShares(persistedPage, persistedPageSize);
  }, [fetchShares]);
  /* eslint-enable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps */

  const handleCopy = useCallback(
    async (share: ShareItem) => {
      const url = buildShareURL(share.shareId);
      const ok = await copyTextToClipboard(url);
      if (ok) {
        setCopiedID(share.shareId);
        toast.success(t("common.copied_to_clipboard"));
        window.setTimeout(() => setCopiedID(null), 2000);
      } else {
        toast.error(t("shares.copy_error"));
      }
    },
    [t],
  );

  const isExpired = (share: ShareItem) =>
    refreshedAt > 0 && new Date(share.expiresAt).getTime() < refreshedAt;

  const sessionLink = (share: ShareItem) => (
    <a
      href={`/web/sessions/detail/?id=${share.sessionId}`}
      className="inline-flex items-center gap-1 font-mono text-xs text-primary hover:underline"
    >
      #{share.sessionId}
      <ExternalLink className="size-3" />
    </a>
  );

  const statusBadge = (expired: boolean) => (
    <Badge variant={expired ? "destructive" : "secondary"}>
      {expired ? t("shares.expired") : t("shares.active")}
    </Badge>
  );

  const detailFields = (share: ShareItem) => [
    {
      label: t("shares.share_id"),
      value: isExpired(share) ? (
        <span className="font-mono text-xs">{share.shareId}</span>
      ) : (
        <a
          href={buildShareURL(share.shareId)}
          target="_blank"
          rel="noopener noreferrer"
          className="font-mono text-xs text-primary hover:underline"
        >
          {share.shareId}
        </a>
      ),
    },
    { label: t("shares.session_id"), value: sessionLink(share) },
    { label: t("common.created"), value: formatDateTime(share.createdAt) },
    {
      label: t("shares.expires_at"),
      value: formatExpiry(share.expiresAt, t("shares.never_expires")),
    },
    { label: t("shares.status"), value: statusBadge(isExpired(share)) },
  ];

  const deleteConfirm = useDeleteConfirm<ShareItem>({
    onConfirm: async (share) => {
      const rsp = await api.deleteShare(share.shareId);
      if (rsp.error) {
        showErrorToast(rsp.error, { title: t("shares.revoke_error") });
        return;
      }
      toast.success(t("shares.revoke_success"));
      fetchShares(pageInfo.page, pageInfo.pageSize);
    },
    onError: (err) => showErrorToast(err, { title: t("shares.revoke_error") }),
  });

  return (
    <PermissionGuard>
      <div className="space-y-8">
        <PageHeader title={t("shares.title")} description={t("shares.subtitle")} />

        <Card>
          <CardHeader>
            <CardTitle className="font-display">{t("shares.share_links")}</CardTitle>
          </CardHeader>
          <CardContent>
            {loading ? (
              <TableSkeleton rows={4} />
            ) : shares.length === 0 ? (
              <ListEmptyState
                icon={Share2}
                message={t("shares.no_shares")}
                hint={t("shares.create_hint")}
              />
            ) : (
              <>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead priority="lg">{t("shares.share_id")}</TableHead>
                      <TableHead>{t("shares.session_id")}</TableHead>
                      <TableHead priority="md">{t("common.created")}</TableHead>
                      <TableHead priority="md">{t("shares.expires_at")}</TableHead>
                      <TableHead>{t("shares.status")}</TableHead>
                      <TableHead priority="md" className="text-right">
                        {t("common.actions")}
                      </TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {shares.map((share) => {
                      const expired =
                        refreshedAt > 0 && new Date(share.expiresAt).getTime() < refreshedAt;
                      return (
                        <TableRow
                          key={share.shareId}
                          className={expired ? "bg-muted/30 text-muted-foreground" : undefined}
                          {...rowDetailProps(
                            isCompact,
                            () => setDetail(share),
                            `${t("row_detail.view")}: #${share.sessionId}`,
                          )}
                        >
                          <TableCell priority="lg" className="max-w-[220px] font-mono text-xs">
                            <TooltipRoot>
                              <TooltipTrigger
                                render={
                                  <span className="block truncate text-left">
                                    {expired ? (
                                      <span>{share.shareId}</span>
                                    ) : (
                                      <a
                                        href={buildShareURL(share.shareId)}
                                        target="_blank"
                                        rel="noopener noreferrer"
                                        className="inline-flex items-center gap-1 text-primary hover:underline"
                                      >
                                        {share.shareId}
                                        <ExternalLink className="size-3" />
                                      </a>
                                    )}
                                  </span>
                                }
                              />
                              <TooltipContent side="top" className="max-w-xs break-all">
                                {share.shareId}
                              </TooltipContent>
                            </TooltipRoot>
                          </TableCell>
                          <TableCell>{sessionLink(share)}</TableCell>
                          <TableCell priority="md" className="text-xs text-muted-foreground">
                            {formatDateTime(share.createdAt)}
                          </TableCell>
                          <TableCell
                            priority="md"
                            className={
                              expired
                                ? "text-xs font-medium text-rose-500"
                                : "text-xs text-muted-foreground"
                            }
                          >
                            {formatExpiry(share.expiresAt, t("shares.never_expires"))}
                          </TableCell>
                          <TableCell>{statusBadge(expired)}</TableCell>
                          <TableCell priority="md" className="text-right">
                            <div className="flex items-center justify-end gap-1.5">
                              <Button
                                variant="outline"
                                size="xs"
                                onClick={() => handleCopy(share)}
                                className="gap-1"
                                disabled={expired}
                              >
                                {copiedID === share.shareId ? (
                                  <>
                                    <Check className="size-3" />
                                    {t("common.copied")}
                                  </>
                                ) : (
                                  <>
                                    <Copy className="size-3" />
                                    {t("common.copy")}
                                  </>
                                )}
                              </Button>
                              <DeleteButton
                                label={t("shares.revoke")}
                                onClick={() => deleteConfirm.openDelete(share)}
                              />
                            </div>
                          </TableCell>
                        </TableRow>
                      );
                    })}
                  </TableBody>
                </Table>

                <PaginationBar
                  pageInfo={pageInfo}
                  onChange={(page, pageSize) => fetchShares(page, pageSize)}
                  totalLabel={t("pagination.shares")}
                />
                <RowDetailDrawer
                  open={detail !== null}
                  onOpenChange={(open) => !open && setDetail(null)}
                  title={detail ? `#${detail.sessionId}` : ""}
                  fields={detail ? detailFields(detail) : []}
                  actions={
                    detail && (
                      <>
                        <Button
                          variant="outline"
                          onClick={() => handleCopy(detail)}
                          disabled={isExpired(detail)}
                        >
                          {copiedID === detail.shareId ? (
                            <Check data-icon="inline-start" />
                          ) : (
                            <Copy data-icon="inline-start" />
                          )}
                          {copiedID === detail.shareId ? t("common.copied") : t("common.copy")}
                        </Button>
                        <DeleteButton
                          label={t("shares.revoke")}
                          onClick={() => {
                            setDetail(null);
                            deleteConfirm.openDelete(detail);
                          }}
                        />
                      </>
                    )
                  }
                />
              </>
            )}
          </CardContent>
        </Card>

        <DeleteConfirmDialog
          {...deleteConfirm.dialogProps}
          title={t("shares.delete_confirm")}
          description={t("shares.delete_dialog_desc").replace(
            "{id}",
            String(deleteConfirm.target?.sessionId ?? ""),
          )}
          confirmLabel={t("shares.revoke")}
          loadingLabel={t("shares.revoking")}
        />
      </div>
    </PermissionGuard>
  );
}
