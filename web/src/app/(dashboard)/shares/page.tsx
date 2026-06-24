"use client";

/**
 * Shares management page — lists every active share link the current user
 * owns, lets them copy a link, jump to the source session, or revoke a share.
 */

import { useCallback, useEffect, useState } from "react";
import { usePersistentState } from "@/hooks/use-persistent-state";
import {
  AlertTriangle,
  Check,
  Copy,
  ExternalLink,
  Share2,
  Trash2,
} from "lucide-react";
import { toast } from "sonner";

import { api, ApiError } from "@/lib/api-client";
import type { PageInfo, ShareItem } from "@/lib/types";
import { buildShareURL } from "@/components/share/share-dialog";
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
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { PaginationBar } from "@/components/pagination-bar";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { useT } from "@/lib/i18n";

interface DeleteTarget {
  shareId: string;
  sessionId: number;
}

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
  const [deleteTarget, setDeleteTarget] = useState<DeleteTarget | null>(null);
  const [deleting, setDeleting] = useState(false);
  // Snapshot of "now" taken at fetch time so the expired-check during render
  // stays pure (react-hooks/purity forbids `Date.now()` inside render).
  const [refreshedAt, setRefreshedAt] = useState<number>(0);

  const fetchShares = useCallback(async (page: number, pageSize: number) => {
    setLoading(true);
    try {
      const rsp = await api.listShares(page, pageSize);
      if (rsp.error) {
        toast.error(rsp.error.message || t("common.error"));
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
      const msg =
        err instanceof ApiError
          ? `${t("common.error")} (${err.status})`
          : err instanceof Error
            ? err.message
            : t("common.error");
      toast.error(msg);
    } finally {
      setLoading(false);
    }
  }, [setPersistedPage, setPersistedPageSize]);

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

  const handleDelete = useCallback(async () => {
    if (!deleteTarget) return;
    setDeleting(true);
    try {
      const rsp = await api.deleteShare(deleteTarget.shareId);
      if (rsp.error) {
        toast.error(rsp.error.message || t("shares.revoke_error"));
        return;
      }
      toast.success(t("shares.revoke_success"));
      fetchShares(pageInfo.page, pageInfo.pageSize);
    } catch (err) {
      const msg =
        err instanceof ApiError
          ? `${t("shares.revoke_error")} (${err.status})`
          : err instanceof Error
            ? err.message
            : t("shares.revoke_error");
      toast.error(msg);
    } finally {
      setDeleting(false);
      setDeleteTarget(null);
    }
  }, [deleteTarget, fetchShares, pageInfo.page, pageInfo.pageSize]);

  return (
    <div className="space-y-8">
      <div>
        <h1 className="font-display text-3xl font-semibold tracking-tight text-foreground">
          {t("shares.title")}
        </h1>
        <p className="mt-1.5 text-sm text-muted-foreground">
          {t("shares.subtitle")}
        </p>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="font-display">{t("shares.share_links")}</CardTitle>
        </CardHeader>
        <CardContent>
          {loading ? (
            <div className="space-y-3">
              {Array.from({ length: 4 }).map((_, i) => (
                <Skeleton key={i} className="h-12 w-full" />
              ))}
            </div>
          ) : shares.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-12 text-center">
              <Share2 className="mb-3 size-10 text-muted-foreground/40" />
              <p className="text-sm text-muted-foreground">
                {t("shares.no_shares")}
              </p>
              <p className="mt-1 text-xs text-muted-foreground/70">
                {t("shares.create_hint")}
              </p>
            </div>
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
                            <Button
                              variant="destructive"
                              size="xs"
                              onClick={() =>
                                setDeleteTarget({
                                  shareId: share.shareId,
                                  sessionId: share.sessionId,
                                })
                              }
                              className="gap-1"
                            >
                              <Trash2 className="size-3" />
                              {t("shares.revoke")}
                            </Button>
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

      <AlertDialog
        open={deleteTarget !== null}
        onOpenChange={(open) => {
          if (!open) setDeleteTarget(null);
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle className="flex items-center gap-2">
              <AlertTriangle className="size-5 text-destructive" />
              {t("shares.delete_confirm")}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t("shares.delete_dialog_desc").replace("{id}", String(deleteTarget?.sessionId ?? ""))}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
              <AlertDialogCancel>{t("common.cancel")}</AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              onClick={handleDelete}
              disabled={deleting}
            >
              {deleting ? t("shares.revoking") : t("shares.revoke")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
