"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api-client";
import { PermissionGuard } from "@/components/permission-guard";
import type { BlockedItem, PageInfo } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
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
import { Ban, Plus, Search, Trash2, AlertTriangle } from "lucide-react";
import { PaginationBar } from "@/components/pagination-bar";
import { toast } from "sonner";
import { usePersistentState } from "@/hooks/use-persistent-state";
import { useIsMobile } from "@/hooks/use-mobile";
import { useT } from "@/lib/i18n";

const emptyForm = { word: "" };

export default function BlockPage() {
  const [items, setItems] = useState<BlockedItem[]>([]);
  const [persistedPage, setPersistedPage] = usePersistentState("dashboard.blocked.page", 1);
  const [persistedPageSize, setPersistedPageSize] = usePersistentState("dashboard.blocked.pageSize", 20);
  const [pageInfo, setPageInfo] = useState<PageInfo>({ page: persistedPage, pageSize: persistedPageSize, total: 0 });
  const [loading, setLoading] = useState(true);
  const [searchQuery, setSearchQuery] = useState("");
  const t = useT();
  const [dialogOpen, setDialogOpen] = useState(false);
  const [form, setForm] = useState(emptyForm);
  const [saving, setSaving] = useState(false);
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<BlockedItem | null>(null);
  const isMobile = useIsMobile();

  const fetchItems = useCallback(async (page: number, pageSize: number, query?: string) => {
    setLoading(true);
    try {
      const safeSize = pageSize > 0 ? pageSize : 20;
      const rsp = await api.listBlocked(page, safeSize, query);
      setItems(rsp.blocked ?? []);
      if (rsp.pageInfo) {
        setPageInfo(rsp.pageInfo);
        setPersistedPage(rsp.pageInfo.page);
        setPersistedPageSize(rsp.pageInfo.pageSize);
      }
    } catch {
      toast.error(t("blocked.load_error"));
    } finally {
      setLoading(false);
    }
  }, [setPersistedPage, setPersistedPageSize]);

  /* eslint-disable react-hooks/set-state-in-effect -- Re-fetch list when the persisted page or size changes */
  useEffect(() => { fetchItems(persistedPage, persistedPageSize); }, [fetchItems, persistedPage, persistedPageSize]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const handleSearch = useCallback(() => {
    setPersistedPage(1);
    fetchItems(1, persistedPageSize, searchQuery || undefined);
  }, [fetchItems, persistedPageSize, searchQuery, setPersistedPage]);

  const handleCreate = useCallback(async () => {
    if (!form.word.trim()) return;
    setSaving(true);
    try {
      await api.createBlocked({ word: form.word.trim() });
      toast.success(t("blocked.created_success"));
      setDialogOpen(false);
      setForm(emptyForm);
      fetchItems(persistedPage, persistedPageSize);
    } catch {
      toast.error(t("blocked.create_error"));
    } finally {
      setSaving(false);
    }
  }, [form.word, fetchItems, persistedPage, persistedPageSize]);

  const handleDelete = useCallback(async () => {
    if (!deleteTarget) return;
    try {
      await api.deleteBlocked(deleteTarget.id);
      toast.success(t("blocked.deleted_success"));
      setDeleteConfirmOpen(false);
      setDeleteTarget(null);
      fetchItems(persistedPage, persistedPageSize);
    } catch {
      toast.error(t("blocked.delete_error"));
    }
  }, [deleteTarget, fetchItems, persistedPage, persistedPageSize]);

  return (
    <PermissionGuard adminOnly>
      <div className="space-y-8">
        <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
          <div>
            <h1 className="font-display text-2xl md:text-3xl font-semibold tracking-tight text-foreground">{t("blocked.title")}</h1>
            <p className="mt-1.5 text-sm text-muted-foreground">{t("blocked.subtitle")}</p>
          </div>
          <Button onClick={() => { setForm(emptyForm); setDialogOpen(true); }}>
            <Plus /> {t("blocked.create")}
          </Button>
        </div>

        <Card>
          <CardHeader>
            <CardTitle className="font-display">{t("blocked.all_words")}</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="mb-4">
              <div className="relative w-full md:max-w-sm">
                <Search className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  placeholder={t("blocked.search_placeholder")}
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  onKeyDown={(e) => { if (e.key === "Enter") handleSearch(); }}
                  className="pl-9"
                />
              </div>
            </div>
            {loading ? (
              <div className="space-y-3">
                {Array.from({ length: 3 }).map((_, i) => (
                  <Skeleton key={i} className="h-12 w-full" />
                ))}
              </div>
            ) : items.length === 0 ? (
              <div className="flex flex-col items-center justify-center py-12 text-center">
                <Ban className="mb-3 size-10 text-muted-foreground/40" />
                <p className="text-sm text-muted-foreground">{t("blocked.no_words")}</p>
              </div>
            ) : (
              <>
                {isMobile ? (
                  <div className="space-y-3">
                    {items.map((item) => (
                      <div key={item.id} className="rounded-lg border border-border bg-card p-4">
                        <div className="flex items-start justify-between gap-3">
                          <div className="min-w-0 flex-1">
                            <p className="text-sm font-medium">{item.word}</p>
                            <p className="mt-0.5 text-xs text-muted-foreground">{t("blocked.hit_count")}: {item.hitCount}</p>
                          </div>
                          <Button variant="destructive" size="sm"
                            onClick={() => { setDeleteTarget(item); setDeleteConfirmOpen(true); }}>
                            <Trash2 />
                          </Button>
                        </div>
                      </div>
                    ))}
                  </div>
                ) : (
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead className="w-16">{t("blocked.id")}</TableHead>
                        <TableHead>{t("blocked.word")}</TableHead>
                        <TableHead className="w-24">{t("blocked.hit_count")}</TableHead>
                        <TableHead className="w-32">{t("common.created")}</TableHead>
                        <TableHead className="w-20">{t("common.actions")}</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {items.map((item) => (
                        <TableRow key={item.id}>
                          <TableCell className="text-muted-foreground">{item.id}</TableCell>
                          <TableCell className="font-medium">{item.word}</TableCell>
                          <TableCell>{item.hitCount}</TableCell>
                          <TableCell className="text-muted-foreground">{new Date(item.createdAt).toLocaleDateString()}</TableCell>
                          <TableCell>
                            <Button variant="destructive" size="sm"
                              onClick={() => { setDeleteTarget(item); setDeleteConfirmOpen(true); }}>
                              <Trash2 />
                            </Button>
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                )}
                <PaginationBar
                  pageInfo={pageInfo}
                  onChange={(page, pageSize) => fetchItems(page, pageSize, searchQuery || undefined)}
                  totalLabel={t("pagination.items")}
                />
              </>
            )}
          </CardContent>
        </Card>

        <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
          <DialogContent className="sm:max-w-md">
            <DialogHeader>
              <DialogTitle>{t("blocked.create")}</DialogTitle>
              <DialogDescription>{t("blocked.create_placeholder")}</DialogDescription>
            </DialogHeader>
            <div className="space-y-3">
              <Input
                placeholder={t("blocked.create_placeholder")}
                value={form.word}
                onChange={(e) => setForm({ word: e.target.value })}
                onKeyDown={(e) => { if (e.key === "Enter") handleCreate(); }}
              />
            </div>
            <DialogFooter>
              <Button variant="outline" onClick={() => setDialogOpen(false)}>{t("common.cancel")}</Button>
              <Button onClick={handleCreate} disabled={!form.word.trim() || saving}>
                {saving ? t("common.saving") : t("common.create")}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        <AlertDialog open={deleteConfirmOpen} onOpenChange={setDeleteConfirmOpen}>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle className="flex items-center gap-2">
                <AlertTriangle className="size-5 text-destructive" /> {t("common.are_you_sure")}
              </AlertDialogTitle>
              <AlertDialogDescription>
                {t("blocked.delete_confirm")}
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>{t("common.cancel")}</AlertDialogCancel>
              <AlertDialogAction variant="destructive" onClick={handleDelete}>{t("common.delete")}</AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </div>
    </PermissionGuard>
  );
}
