"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api-client";
import { showErrorToast } from "@/lib/api-error-handler";
import { PermissionGuard } from "@/components/permission-guard";
import type { BlockedAction, BlockedItem, PageInfo } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
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
import { Ban, Check, Trash2 } from "lucide-react";
import { PaginationBar } from "@/components/pagination-bar";
import { toast } from "sonner";
import { usePersistentState } from "@/hooks/use-persistent-state";
import { useOptimisticUpdate } from "@/hooks/use-optimistic-update";
import { useDeleteConfirm } from "@/hooks/use-delete-confirm";
import { useIsMobile } from "@/hooks/use-mobile";
import { useT } from "@/lib/i18n";
import { cn } from "@/lib/utils";

/** 动作徽章：点击直接切换 deny⇄omit */
function ActionBadge({
  action,
  t,
  onClick,
  disabled,
}: {
  action: BlockedAction;
  t: (key: string) => string;
  onClick: () => void;
  disabled?: boolean;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      title={t("blocked.action_switch_hint")}
      className={cn(
        "inline-flex cursor-pointer items-center rounded-full px-2 py-0.5 text-[11px] font-medium transition-colors",
        "hover:ring-2 hover:ring-current/20 disabled:cursor-not-allowed disabled:opacity-60",
        action === "deny"
          ? "bg-destructive/10 text-destructive hover:bg-destructive/20"
          : "bg-emerald-500/10 text-emerald-600 hover:bg-emerald-500/20 dark:text-emerald-400",
      )}
    >
      {action === "deny" ? t("blocked.action_deny") : t("blocked.action_omit")}
    </button>
  );
}

/** 勾选框（对齐 sessions 的 role="checkbox" 自绘模式） */
function SelectCheckbox({ checked, onToggle }: { checked: boolean; onToggle: () => void }) {
  return (
    <div
      role="checkbox"
      aria-checked={checked}
      tabIndex={0}
      onClick={onToggle}
      onKeyDown={(e) => {
        if (e.key === " " || e.key === "Enter") onToggle();
      }}
      className={cn(
        "flex size-4 cursor-pointer items-center justify-center rounded border transition-colors",
        checked
          ? "border-primary bg-primary text-primary-foreground"
          : "border-muted-foreground/30 hover:border-muted-foreground",
      )}
    >
      {checked && <Check className="size-3" />}
    </div>
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
  const [inlineWord, setInlineWord] = useState("");
  const [adding, setAdding] = useState(false);
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [batchDeleting, setBatchDeleting] = useState(false);
  const [batchDeleteConfirmOpen, setBatchDeleteConfirmOpen] = useState(false);
  const t = useT();
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
        setSelected(new Set()); // 翻页/刷新清空选中（对齐 sessions）
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

  const handleInlineAdd = useCallback(async () => {
    const word = inlineWord.trim();
    if (!word || adding) return;
    setAdding(true);
    try {
      await api.createBlocked({ word, action: "deny" });
      toast.success(t("blocked.created_success"));
      setInlineWord("");
      fetchItems(persistedPage, persistedPageSize);
    } catch (err) {
      showErrorToast(err, { title: t("blocked.create_error") });
    } finally {
      setAdding(false);
    }
  }, [inlineWord, adding, fetchItems, persistedPage, persistedPageSize, t]);

  // action 徽章切换：乐观更新 + 失败回滚，避免整表重拉导致闪烁
  const toggleAction = useOptimisticUpdate<BlockedItem>({
    setItems,
    getKey: (item) => item.id,
    update: async (item) => {
      await api.updateBlocked(item.id, { action: item.action });
    },
    onSuccess: () => toast.success(t("blocked.action_updated")),
    onError: (err) => showErrorToast(err, { title: t("blocked.action_update_error") }),
  });

  const deleteConfirm = useDeleteConfirm<BlockedItem>({
    onConfirm: async (item) => {
      await api.deleteBlocked(item.id);
      toast.success(t("blocked.deleted_success"));
      fetchItems(persistedPage, persistedPageSize);
    },
    onError: (err) => showErrorToast(err, { title: t("blocked.delete_error") }),
    closeOnError: false,
  });

  const toggleSelect = useCallback((id: number) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  }, []);

  const toggleSelectAll = useCallback(() => {
    if (selected.size === items.length) {
      setSelected(new Set());
    } else {
      setSelected(new Set(items.map((i) => i.id)));
    }
  }, [selected.size, items]);

  const handleBatchDelete = useCallback(async () => {
    if (selected.size === 0) return;
    setBatchDeleting(true);
    try {
      const ids = Array.from(selected);
      const rsp = await api.batchDeleteBlocked(ids);
      toast.success(
        t("blocked.batch_delete_success").replace(
          "{count}",
          String(rsp.deletedCount ?? ids.length),
        ),
      );
      fetchItems(persistedPage, persistedPageSize);
    } catch (err) {
      showErrorToast(err, { title: t("blocked.batch_delete_error") });
    } finally {
      setBatchDeleting(false);
      setBatchDeleteConfirmOpen(false);
    }
  }, [selected, fetchItems, persistedPage, persistedPageSize, t]);

  return (
    <PermissionGuard adminOnly>
      <div className="space-y-8">
        <PageHeader title={t("blocked.title")} description={t("blocked.subtitle")} />

        <Card>
          <CardHeader>
            <CardTitle className="font-display">{t("blocked.all_words")}</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="mb-4 flex flex-col gap-2 sm:flex-row sm:items-center">
              <SearchInput
                className="sm:max-w-xs"
                placeholder={t("blocked.search_placeholder")}
                value={searchQuery}
                onChange={setSearchQuery}
                onSearch={handleSearch}
              />
              <Input
                className="sm:max-w-xs"
                placeholder={t("blocked.inline_add_placeholder")}
                value={inlineWord}
                onChange={(e) => setInlineWord(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") handleInlineAdd();
                }}
                disabled={adding}
              />
              {selected.size > 0 && (
                <Button
                  variant="destructive"
                  size="sm"
                  onClick={() => setBatchDeleteConfirmOpen(true)}
                  className="gap-1.5 sm:ml-auto"
                >
                  <Trash2 className="size-3.5" />
                  {t("common.delete")} {selected.size}
                </Button>
              )}
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
                    {items.map((item) => {
                      const isSelected = selected.has(item.id);
                      return (
                        <div key={item.id} className="rounded-lg border border-border bg-card p-4">
                          <div className="flex items-start justify-between gap-3">
                            <div className="flex min-w-0 flex-1 items-start gap-2">
                              <SelectCheckbox
                                checked={isSelected}
                                onToggle={() => toggleSelect(item.id)}
                              />
                              <div className="min-w-0 flex-1">
                                <p className="text-sm font-medium">{item.word}</p>
                                <p className="mt-0.5 text-xs text-muted-foreground">
                                  {t("blocked.hit_count")}: {item.hitCount}
                                </p>
                                <div className="mt-1.5">
                                  <ActionBadge
                                    action={item.action}
                                    t={t}
                                    disabled={toggleAction.updatingKey !== null}
                                    onClick={() =>
                                      toggleAction.apply(item, {
                                        action: item.action === "deny" ? "omit" : "deny",
                                      })
                                    }
                                  />
                                </div>
                              </div>
                            </div>
                            <div className="flex shrink-0 items-center gap-1">
                              <DeleteButton
                                label={t("common.delete")}
                                onClick={() => deleteConfirm.openDelete(item)}
                              />
                            </div>
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
                          <SelectCheckbox
                            checked={selected.size === items.length}
                            onToggle={toggleSelectAll}
                          />
                        </TableHead>
                        <TableHead className="w-16">{t("blocked.id")}</TableHead>
                        <TableHead>{t("blocked.word")}</TableHead>
                        <TableHead className="w-32">{t("blocked.action")}</TableHead>
                        <TableHead className="w-24">{t("blocked.hit_count")}</TableHead>
                        <TableHead className="w-32">{t("common.created")}</TableHead>
                        <TableHead className="w-24">{t("common.actions")}</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {items.map((item) => {
                        const isSelected = selected.has(item.id);
                        return (
                          <TableRow key={item.id}>
                            <TableCell className="w-10">
                              <SelectCheckbox
                                checked={isSelected}
                                onToggle={() => toggleSelect(item.id)}
                              />
                            </TableCell>
                            <TableCell className="text-muted-foreground">{item.id}</TableCell>
                            <TableCell className="font-medium">{item.word}</TableCell>
                            <TableCell>
                              <ActionBadge
                                action={item.action}
                                t={t}
                                disabled={toggleAction.updatingKey !== null}
                                onClick={() =>
                                  toggleAction.apply(item, {
                                    action: item.action === "deny" ? "omit" : "deny",
                                  })
                                }
                              />
                            </TableCell>
                            <TableCell>{item.hitCount}</TableCell>
                            <TableCell className="text-muted-foreground">
                              {new Date(item.createdAt).toLocaleDateString()}
                            </TableCell>
                            <TableCell>
                              <DeleteButton
                                label={t("common.delete")}
                                onClick={() => deleteConfirm.openDelete(item)}
                              />
                            </TableCell>
                          </TableRow>
                        );
                      })}
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

        <DeleteConfirmDialog
          {...deleteConfirm.dialogProps}
          title={t("common.are_you_sure")}
          description={t("blocked.delete_confirm")}
          confirmLabel={t("common.delete")}
        />

        <DeleteConfirmDialog
          open={batchDeleteConfirmOpen}
          onOpenChange={setBatchDeleteConfirmOpen}
          title={t("common.are_you_sure")}
          description={t("blocked.batch_delete_confirm").replace("{count}", String(selected.size))}
          confirmLabel={t("common.delete")}
          loading={batchDeleting}
          onConfirm={handleBatchDelete}
        />
      </div>
    </PermissionGuard>
  );
}
