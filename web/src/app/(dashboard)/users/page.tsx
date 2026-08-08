"use client";

import { useCallback, useEffect, useState } from "react";
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
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { PageHeader } from "@/components/page-header";
import { SearchInput } from "@/components/search-input";
import { ListEmptyState } from "@/components/list-empty-state";
import { TableSkeleton } from "@/components/table-skeleton";
import { PaginationBar } from "@/components/pagination-bar";
import { DeleteConfirmDialog } from "@/components/delete-confirm-dialog";
import { toast } from "sonner";
import { usePersistentState } from "@/hooks/use-persistent-state";
import { useIsMobile } from "@/hooks/use-mobile";
import { useT } from "@/lib/i18n";

const PERMISSIONS = ["pending", "user", "admin"] as const;

type UserAction = "promote" | "demote" | "delete";

interface UserRowActionsProps {
  user: UserItem;
  currentUserId: number | null;
  onAction: (action: UserAction, user: UserItem) => void;
}

/** 行操作菜单：pending→升为 User；user→降级为 Pending；均可删除；admin 与自己只读 */
function UserRowActions({ user, currentUserId, onAction }: UserRowActionsProps) {
  const t = useT();
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
  const [persistedPageSize, setPersistedPageSize] = usePersistentState("dashboard.users.pageSize", 20);
  const [pageInfo, setPageInfo] = useState<PageInfo>({ page: persistedPage, pageSize: persistedPageSize, total: 0 });
  const [loading, setLoading] = useState(true);
  const [searchQuery, setSearchQuery] = useState("");
  const [permission, setPermission] = useState("");
  const [acting, setActing] = useState<UserAction | null>(null);
  const [confirmAction, setConfirmAction] = useState<UserAction | null>(null);
  const [confirmUser, setConfirmUser] = useState<UserItem | null>(null);
  const t = useT();
  const isMobile = useIsMobile();

  const fetchUsers = useCallback(async (page: number, pageSize: number, query?: string, perm?: string) => {
    setLoading(true);
    try {
      const rsp = await api.listUsers(page, pageSize, {
        query: query || undefined,
        permission: perm || undefined,
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
  }, [setPersistedPage, setPersistedPageSize, t]);

  /* eslint-disable react-hooks/set-state-in-effect -- Re-fetch list when the persisted page or size changes */
  useEffect(() => {
    fetchUsers(persistedPage, persistedPageSize, searchQuery || undefined, permission || undefined);
  }, [fetchUsers, persistedPage, persistedPageSize, searchQuery, permission]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const handleSearch = useCallback(() => {
    setPersistedPage(1);
    fetchUsers(1, persistedPageSize, searchQuery || undefined, permission || undefined);
  }, [fetchUsers, persistedPageSize, permission, searchQuery, setPersistedPage]);

  const runAction = useCallback(async (action: UserAction, user: UserItem) => {
    setActing(action);
    try {
      switch (action) {
        case "promote":
          await api.approveUser(user.id);
          toast.success(t("users.approved_success"));
          break;
        case "demote":
          await api.demoteUser(user.id);
          toast.success(t("users.demote_success"));
          break;
        case "delete":
          await api.deleteUser(user.id);
          toast.success(t("users.delete_success"));
          break;
      }
      fetchUsers(pageInfo.page, pageInfo.pageSize, searchQuery || undefined, permission || undefined);
    } catch (err) {
      showErrorToast(err, {
        title:
          action === "promote"
            ? t("users.approve_error")
            : action === "demote"
              ? t("users.demote_error")
              : t("users.delete_error"),
      });
    } finally {
      setActing(null);
    }
  }, [fetchUsers, pageInfo.page, pageInfo.pageSize, permission, searchQuery, t]);

  const handleAction = useCallback((action: UserAction, user: UserItem) => {
    if (action === "promote") {
      runAction("promote", user);
      return;
    }
    setConfirmAction(action);
    setConfirmUser(user);
  }, [runAction]);

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
            <div className="mb-4 flex flex-wrap items-center gap-3">
              <Select
                value={permission}
                onValueChange={(v) => {
                  setPermission(v === "all" ? "" : (v as string));
                  setPersistedPage(1);
                }}
              >
                <SelectTrigger className="w-40" aria-label={t("users.permission_filter")}>
                  <SelectValue placeholder={t("users.all_permissions")} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">{t("users.all_permissions")}</SelectItem>
                  {PERMISSIONS.map((p) => (
                    <SelectItem key={p} value={p}>{t(`permission.${p}`)}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <SearchInput
                placeholder={t("users.search_placeholder")}
                value={searchQuery}
                onChange={setSearchQuery}
                onSearch={handleSearch}
              />
            </div>
            {loading ? (
              <TableSkeleton />
            ) : items.length === 0 ? (
              <ListEmptyState icon={<Users className="mb-3 size-10 text-muted-foreground/40" />} message={t("users.no_users")} />
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
                          <TableCell className="text-muted-foreground">{t(`permission.${user.permission}`)}</TableCell>
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
                  onChange={(page, pageSize) => fetchUsers(page, pageSize, searchQuery || undefined, permission || undefined)}
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
          title={confirmAction === "demote" ? t("users.demote_confirm_title") : t("users.delete_confirm_title")}
          description={confirmAction === "demote" ? t("users.demote_confirm_desc") : t("users.delete_confirm_desc")}
          confirmLabel={confirmAction === "demote" ? t("users.demote") : t("common.delete")}
          loadingLabel={t("common.processing")}
          loading={acting !== null}
          onConfirm={handleConfirm}
        />
      </div>
    </PermissionGuard>
  );
}
