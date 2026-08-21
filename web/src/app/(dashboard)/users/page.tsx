"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { MoreHorizontal, Users } from "lucide-react";
import { api } from "@/lib/api-client";
import { showErrorToast } from "@/lib/api-error-handler";
import { useAuth } from "@/lib/auth-context";
import { PermissionGuard } from "@/components/permission-guard";
import type { PageInfo, UserItem } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { PageHeader } from "@/components/page-header";
import { ListEmptyState } from "@/components/list-empty-state";
import { TableSkeleton } from "@/components/table-skeleton";
import { PaginationBar } from "@/components/pagination-bar";
import { DeleteConfirmDialog } from "@/components/delete-confirm-dialog";
import { toast } from "sonner";
import { usePersistentState } from "@/hooks/use-persistent-state";
import { useOptimisticUpdate } from "@/hooks/use-optimistic-update";
import { useIsMobile } from "@/hooks/use-mobile";
import { useI18n } from "@/lib/i18n";
import { FilterBar } from "@/components/filter-bar/filter-bar";
import { useFilterBar } from "@/components/filter-bar/use-filter-bar";
import type { FacetDef, FilterBarQueryParams } from "@/components/filter-bar/types";

const PERMISSIONS = ["pending", "demo", "user", "admin"] as const;

type UserAction = "promote" | "demote" | "delete" | "setDemo" | "restoreDemo";

interface UserRowActionsProps {
  user: UserItem;
  currentUserId: number | null;
  onAction: (action: UserAction, user: UserItem) => void;
}

/** 行操作菜单：pending→升为 User；user→降级为 Pending；pending/user→设为 Demo；demo→恢复为 User；均可删除；admin 与自己只读 */
function UserRowActions({ user, currentUserId, onAction }: UserRowActionsProps) {
  const { t } = useI18n();
  const canOperate = user.permission !== "admin" && user.id !== currentUserId;
  if (!canOperate) {
    return null;
  }
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={<Button variant="ghost" size="icon" aria-label={t("users.actions_aria")} />}
      >
        <MoreHorizontal className="size-4" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="min-w-36 p-1">
        {user.permission === "pending" && (
          <DropdownMenuItem onClick={() => onAction("promote", user)}>
            {t("users.promote")}
          </DropdownMenuItem>
        )}
        {user.permission === "user" && (
          <DropdownMenuItem onClick={() => onAction("demote", user)}>
            {t("users.demote")}
          </DropdownMenuItem>
        )}
        {(user.permission === "pending" || user.permission === "user") && (
          <DropdownMenuItem onClick={() => onAction("setDemo", user)}>
            {t("users.set_demo")}
          </DropdownMenuItem>
        )}
        {user.permission === "demo" && (
          <DropdownMenuItem onClick={() => onAction("restoreDemo", user)}>
            {t("users.restore_demo")}
          </DropdownMenuItem>
        )}
        <DropdownMenuItem variant="destructive" onClick={() => onAction("delete", user)}>
          {t("users.delete_menu")}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

export default function UsersPage() {
  const { user: currentUser } = useAuth();
  const [items, setItems] = useState<UserItem[]>([]);
  const [persistedPage, setPersistedPage] = usePersistentState("dashboard.users.page", 1);
  const [persistedPageSize, setPersistedPageSize] = usePersistentState(
    "dashboard.users.pageSize",
    20,
  );
  const [pageInfo, setPageInfo] = useState<PageInfo>({
    page: persistedPage,
    pageSize: persistedPageSize,
    total: 0,
  });
  const [loading, setLoading] = useState(true);
  const [acting, setActing] = useState<UserAction | null>(null);
  const [confirmAction, setConfirmAction] = useState<UserAction | null>(null);
  const [confirmUser, setConfirmUser] = useState<UserItem | null>(null);
  const { t, locale } = useI18n();
  const isMobile = useIsMobile();

  const facets = useMemo<FacetDef[]>(
    () => [
      {
        key: "permission",
        label: t("users.permission_filter"),
        options: [...PERMISSIONS],
        target: "param",
        single: true,
        formatValue: (v) => t(`permission.${v}`),
      },
    ],
    // locale 必须在依赖里：t 引用已稳定（见 lib/i18n.tsx），翻译文本刷新只能靠 locale 驱动重算
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [locale],
  );

  const filterBar = useFilterBar({
    persistKey: "dashboard.users",
    facets,
    freeTextPlaceholder: t("users.search_placeholder"),
  });
  const { queryParams } = filterBar;

  const fetchUsers = useCallback(
    async (page: number, pageSize: number, qp: FilterBarQueryParams) => {
      setLoading(true);
      try {
        const rsp = await api.listUsers(page, pageSize, {
          query: qp.freeText || undefined,
          permission: qp.params.permission,
        });
        setItems(rsp.items ?? []);
        if (rsp.pageInfo) {
          setPageInfo(rsp.pageInfo);
          setPersistedPage(rsp.pageInfo.page);
          setPersistedPageSize(rsp.pageInfo.pageSize);
        }
      } catch (err) {
        showErrorToast(err, { title: t("users.load_error") });
      } finally {
        setLoading(false);
      }
    },
    [setPersistedPage, setPersistedPageSize, t],
  );

  /* eslint-disable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps -- token 变化回到第 1 页查询；挂载时以持久化筛选发起首次查询 */
  useEffect(() => {
    fetchUsers(1, persistedPageSize, queryParams);
  }, [queryParams]);
  /* eslint-enable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps */

  // 批准（pending→user）：乐观更新 + 失败回滚，避免整表重拉导致闪烁
  const promoteUser = useOptimisticUpdate<UserItem>({
    setItems,
    getKey: (u) => u.id,
    update: async (u) => {
      await api.approveUser(u.id);
    },
    onSuccess: () => toast.success(t("users.approved_success")),
    onError: (err) => showErrorToast(err, { title: t("users.approve_error") }),
  });

  const runAction = useCallback(
    async (action: UserAction, user: UserItem) => {
      setActing(action);
      try {
        if (action === "demote") {
          await api.demoteUser(user.id);
          toast.success(t("users.demote_success"));
        } else if (action === "setDemo") {
          await api.setDemoUser(user.id);
          toast.success(t("users.set_demo_success"));
        } else if (action === "restoreDemo") {
          await api.restoreDemoUser(user.id);
          toast.success(t("users.restore_demo_success"));
        } else {
          await api.deleteUser(user.id);
          toast.success(t("users.delete_success"));
        }
        fetchUsers(pageInfo.page, pageInfo.pageSize, queryParams);
      } catch (err) {
        const titles: Record<UserAction, string> = {
          promote: t("users.approve_error"),
          demote: t("users.demote_error"),
          delete: t("users.delete_error"),
          setDemo: t("users.set_demo_error"),
          restoreDemo: t("users.restore_demo_error"),
        };
        showErrorToast(err, { title: titles[action] });
      } finally {
        setActing(null);
      }
    },
    [fetchUsers, pageInfo.page, pageInfo.pageSize, queryParams, t],
  );

  const handleAction = useCallback(
    (action: UserAction, user: UserItem) => {
      if (action === "promote") {
        promoteUser.apply(user, { permission: "user" });
        return;
      }
      setConfirmAction(action);
      setConfirmUser(user);
    },
    [promoteUser],
  );

  const confirmMeta: Record<
    Exclude<UserAction, "promote">,
    { title: string; desc: string; label: string }
  > = {
    demote: {
      title: t("users.demote_confirm_title"),
      desc: t("users.demote_confirm_desc"),
      label: t("users.demote"),
    },
    delete: {
      title: t("users.delete_confirm_title"),
      desc: t("users.delete_confirm_desc"),
      label: t("common.delete"),
    },
    setDemo: {
      title: t("users.set_demo_confirm_title"),
      desc: t("users.set_demo_confirm_desc"),
      label: t("users.set_demo"),
    },
    restoreDemo: {
      title: t("users.restore_demo_confirm_title"),
      desc: t("users.restore_demo_confirm_desc"),
      label: t("users.restore_demo"),
    },
  };

  const handleConfirm = useCallback(() => {
    if (confirmAction && confirmUser) {
      runAction(confirmAction, confirmUser);
    }
    setConfirmAction(null);
    setConfirmUser(null);
  }, [confirmAction, confirmUser, runAction]);

  return (
    <PermissionGuard adminOnly>
      <div className="space-y-8">
        <PageHeader title={t("users.title")} description={t("users.subtitle")} />

        <Card>
          <CardHeader>
            <CardTitle className="font-display">{t("users.list_title")}</CardTitle>
          </CardHeader>
          <CardContent>
            {/* Filters — faceted bar */}
            <div className="mb-4 flex">
              <FilterBar
                {...filterBar}
                facets={facets}
                placeholder={t("users.search_placeholder")}
              />
            </div>
            {filterBar.tokens.length > 0 && (
              <p className="-mt-2 mb-3 text-xs text-muted-foreground">
                {t("filter_bar.applied_count").replace("{count}", String(filterBar.tokens.length))}
              </p>
            )}
            {loading ? (
              <TableSkeleton />
            ) : items.length === 0 ? (
              <ListEmptyState
                icon={<Users className="mb-3 size-10 text-muted-foreground/40" />}
                message={t("users.no_users")}
              />
            ) : (
              <>
                {isMobile ? (
                  <div className="space-y-3">
                    {items.map((user) => (
                      <div key={user.id} className="rounded-lg border border-border bg-card p-4">
                        <div className="flex items-start justify-between gap-3">
                          <div className="min-w-0 flex-1">
                            <p className="text-sm font-medium">{user.name}</p>
                            <p className="mt-0.5 text-xs text-muted-foreground">{user.email}</p>
                            <p className="mt-0.5 text-xs text-muted-foreground">
                              {t(`permission.${user.permission}`)}
                            </p>
                          </div>
                          <UserRowActions
                            user={user}
                            currentUserId={currentUser?.id ?? null}
                            onAction={handleAction}
                          />
                        </div>
                      </div>
                    ))}
                  </div>
                ) : (
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>{t("users.name")}</TableHead>
                        <TableHead>{t("users.email")}</TableHead>
                        <TableHead>{t("users.permission")}</TableHead>
                        <TableHead>{t("users.created_at")}</TableHead>
                        <TableHead>{t("users.last_login")}</TableHead>
                        <TableHead className="w-14 text-right">{t("common.actions")}</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {items.map((user) => (
                        <TableRow key={user.id}>
                          <TableCell className="font-medium">{user.name}</TableCell>
                          <TableCell className="text-muted-foreground">{user.email}</TableCell>
                          <TableCell className="text-muted-foreground">
                            {t(`permission.${user.permission}`)}
                          </TableCell>
                          <TableCell className="text-muted-foreground">
                            {user.createdAt ? new Date(user.createdAt).toLocaleDateString() : "—"}
                          </TableCell>
                          <TableCell className="text-muted-foreground">
                            {user.lastLogin ? new Date(user.lastLogin).toLocaleDateString() : "—"}
                          </TableCell>
                          <TableCell className="text-right">
                            <UserRowActions
                              user={user}
                              currentUserId={currentUser?.id ?? null}
                              onAction={handleAction}
                            />
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                )}
                <PaginationBar
                  pageInfo={pageInfo}
                  onChange={(page, pageSize) => fetchUsers(page, pageSize, queryParams)}
                  totalLabel={t("pagination.items")}
                />
              </>
            )}
          </CardContent>
        </Card>

        <DeleteConfirmDialog
          open={confirmAction !== null}
          onOpenChange={(open) => {
            if (!open) {
              setConfirmAction(null);
              setConfirmUser(null);
            }
          }}
          title={
            confirmAction && confirmAction !== "promote" ? confirmMeta[confirmAction].title : ""
          }
          description={
            confirmAction && confirmAction !== "promote" ? confirmMeta[confirmAction].desc : ""
          }
          confirmLabel={
            confirmAction && confirmAction !== "promote" ? confirmMeta[confirmAction].label : ""
          }
          loadingLabel={t("common.processing")}
          loading={acting !== null}
          onConfirm={handleConfirm}
        />
      </div>
    </PermissionGuard>
  );
}
