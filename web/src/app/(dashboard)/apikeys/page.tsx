"use client";

import { useCallback, useEffect, useState } from "react";
import { usePersistentState } from "@/hooks/use-persistent-state";
import { api } from "@/lib/api-client";
import { showErrorToast } from "@/lib/api-error-handler";
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
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Key, Plus, Copy, Check } from "lucide-react";
import { PaginationBar } from "@/components/pagination-bar";
import { toast } from "sonner";
import { DeleteButton } from "@/components/delete-button";
import { DeleteConfirmDialog } from "@/components/delete-confirm-dialog";
import { PageHeader } from "@/components/page-header";
import { SearchInput } from "@/components/search-input";
import { ListEmptyState } from "@/components/list-empty-state";
import { TableSkeleton } from "@/components/table-skeleton";
import { useDeleteConfirm } from "@/hooks/use-delete-confirm";
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
    } catch (err) {
      showErrorToast(err, { title: t("apikeys.load_error") });
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
        showErrorToast(rsp.error, { title: t("apikeys.create_error") });
        return;
      }
      if (rsp.key) {
        setCreatedKey(rsp.key);
        setNewKeyName("");
        toast.success(t("apikeys.created_success"));
        fetchKeys(pageInfo.page, pageInfo.pageSize, searchQuery || undefined);
      }
    } catch (err) {
      showErrorToast(err, { title: t("apikeys.create_error") });
    } finally {
      setCreating(false);
    }
  };

  const deleteConfirm = useDeleteConfirm<APIKeyItem>({
    onConfirm: async (key) => {
      await api.deleteAPIKey(key.id);
      toast.success(t("apikeys.deleted_success"));
      fetchKeys(pageInfo.page, pageInfo.pageSize, searchQuery || undefined);
    },
    onError: (err) => showErrorToast(err, { title: t("apikeys.delete_error") }),
  });

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
      <PageHeader
        title={t("apikeys.title")}
        description={t("apikeys.subtitle")}
        actions={
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
        }
      />

      <Card>
        <CardHeader>
          <CardTitle className="font-display">{t("apikeys.your_keys")}</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="mb-4">
            <SearchInput
              placeholder={t("apikeys.search_keys")}
              value={searchQuery}
              onChange={setSearchQuery}
              onSearch={() => fetchKeys(1, pageInfo.pageSize, searchQuery || undefined)}
            />
          </div>
          {loading ? (
            <TableSkeleton />
          ) : keys.length === 0 ? (
            <ListEmptyState icon={<Key className="mb-3 size-10 text-muted-foreground/40" />} message={t("apikeys.empty")} />
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
                        <DeleteButton
                          label={t("common.delete")}
                          disabled={deleteConfirm.loading && deleteConfirm.target?.id === key.id}
                          onClick={() => deleteConfirm.openDelete(key)}
                        />
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
                        <DeleteButton
                          label={t("common.delete")}
                          disabled={deleteConfirm.loading && deleteConfirm.target?.id === key.id}
                          onClick={() => deleteConfirm.openDelete(key)}
                        />
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
              )}

              <PaginationBar
                pageInfo={pageInfo}
                onChange={(page, pageSize) => fetchKeys(page, pageSize, searchQuery || undefined)}
                totalLabel={t("pagination.keys")}
              />
            </>
          )}
        </CardContent>
      </Card>

      <DeleteConfirmDialog
        {...deleteConfirm.dialogProps}
        title={t("common.are_you_sure")}
        description={t("apikeys.delete_description").replace("{name}", deleteConfirm.target?.name ?? "")}
        confirmLabel={t("common.delete")}
        loadingLabel={t("common.deleting")}
      />
    </div>
  );
}