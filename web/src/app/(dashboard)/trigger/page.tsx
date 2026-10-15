"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api-client";
import { showErrorToast } from "@/lib/api-error-handler";
import { useAuth } from "@/lib/auth-context";
import { PermissionGuard } from "@/components/permission-guard";
import type { TriggerAction, TriggerItem, PageInfo } from "@/lib/types";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Item,
  ItemActions,
  ItemContent,
  ItemDescription,
  ItemGroup,
  ItemMedia,
  ItemTitle,
} from "@/components/ui/item";
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
import {
  TooltipProvider,
  TooltipRoot,
  TooltipTrigger,
  TooltipContent,
} from "@/components/ui/tooltip";
import { DeleteConfirmDialog } from "@/components/delete-confirm-dialog";
import { PageHeader } from "@/components/page-header";
import { ListEmptyState } from "@/components/list-empty-state";
import { TableSkeleton } from "@/components/table-skeleton";
import { Ban, Lock, Trash2 } from "lucide-react";
import { PaginationBar } from "@/components/pagination-bar";
import { FilterBar } from "@/components/filter-bar/filter-bar";
import { useFilterBar } from "@/components/filter-bar/use-filter-bar";
import { toast } from "sonner";
import { usePersistentState } from "@/hooks/use-persistent-state";
import { useOptimisticUpdate } from "@/hooks/use-optimistic-update";
import { useDeleteConfirm } from "@/hooks/use-delete-confirm";
import { useRequestSeq } from "@/hooks/use-request-seq";
import { useIsMobile } from "@/hooks/use-mobile";
import { useT } from "@/lib/i18n";
import { cn } from "@/lib/utils";
import { copyTextToClipboard } from "@/lib/clipboard";

/** 动作徽章：点击循环切换 deny→omit→capture，悬停用 Tooltip 展示切换提示 */
function ActionBadge({
  action,
  t,
  onClick,
  disabled,
}: {
  action: TriggerAction;
  t: (key: string) => string;
  onClick: () => void;
  disabled?: boolean;
}) {
  return (
    <TooltipProvider>
      <TooltipRoot>
        <TooltipTrigger
          render={
            <Badge
              render={<button type="button" onClick={onClick} disabled={disabled} />}
              variant={action === "deny" ? "destructive" : "secondary"}
              className={cn(
                "cursor-pointer disabled:cursor-not-allowed disabled:opacity-60",
                action === "capture" &&
                  "border-transparent bg-blue-500/10 text-blue-600 hover:bg-blue-500/20 dark:text-blue-400 moonshot:bg-secondary moonshot:text-muted-foreground moonshot:hover:bg-accent",
                action === "omit" &&
                  "border-transparent bg-emerald-500/10 text-emerald-600 hover:bg-emerald-500/20 dark:text-emerald-400 moonshot:bg-secondary moonshot:text-muted-foreground moonshot:hover:bg-accent",
              )}
            >
              {action === "deny"
                ? t("trigger.action_deny")
                : action === "capture"
                  ? t("trigger.action_capture")
                  : t("trigger.action_omit")}
            </Badge>
          }
        />
        <TooltipContent side="top">{t("trigger.action_switch_hint")}</TooltipContent>
      </TooltipRoot>
    </TooltipProvider>
  );
}

export default function TriggerPage() {
  const [items, setItems] = useState<TriggerItem[]>([]);
  const [persistedPage, setPersistedPage] = usePersistentState("dashboard.trigger.page", 1);
  const [persistedPageSize, setPersistedPageSize] = usePersistentState(
    "dashboard.trigger.pageSize",
    20,
  );
  const [pageInfo, setPageInfo] = useState<PageInfo>({
    page: persistedPage,
    pageSize: persistedPageSize,
    total: 0,
  });
  const [loading, setLoading] = useState(true);
  const [inlineWord, setInlineWord] = useState("");
  const [adding, setAdding] = useState(false);
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [batchDeleting, setBatchDeleting] = useState(false);
  const [batchDeleteConfirmOpen, setBatchDeleteConfirmOpen] = useState(false);
  const t = useT();
  const { isDemo } = useAuth();
  const isMobile = useIsMobile();

  const filterBar = useFilterBar({
    persistKey: "dashboard.trigger",
    facets: [],
    freeTextPlaceholder: t("trigger.search_placeholder"),
  });
  const { queryParams } = filterBar;

  // 竞态守卫：关键词/翻页连点时慢响应不得覆盖新响应
  const requestSeq = useRequestSeq();
  const fetchItems = useCallback(
    async (page: number, pageSize: number, query?: string) => {
      const seq = requestSeq.begin();
      setLoading(true);
      try {
        const safeSize = pageSize > 0 ? pageSize : 20;
        const rsp = await api.listTrigger(page, safeSize, query);
        if (!requestSeq.isCurrent(seq)) return;
        setItems(rsp.trigger ?? []);
        if (rsp.pageInfo) {
          setPageInfo(rsp.pageInfo);
          setPersistedPage(rsp.pageInfo.page);
          setPersistedPageSize(rsp.pageInfo.pageSize);
        }
        setSelected(new Set()); // 翻页/刷新清空选中（对齐 sessions）
      } catch (err) {
        if (!requestSeq.isCurrent(seq)) return;
        showErrorToast(err, { title: t("trigger.load_error") });
      } finally {
        if (requestSeq.isCurrent(seq)) setLoading(false);
      }
    },
    [setPersistedPage, setPersistedPageSize, t, requestSeq],
  );

  /* eslint-disable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps -- 关键词 token 变化回到第 1 页查询；挂载时以持久化关键词发起首次查询 */
  useEffect(() => {
    fetchItems(1, persistedPageSize, queryParams.freeText || undefined);
  }, [queryParams]);
  /* eslint-enable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps */

  const handleInlineAdd = useCallback(async () => {
    const word = inlineWord.trim();
    if (!word || adding) return;
    setAdding(true);
    try {
      await api.createTrigger({ word, action: "deny" });
      toast.success(t("trigger.created_success"));
      setInlineWord("");
      fetchItems(persistedPage, persistedPageSize);
    } catch (err) {
      showErrorToast(err, { title: t("trigger.create_error") });
    } finally {
      setAdding(false);
    }
  }, [inlineWord, adding, fetchItems, persistedPage, persistedPageSize, t]);

  // action 徽章切换：乐观更新 + 失败回滚，避免整表重拉导致闪烁
  const toggleAction = useOptimisticUpdate<TriggerItem>({
    setItems,
    getKey: (item) => item.id,
    update: async (item) => {
      await api.updateTrigger(item.id, { action: item.action });
    },
    onSuccess: () => toast.success(t("trigger.action_updated")),
    onError: (err) => showErrorToast(err, { title: t("trigger.action_update_error") }),
  });

  const deleteConfirm = useDeleteConfirm<TriggerItem>({
    onConfirm: async (item) => {
      await api.deleteTrigger(item.id);
      toast.success(t("trigger.deleted_success"));
      fetchItems(persistedPage, persistedPageSize);
    },
    onError: (err) => showErrorToast(err, { title: t("trigger.delete_error") }),
    closeOnError: false,
  });

  const handleCopyWord = (word: string) => {
    if (!word) return;
    void copyTextToClipboard(word).then((ok) =>
      ok ? toast.success(t("common.copied_to_clipboard")) : toast.error(t("common.copy_failed")),
    );
  };

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
      const rsp = await api.batchDeleteTrigger(ids);
      toast.success(
        t("trigger.batch_delete_success").replace(
          "{count}",
          String(rsp.deletedCount ?? ids.length),
        ),
      );
      fetchItems(persistedPage, persistedPageSize);
    } catch (err) {
      showErrorToast(err, { title: t("trigger.batch_delete_error") });
    } finally {
      setBatchDeleting(false);
      setBatchDeleteConfirmOpen(false);
    }
  }, [selected, fetchItems, persistedPage, persistedPageSize, t]);

  return (
    <PermissionGuard adminOnly module="trigger">
      <TooltipProvider>
        <div className="space-y-8">
          <PageHeader title={t("trigger.title")} description={t("trigger.subtitle")} />

          <Card>
            <CardHeader>
              <CardTitle className="font-display">{t("trigger.all_words")}</CardTitle>
            </CardHeader>
            <CardContent>
              {/* Search — faceted bar */}
              <div className="mb-4 flex flex-col gap-2 sm:flex-row sm:items-center">
                <FilterBar
                  {...filterBar}
                  facets={[]}
                  placeholder={t("trigger.search_placeholder")}
                />
                <Input
                  className="sm:max-w-xs"
                  placeholder={t("trigger.inline_add_placeholder")}
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
                    disabled={isDemo()}
                    onClick={() => setBatchDeleteConfirmOpen(true)}
                    className="gap-1.5 sm:ml-auto"
                  >
                    {isDemo() ? <Lock className="size-3.5" /> : <Trash2 className="size-3.5" />}
                    {t("common.delete")} {selected.size}
                  </Button>
                )}
              </div>
              {filterBar.tokens.length > 0 && (
                <p className="-mt-2 mb-3 text-xs text-muted-foreground">
                  {t("filter_bar.applied_count").replace(
                    "{count}",
                    String(filterBar.tokens.length),
                  )}
                </p>
              )}
              {loading ? (
                <TableSkeleton />
              ) : items.length === 0 ? (
                <ListEmptyState icon={Ban} message={t("trigger.no_words")} />
              ) : (
                <>
                  {isMobile ? (
                    <ItemGroup className="gap-3">
                      {items.map((item) => {
                        const isSelected = selected.has(item.id);
                        return (
                          <Item
                            key={item.id}
                            variant="outline"
                            role="listitem"
                            className="items-start"
                          >
                            <ItemMedia>
                              <Checkbox
                                checked={isSelected}
                                onCheckedChange={() => toggleSelect(item.id)}
                              />
                            </ItemMedia>
                            <ItemContent className="min-w-0">
                              <TooltipRoot>
                                <TooltipTrigger
                                  render={
                                    <ItemTitle
                                      className="line-clamp-none cursor-pointer break-all underline-offset-2 hover:underline"
                                      onClick={() => handleCopyWord(item.word)}
                                    >
                                      {item.word}
                                    </ItemTitle>
                                  }
                                />
                                <TooltipContent side="top">
                                  {t("trigger.copy_word_title")}
                                </TooltipContent>
                              </TooltipRoot>
                              <ItemDescription>
                                {t("trigger.hit_count")}: {item.hitCount}
                              </ItemDescription>
                              <div>
                                <ActionBadge
                                  action={item.action}
                                  t={t}
                                  disabled={toggleAction.updatingKey !== null}
                                  onClick={() =>
                                    toggleAction.apply(item, {
                                      action:
                                        item.action === "deny"
                                          ? "omit"
                                          : item.action === "omit"
                                            ? "capture"
                                            : "deny",
                                    })
                                  }
                                />
                              </div>
                            </ItemContent>
                            <ItemActions>
                              <DeleteButton
                                label={t("common.delete")}
                                locked={isDemo()}
                                onClick={() => deleteConfirm.openDelete(item)}
                              />
                            </ItemActions>
                          </Item>
                        );
                      })}
                    </ItemGroup>
                  ) : (
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead className="w-10">
                            <Checkbox
                              checked={items.length > 0 && selected.size === items.length}
                              onCheckedChange={toggleSelectAll}
                            />
                          </TableHead>
                          <TableHead className="w-16">{t("trigger.id")}</TableHead>
                          <TableHead>{t("trigger.word")}</TableHead>
                          <TableHead className="w-32">{t("trigger.action")}</TableHead>
                          <TableHead className="w-24">{t("trigger.hit_count")}</TableHead>
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
                                <Checkbox
                                  checked={isSelected}
                                  onCheckedChange={() => toggleSelect(item.id)}
                                />
                              </TableCell>
                              <TableCell className="text-muted-foreground">{item.id}</TableCell>
                              <TableCell className="font-medium">
                                <TooltipRoot>
                                  <TooltipTrigger
                                    render={
                                      <span
                                        className="cursor-pointer underline-offset-2 hover:underline"
                                        onClick={() => handleCopyWord(item.word)}
                                      >
                                        {item.word}
                                      </span>
                                    }
                                  />
                                  <TooltipContent side="top">
                                    {t("trigger.copy_word_title")}
                                  </TooltipContent>
                                </TooltipRoot>
                              </TableCell>
                              <TableCell>
                                <ActionBadge
                                  action={item.action}
                                  t={t}
                                  disabled={toggleAction.updatingKey !== null}
                                  onClick={() =>
                                    toggleAction.apply(item, {
                                      action:
                                        item.action === "deny"
                                          ? "omit"
                                          : item.action === "omit"
                                            ? "capture"
                                            : "deny",
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
                                  locked={isDemo()}
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
                      fetchItems(page, pageSize, queryParams.freeText || undefined)
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
            description={t("trigger.delete_confirm")}
            confirmLabel={t("common.delete")}
          />

          <DeleteConfirmDialog
            open={batchDeleteConfirmOpen}
            onOpenChange={setBatchDeleteConfirmOpen}
            title={t("common.are_you_sure")}
            description={t("trigger.batch_delete_confirm").replace(
              "{count}",
              String(selected.size),
            )}
            confirmLabel={t("common.delete")}
            loading={batchDeleting}
            onConfirm={handleBatchDelete}
          />
        </div>
      </TooltipProvider>
    </PermissionGuard>
  );
}
