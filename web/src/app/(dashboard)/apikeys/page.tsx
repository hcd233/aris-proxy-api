"use client";

import { useCallback, useEffect, useState } from "react";
import { usePersistentState } from "@/hooks/use-persistent-state";
import { api } from "@/lib/api-client";
import type { APIKeyItem, APIKeyDetail, PageInfo } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Key, Plus, Trash2, Copy, Check, AlertTriangle, Search } from "lucide-react";
import { PaginationBar } from "@/components/pagination-bar";
import { toast } from "sonner";
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
import { useIsMobile } from "@/hooks/use-mobile";
import { useT } from "@/lib/i18n";

export default function APIKeysPage() {
  const t = useT();
  const isMobile = useIsMobile();
  const [keys, setKeys] = useState<APIKeyItem[]>([]);
  const [persistedPage, setPersistedPage] = usePersistentState("dashboard.apikeys.page", 1);
  const [persistedPageSize, setPersistedPageSize] = usePersistentState("dashboard.apikeys.pageSize", 20);
  const [pageInfo, setPageInfo] = useState<PageInfo>({ page: persistedPage, pageSize: persistedPageSize, total: 0 });
  const [loading, setLoading] = useState(true);
  const [searchQuery, setSearchQuery] = useState("");
  const [createOpen, setCreateOpen] = useState(false);
  const [newKeyName, setNewKeyName] = useState("");
  const [creating, setCreating] = useState(false);
  const [createdKey, setCreatedKey] = useState<APIKeyDetail | null>(null);
  const [copied, setCopied] = useState(false);
  const [deleting, setDeleting] = useState<number | null>(null);
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<{ id: number; name: string } | null>(null);

  const fetchKeys = useCallback(async (page: number, pageSize: number, query?: string) => {
    setLoading(true);
    try {
      const rsp = await api.listAPIKeys(page, pageSize, query);
      setKeys(rsp.keys ?? []);
      if (rsp.pageInfo) {
        setPageInfo(rsp.pageInfo);
        setPersistedPage(rsp.pageInfo.page);
        setPersistedPageSize(rsp.pageInfo.pageSize);
      }
    } catch {
      toast.error(t("apikeys.load_error"));
    } finally {
      setLoading(false);
    }
  }, [t, setPersistedPage, setPersistedPageSize]);

  /* eslint-disable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps -- Data fetching requires setting state from async effects on mount */
  useEffect(() => {
    fetchKeys(persistedPage, persistedPageSize);
  }, [fetchKeys]);
  /* eslint-enable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps */

  const handleCreate = async () => {
    if (!newKeyName.trim()) return;
    setCreating(true);
    try {
      const rsp = await api.createAPIKey({ name: newKeyName.trim() });
      if (rsp.error) {
        toast.error(rsp.error.message);
        return;
      }
      if (rsp.key) {
        setCreatedKey(rsp.key);
        setNewKeyName("");
        toast.success(t("apikeys.created_success"));
        fetchKeys(pageInfo.page, pageInfo.pageSize, searchQuery || undefined);
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("apikeys.create_error"));
    } finally {
      setCreating(false);
    }
  };

  const openDeleteConfirm = (key: APIKeyItem) => {
    setDeleteTarget({ id: key.id, name: key.name });
    setDeleteConfirmOpen(true);
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;
    setDeleting(deleteTarget.id);
    try {
      await api.deleteAPIKey(deleteTarget.id);
      toast.success(t("apikeys.deleted_success"));
      fetchKeys(pageInfo.page, pageInfo.pageSize, searchQuery || undefined);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("apikeys.delete_error"));
    } finally {
      setDeleting(null);
      setDeleteConfirmOpen(false);
      setDeleteTarget(null);
    }
  };

  const handleCopy = (key: string) => {
    navigator.clipboard.writeText(key);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
    toast.success(t("common.copied_to_clipboard"));
  };

  const closeCreateDialog = () => {
    setCreateOpen(false);
    setCreatedKey(null);
    setNewKeyName("");
  };

  return (
    <div className="space-y-8">
      <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
        <div>
          <h1 className="font-display text-2xl md:text-3xl font-semibold tracking-tight text-foreground">{t("apikeys.title")}</h1>
          <p className="mt-1.5 text-sm text-muted-foreground">
            {t("apikeys.subtitle")}
          </p>
        </div>
        <Dialog
          open={createOpen}
          onOpenChange={(open) => {
            if (!open) closeCreateDialog();
            else setCreateOpen(true);
          }}
        >
          <DialogTrigger
            render={<Button />}
          >
            <Plus className="mr-1 size-4" />
            {t("apikeys.create_key")}
          </DialogTrigger>
          <DialogContent>
            {createdKey ? (
              <>
                <DialogHeader>
                  <DialogTitle>{t("apikeys.key_created")}</DialogTitle>
                  <DialogDescription>
                    {t("apikeys.copy_key_warning")}
                  </DialogDescription>
                </DialogHeader>
                <div className="flex items-center gap-2 rounded-lg bg-muted p-3">
                  <code className="flex-1 break-all text-sm">{createdKey.key}</code>
                  <Button
                    variant="ghost"
                    size="icon-sm"
                    onClick={() => handleCopy(createdKey.key)}
                  >
                    {copied ? <Check className="size-4" /> : <Copy className="size-4" />}
                  </Button>
                </div>
                <DialogFooter showCloseButton>
                  <Button onClick={closeCreateDialog}>{t("common.done")}</Button>
                </DialogFooter>
              </>
            ) : (
              <>
                <DialogHeader>
                  <DialogTitle>{t("apikeys.create")}</DialogTitle>
                  <DialogDescription>
                    {t("apikeys.create_description")}
                  </DialogDescription>
                </DialogHeader>
                <div className="space-y-2">
                  <Label htmlFor="key-name">{t("apikeys.create_name_label")}</Label>
                  <Input
                    id="key-name"
                    placeholder={t("apikeys.create_name_placeholder")}
                    value={newKeyName}
                    onChange={(e) => setNewKeyName(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === "Enter") handleCreate();
                    }}
                  />
                </div>
                <DialogFooter>
                  <Button variant="outline" onClick={closeCreateDialog}>
                    {t("common.cancel")}
                  </Button>
                  <Button onClick={handleCreate} disabled={!newKeyName.trim() || creating}>
                    {creating ? t("common.creating") : t("common.create")}
                  </Button>
                </DialogFooter>
              </>
            )}
          </DialogContent>
        </Dialog>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="font-display">{t("apikeys.your_keys")}</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="mb-4">
            <div className="relative w-full md:max-w-sm">
              <Search className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
              <Input
                placeholder={t("apikeys.search_keys")}
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") fetchKeys(1, pageInfo.pageSize, searchQuery || undefined);
                }}
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
          ) : keys.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-12 text-center">
              <Key className="mb-3 size-10 text-muted-foreground/40" />
              <p className="text-sm text-muted-foreground">
                {t("apikeys.empty")}
              </p>
            </div>
          ) : (
            <>
              {isMobile ? (
                <div className="space-y-3">
                  {keys.map((key) => (
                    <div
                      key={key.id}
                      className="rounded-lg border border-border bg-card p-4"
                    >
                      <div className="flex items-start justify-between gap-3">
                        <div className="min-w-0 flex-1">
                          <p className="truncate text-sm font-medium">{key.name}</p>
                          <p className="mt-0.5 truncate font-mono text-xs text-muted-foreground">
                            {key.key}
                          </p>
                        </div>
                        <Button
                          variant="destructive"
                          size="xs"
                          disabled={deleting === key.id}
                          onClick={() => openDeleteConfirm(key)}
                        >
                          <Trash2 className="mr-1 size-3" />
                          {t("common.delete")}
                        </Button>
                      </div>
                      <p className="mt-2 text-xs text-muted-foreground">
                        {t("apikeys.created")} {new Date(key.createdAt).toLocaleDateString()}
                      </p>
                    </div>
                  ))}
                </div>
              ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t("apikeys.name")}</TableHead>
                    <TableHead>{t("apikeys.key")}</TableHead>
                    <TableHead>{t("apikeys.created")}</TableHead>
                    <TableHead className="text-right">{t("common.actions")}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {keys.map((key) => (
                    <TableRow key={key.id}>
                      <TableCell className="font-medium">{key.name}</TableCell>
                      <TableCell className="font-mono text-xs text-muted-foreground">
                        {key.key}
                      </TableCell>
                      <TableCell className="text-muted-foreground">
                        {new Date(key.createdAt).toLocaleDateString()}
                      </TableCell>
                      <TableCell className="text-right">
                        <Button
                          variant="destructive"
                          size="xs"
                          disabled={deleting === key.id}
                          onClick={() => openDeleteConfirm(key)}
                        >
                          <Trash2 className="mr-1 size-3" />
                          {t("common.delete")}
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
              )}

              <PaginationBar
                pageInfo={pageInfo}
                onChange={(page, pageSize) => fetchKeys(page, pageSize, searchQuery || undefined)}
                totalLabel="keys"
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
              {t("common.are_you_sure")}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t("apikeys.delete_description").replace("{name}", deleteTarget?.name ?? "")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
              <AlertDialogCancel>{t("common.cancel")}</AlertDialogCancel>
            <AlertDialogAction variant="destructive" onClick={handleDelete} disabled={deleting !== null}>
              {deleting !== null ? t("common.deleting") : t("common.delete")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}