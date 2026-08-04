"use client";

/**
 * Shares management page — lists every active share link the current user
 * owns, lets them copy a link, jump to the source session, or revoke a share.
 */

import { useCallback, useEffect, useState } from "react";
import { usePersistentState } from "@/hooks/use-persistent-state";
import {
  Check,
  Copy,
  ExternalLink,
  Share2,
} from "lucide-react";
import { toast } from "sonner";
import { api } from "@/lib/api-client";
import { showErrorToast } from "@/lib/api-error-handler";
import type { PageInfo, ShareItem } from "@/lib/types";
import { buildShareURL } from "@/components/share/share-dialog";
import { DeleteButton } from "@/components/delete-button";
import { DeleteConfirmDialog } from "@/components/delete-confirm-dialog";
import { PageHeader } from "@/components/page-header";
import { ListEmptyState } from "@/components/list-empty-state";
import { TableSkeleton } from "@/components/table-skeleton";
import { useDeleteConfirm } from "@/hooks/use-delete-confirm";
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
import { useT } from "@/lib/i18n";

export default function SharesPage() {
  const t = useT();
  const [shares, setShares] = useState<ShareItem[]>([]);
  const [persistedPage, setPersistedPage] = usePersistentState("dashboard.shares.page", 1);
  const [persistedPageSize, setPersistedPageSize] = usePersistentState("dashboard.shares.pageSize", 20);
  const [pageInfo, setPageInfo] = useState<PageInfo>({
    page: persistedPage,
    pageSize: persistedPageSize,
    total: 0,
  });
  const [loading, setLoading] = useState(true);
  const [copiedID, setCopiedID] = useState<string | null>(null);
  // Snapshot of "now" taken at fetch time so the expired-check during render
  // stays pure (react-hooks/purity forbids `Date.now()` inside render).
  const [refreshedAt, setRefreshedAt] = useState<number>(0);

  const fetchShares = useCallback(async (page: number, pageSize: number) => {
    setLoading(true);
    try {
      const rsp = await api.listShares(page, pageSize);
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
      showErrorToast(err, { title: t("common.error") });
    } finally {
      setLoading(false);
    }
  }, [setPersistedPage, setPersistedPageSize, t]);

  /* eslint-disable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps -- Data fetching requires setting state from async effects on mount */
  useEffect(() => {
    fetchShares(persistedPage, persistedPageSize);
  }, [fetchShares]);
  /* eslint-enable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps */

  const handleCopy = useCallback(async (share: ShareItem) => {
    const url = buildShareURL(share.shareId);
    try {
      await navigator.clipboard.writeText(url);
      setCopiedID(share.shareId);
      toast.success(t("common.copied_to_clipboard"));
      window.setTimeout(() => setCopiedID(null), 2000);
    } catch {
      toast.error(t("shares.copy_error"));
    }
  }, [t]);

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
    <div className="space-y-8">
      <PageHeader
        title={t("shares.title")}
        description={t("shares.subtitle")}
      />

      <Card>
        <CardHeader>
          <CardTitle className="font-display">{t("shares.share_links")}</CardTitle>
        </CardHeader>
        <CardContent>
          {loading ? (
            <TableSkeleton rows={4} />
          ) : shares.length === 0 ? (
            <ListEmptyState
              icon={<Share2 className="mb-3 size-10 text-muted-foreground/40" />}
              message={t("shares.no_shares")}
              hint={t("shares.create_hint")}
            />
          ) : (
            <>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t("shares.share_id")}</TableHead>
                    <TableHead>{t("shares.session_id")}</TableHead>
                    <TableHead>{t("common.created")}</TableHead>
                    <TableHead>{t("shares.expires_at")}</TableHead>
                    <TableHead>{t("shares.status")}</TableHead>
                    <TableHead className="text-right">{t("common.actions")}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {shares.map((share) => {
                    const expired =
                      refreshedAt > 0 &&
                      new Date(share.expiresAt).getTime() < refreshedAt;
                    return (
                      <TableRow
                        key={share.shareId}
                        className={expired ? "bg-muted/30 text-muted-foreground" : undefined}
                      >
                        <TableCell className="max-w-[220px] truncate font-mono text-xs">
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
                        </TableCell>
                        <TableCell>
                          <a
                            href={`/web/sessions/detail/?id=${share.sessionId}`}
                            className="inline-flex items-center gap-1 font-mono text-xs text-primary hover:underline"
                          >
                            #{share.sessionId}
                            <ExternalLink className="size-3" />
                          </a>
                        </TableCell>
                        <TableCell className="text-xs text-muted-foreground">
                          {new Date(share.createdAt).toLocaleString()}
                        </TableCell>
                        <TableCell
                          className={
                            expired
                              ? "text-xs font-medium text-rose-500"
                              : "text-xs text-muted-foreground"
                          }
                        >
                          {new Date(share.expiresAt).toLocaleString()}
                        </TableCell>
                        <TableCell>
                          <Badge variant={expired ? "destructive" : "secondary"}>
                            {expired ? t("shares.expired") : t("shares.active")}
                          </Badge>
                        </TableCell>
                        <TableCell className="text-right">
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
            </>
          )}
        </CardContent>
      </Card>

      <DeleteConfirmDialog
        {...deleteConfirm.dialogProps}
        title={t("shares.delete_confirm")}
        description={t("shares.delete_dialog_desc").replace("{id}", String(deleteConfirm.target?.sessionId ?? ""))}
        confirmLabel={t("shares.revoke")}
        loadingLabel={t("shares.revoking")}
      />
    </div>
  );
}
