"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api-client";
import { showErrorToast } from "@/lib/api-error-handler";
import { PermissionGuard } from "@/components/permission-guard";
import type { BlockedAction, BlockedItem, PageInfo } from "@/lib/types";
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
import { Label } from "@/components/ui/label";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { DeleteButton } from "@/components/delete-button";
import { DeleteConfirmDialog } from "@/components/delete-confirm-dialog";
import { PageHeader } from "@/components/page-header";
import { SearchInput } from "@/components/search-input";
import { ListEmptyState } from "@/components/list-empty-state";
import { TableSkeleton } from "@/components/table-skeleton";
import { Ban, Plus, RefreshCw } from "lucide-react";
import { PaginationBar } from "@/components/pagination-bar";
import { toast } from "sonner";
import { usePersistentState } from "@/hooks/use-persistent-state";
import { useDeleteConfirm } from "@/hooks/use-delete-confirm";
import { useIsMobile } from "@/hooks/use-mobile";
import { useT } from "@/lib/i18n";
import { cn } from "@/lib/utils";

const emptyForm = { word: "", action: "deny" as BlockedAction };

const actionOptions: { value: BlockedAction; labelKey: string }[] = [
  { value: "deny", labelKey: "blocked.action_deny" },
  { value: "allow", labelKey: "blocked.action_allow" },
];

function ActionBadge({ action, t }: { action: BlockedAction; t: (key: string) => string }) {
  return (
    <span
      className={cn(
        "inline-flex items-center rounded-full px-2 py-0.5 text-[11px] font-medium",
        action === "deny"
          ? "bg-destructive/10 text-destructive"
          : "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400",
      )}
    >
      {action === "deny" ? t("blocked.action_deny") : t("blocked.action_allow")}
    </span>
  );
}

export default function BlockPage() {
  const [items, setItems] = useState<BlockedItem[]>([]);
  const [persistedPage, setPersistedPage] = usePersistentState("dashboard.blocked.page", 1);
  const [persistedPageSize, setPersistedPageSize] = usePersistentState(
    "dashboard.blocked.pageSize",
    20,
  );
  const [pageInfo, setPageInfo] = useState<PageInfo>({
    page: persistedPage,
    pageSize: persistedPageSize,
    total: 0,
  });
  const [loading, setLoading] = useState(true);
  const [searchQuery, setSearchQuery] = useState("");
  const t = useT();
  const [dialogOpen, setDialogOpen] = useState(false);
  const [form, setForm] = useState(emptyForm);
  const [saving, setSaving] = useState(false);
  const isMobile = useIsMobile();

  const fetchItems = useCallback(
    async (page: number, pageSize: number, query?: string) => {
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
      } catch (err) {
        showErrorToast(err, { title: t("blocked.load_error") });
      } finally {
        setLoading(false);
      }
    },
    [setPersistedPage, setPersistedPageSize, t],
  );

  /* eslint-disable react-hooks/set-state-in-effect -- Re-fetch list when the persisted page or size changes */
  useEffect(() => {
    fetchItems(persistedPage, persistedPageSize);
  }, [fetchItems, persistedPage, persistedPageSize]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const handleSearch = useCallback(() => {
    setPersistedPage(1);
    fetchItems(1, persistedPageSize, searchQuery || undefined);
  }, [fetchItems, persistedPageSize, searchQuery, setPersistedPage]);

  const handleCreate = useCallback(async () => {
    if (!form.word.trim()) return;
    setSaving(true);
    try {
      await api.createBlocked({ word: form.word.trim(), action: form.action });
      toast.success(t("blocked.created_success"));
      setDialogOpen(false);
      setForm(emptyForm);
      fetchItems(persistedPage, persistedPageSize);
    } catch (err) {
      showErrorToast(err, { title: t("blocked.create_error") });
    } finally {
      setSaving(false);
    }
  }, [form.word, form.action, fetchItems, persistedPage, persistedPageSize, t]);

  const handleToggleAction = useCallback(async (item: BlockedItem) => {
    const next: BlockedAction = item.action === "deny" ? "allow" : "deny";
    try {
      await api.updateBlocked(item.id, { action: next });
      toast.success(t("blocked.action_updated"));
      fetchItems(persistedPage, persistedPageSize);
    } catch (err) {
      showErrorToast(err, { title: t("blocked.action_update_error") });
    }
  }, [fetchItems, persistedPage, persistedPageSize, t]);

  const deleteConfirm = useDeleteConfirm<BlockedItem>({
    onConfirm: async (item) => {
      await api.deleteBlocked(item.id);
      toast.success(t("blocked.deleted_success"));
      fetchItems(persistedPage, persistedPageSize);
    },
    onError: (err) => showErrorToast(err, { title: t("blocked.delete_error") }),
    closeOnError: false,
  });

  return (
    <PermissionGuard adminOnly>
      <div className="space-y-8">
        <PageHeader
          title={t("blocked.title")}
          description={t("blocked.subtitle")}
          actions={
            <Button
              onClick={() => {
                setForm(emptyForm);
                setDialogOpen(true);
              }}
            >
              <Plus /> {t("blocked.create")}
            </Button>
          }
        />

        <Card>
          <CardHeader>
            <CardTitle className="font-display">{t("blocked.all_words")}</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="mb-4">
              <SearchInput
                placeholder={t("blocked.search_placeholder")}
                value={searchQuery}
                onChange={setSearchQuery}
                onSearch={handleSearch}
              />
            </div>
            {loading ? (
              <TableSkeleton />
            ) : items.length === 0 ? (
              <ListEmptyState
                icon={<Ban className="mb-3 size-10 text-muted-foreground/40" />}
                message={t("blocked.no_words")}
              />
            ) : (
              <>
                {isMobile ? (
                  <div className="space-y-3">
                    {items.map((item) => (
                      <div key={item.id} className="rounded-lg border border-border bg-card p-4">
                        <div className="flex items-start justify-between gap-3">
                          <div className="min-w-0 flex-1">
                            <p className="text-sm font-medium">{item.word}</p>
                            <p className="mt-0.5 text-xs text-muted-foreground">
                              {t("blocked.hit_count")}: {item.hitCount}
                            </p>
                            <div className="mt-1.5"><ActionBadge action={item.action} t={t} /></div>
                          </div>
                          <div className="flex shrink-0 items-center gap-1">
                            <Button
                              variant="outline"
                              size="sm"
                              title={t("blocked.action_switch")}
                              onClick={() => handleToggleAction(item)}
                            >
                              <RefreshCw className="size-3.5" />
                            </Button>
                            <DeleteButton
                              label={t("common.delete")}
                              onClick={() => deleteConfirm.openDelete(item)}
                            />
                          </div>
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
                        <TableHead className="w-28">{t("blocked.action")}</TableHead>
                        <TableHead className="w-24">{t("blocked.hit_count")}</TableHead>
                        <TableHead className="w-32">{t("common.created")}</TableHead>
                        <TableHead className="w-28">{t("common.actions")}</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {items.map((item) => (
                        <TableRow key={item.id}>
                          <TableCell className="text-muted-foreground">{item.id}</TableCell>
                          <TableCell className="font-medium">{item.word}</TableCell>
                          <TableCell><ActionBadge action={item.action} t={t} /></TableCell>
                          <TableCell>{item.hitCount}</TableCell>
                          <TableCell className="text-muted-foreground">
                            {new Date(item.createdAt).toLocaleDateString()}
                          </TableCell>
                          <TableCell>
                            <div className="flex items-center gap-1">
                              <Button
                                variant="outline"
                                size="sm"
                                title={t("blocked.action_switch")}
                                onClick={() => handleToggleAction(item)}
                              >
                                <RefreshCw className="size-3.5" />
                              </Button>
                              <DeleteButton
                                label={t("common.delete")}
                                onClick={() => deleteConfirm.openDelete(item)}
                              />
                            </div>
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                )}
                <PaginationBar
                  pageInfo={pageInfo}
                  onChange={(page, pageSize) =>
                    fetchItems(page, pageSize, searchQuery || undefined)
                  }
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
            <div className="space-y-4">
              <Input
                placeholder={t("blocked.create_placeholder")}
                value={form.word}
                onChange={(e) => setForm({ ...form, word: e.target.value })}
                onKeyDown={(e) => {
                  if (e.key === "Enter") handleCreate();
                }}
              />
              <div className="space-y-2">
                <Label>{t("blocked.action")}</Label>
                <RadioGroup
                  value={form.action}
                  onValueChange={(v) => setForm({ ...form, action: v as BlockedAction })}
                  className="flex flex-col gap-2"
                >
                  {actionOptions.map((opt) => (
                    <div key={opt.value} className="flex items-center gap-2">
                      <RadioGroupItem value={opt.value} id={`blocked-action-${opt.value}`} />
                      <Label htmlFor={`blocked-action-${opt.value}`} className="cursor-pointer text-sm font-normal">
                        {t(opt.labelKey)}
                      </Label>
                    </div>
                  ))}
                </RadioGroup>
              </div>
            </div>
            <DialogFooter>
              <Button variant="outline" onClick={() => setDialogOpen(false)}>
                {t("common.cancel")}
              </Button>
              <Button onClick={handleCreate} disabled={!form.word.trim() || saving}>
                {saving ? t("common.saving") : t("common.create")}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        <DeleteConfirmDialog
          {...deleteConfirm.dialogProps}
          title={t("common.are_you_sure")}
          description={t("blocked.delete_confirm")}
          confirmLabel={t("common.delete")}
        />
      </div>
    </PermissionGuard>
  );
}
