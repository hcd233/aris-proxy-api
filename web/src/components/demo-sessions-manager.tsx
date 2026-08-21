"use client";

import { useCallback, useEffect, useState } from "react";
import { Check, MessageSquare, Trash2 } from "lucide-react";
import { api } from "@/lib/api-client";
import { showErrorToast } from "@/lib/api-error-handler";
import { useT } from "@/lib/i18n";
import type { DemoSession, PageInfo } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { ListEmptyState } from "@/components/list-empty-state";
import { TableSkeleton } from "@/components/table-skeleton";
import { PaginationBar } from "@/components/pagination-bar";
import { DeleteConfirmDialog } from "@/components/delete-confirm-dialog";
import { ProviderIcon } from "@/components/provider-icon";
import { usePersistentState } from "@/hooks/use-persistent-state";
import { useIsMobile } from "@/hooks/use-mobile";
import { toast } from "sonner";
import { cn, formatDateTime } from "@/lib/utils";

/** 已选列表共用的行结构（DemoSession 可赋值） */
interface SessionRow {
  id: number;
  summary?: string;
  score?: number;
  messageCount: number;
  toolCount: number;
  createdAt?: string;
  modelIds?: string[];
}

/** 勾选框（对齐 sessions/trigger 的 role="checkbox" 自绘模式） */
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

function SessionListTable({
  items,
  selectedIds,
  onToggle,
  onToggleAll,
  emptyMessage,
  isMobile,
}: {
  items: SessionRow[];
  selectedIds: Set<number>;
  onToggle: (id: number) => void;
  onToggleAll: () => void;
  emptyMessage: string;
  isMobile: boolean;
}) {
  const t = useT();

  if (items.length === 0) {
    return (
      <ListEmptyState
        icon={<MessageSquare className="mb-3 size-10 text-muted-foreground/40" />}
        message={emptyMessage}
      />
    );
  }

  const summary = (item: SessionRow) =>
    item.summary || t("sessions.untitled_session").replace("{id}", String(item.id));

  if (isMobile) {
    return (
      <div className="space-y-3">
        {items.map((item) => {
          const isSelected = selectedIds.has(item.id);
          return (
            <div key={item.id} className="rounded-lg border border-border bg-card p-4">
              <div className="flex items-start gap-2">
                <SelectCheckbox checked={isSelected} onToggle={() => onToggle(item.id)} />
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-medium">{summary(item)}</p>
                  <div className="mt-2 flex items-center gap-3 text-xs text-muted-foreground">
                    <span>
                      {t("common.id")}: {item.id}
                    </span>
                    <span>
                      {t("sessions.messages")}: {item.messageCount}
                    </span>
                    <span>
                      {t("sessions.tools")}: {item.toolCount}
                    </span>
                    {item.score != null && (
                      <span>
                        {t("sessions.score")}: ★{item.score}
                      </span>
                    )}
                  </div>
                  <div className="mt-1.5 flex flex-wrap items-center gap-3 text-xs text-muted-foreground">
                    {item.modelIds && item.modelIds.length > 0 && (
                      <div className="flex items-center gap-1">
                        {item.modelIds.map((m) => (
                          <ProviderIcon key={m} protocol={m} size={12} />
                        ))}
                      </div>
                    )}
                    {item.createdAt && <span>{formatDateTime(item.createdAt)}</span>}
                  </div>
                </div>
              </div>
            </div>
          );
        })}
      </div>
    );
  }

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead className="w-10">
            <SelectCheckbox checked={selectedIds.size === items.length} onToggle={onToggleAll} />
          </TableHead>
          <TableHead>{t("common.id")}</TableHead>
          <TableHead className="w-[160px] whitespace-nowrap">{t("sessions.time")}</TableHead>
          <TableHead>{t("sessions.summary")}</TableHead>
          <TableHead className="w-[160px] text-center">{t("sessions.score")}</TableHead>
          <TableHead className="w-24">{t("sessions.messages")}</TableHead>
          <TableHead className="w-24">{t("sessions.tools")}</TableHead>
          <TableHead className="w-[140px]">{t("sessions.models")}</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {items.map((item) => {
          const isSelected = selectedIds.has(item.id);
          return (
            <TableRow key={item.id}>
              <TableCell className="w-10">
                <SelectCheckbox checked={isSelected} onToggle={() => onToggle(item.id)} />
              </TableCell>
              <TableCell className="font-mono text-xs">{item.id}</TableCell>
              <TableCell className="whitespace-nowrap text-muted-foreground">
                {item.createdAt ? formatDateTime(item.createdAt) : "—"}
              </TableCell>
              <TableCell className="max-w-[200px] truncate">{summary(item)}</TableCell>
              <TableCell className="w-[160px]">
                <div className="flex justify-center">
                  {item.score != null ? (
                    <span className="text-sm">
                      {"★".repeat(item.score)}
                      <span className="text-muted-foreground/30">{"★".repeat(5 - item.score)}</span>
                    </span>
                  ) : (
                    <span className="text-muted-foreground">—</span>
                  )}
                </div>
              </TableCell>
              <TableCell>{item.messageCount}</TableCell>
              <TableCell>{item.toolCount}</TableCell>
              <TableCell>
                <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
                  {item.modelIds && item.modelIds.length > 0 ? (
                    item.modelIds.map((m) => (
                      <span
                        key={m}
                        className="inline-flex items-center gap-1 text-xs text-muted-foreground"
                      >
                        <ProviderIcon protocol={m} size={14} />
                        {m}
                      </span>
                    ))
                  ) : (
                    <span className="text-muted-foreground">—</span>
                  )}
                </div>
              </TableCell>
            </TableRow>
          );
        })}
      </TableBody>
    </Table>
  );
}

/** Demo sessions 管理：已选列表（demo 账户可见的 sessions），增删入口在 sessions 页 */
export function DemoSessionsManager() {
  const t = useT();
  const isMobile = useIsMobile();

  const [selectedSessions, setSelectedSessions] = useState<DemoSession[]>([]);
  const [selectedPage, setSelectedPage] = usePersistentState("dashboard.demo.selected.page", 1);
  const [selectedPageSize, setSelectedPageSize] = usePersistentState(
    "dashboard.demo.selected.pageSize",
    100,
  );
  const [selectedPageInfo, setSelectedPageInfo] = useState<PageInfo>({
    page: selectedPage,
    pageSize: selectedPageSize,
    total: 0,
  });
  const [selectedLoading, setSelectedLoading] = useState(true);
  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set());
  const [removing, setRemoving] = useState(false);
  const [removeConfirmOpen, setRemoveConfirmOpen] = useState(false);

  const fetchSelected = useCallback(
    async (page: number, pageSize: number) => {
      setSelectedLoading(true);
      try {
        const rsp = await api.listDemoSessions(page, pageSize);
        setSelectedSessions(rsp.sessions ?? []);
        if (rsp.pageInfo) {
          setSelectedPageInfo(rsp.pageInfo);
          setSelectedPage(rsp.pageInfo.page);
          setSelectedPageSize(rsp.pageInfo.pageSize);
        }
        setSelectedIds(new Set());
      } catch (err) {
        showErrorToast(err, { title: t("demo.sessions_load_error") });
      } finally {
        setSelectedLoading(false);
      }
    },
    [setSelectedPage, setSelectedPageSize, t],
  );

  /* eslint-disable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps -- Initial fetch on mount with persisted pagination */
  useEffect(() => {
    fetchSelected(selectedPage, selectedPageSize);
  }, [fetchSelected]);
  /* eslint-enable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps */

  const toggleSelectedId = useCallback((id: number) => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }, []);

  const toggleSelectedAll = useCallback(() => {
    if (selectedIds.size === selectedSessions.length) {
      setSelectedIds(new Set());
    } else {
      setSelectedIds(new Set(selectedSessions.map((s) => s.id)));
    }
  }, [selectedIds.size, selectedSessions]);

  const handleRemove = useCallback(async () => {
    if (selectedIds.size === 0) return;
    setRemoving(true);
    try {
      const ids = Array.from(selectedIds);
      await api.removeDemoSessions(ids);
      toast.success(t("demo.remove_success").replace("{count}", String(ids.length)));
      setSelectedIds(new Set());
      await fetchSelected(selectedPage, selectedPageSize);
    } catch (err) {
      showErrorToast(err, { title: t("demo.remove_error") });
    } finally {
      setRemoving(false);
      setRemoveConfirmOpen(false);
    }
  }, [fetchSelected, selectedIds, selectedPage, selectedPageSize, t]);

  return (
    <div className="space-y-8">
      <Card>
        <CardHeader>
          <CardTitle className="font-display">{t("demo.selected_title")}</CardTitle>
          <CardDescription>{t("demo.selected_desc")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center justify-between gap-3">
            {selectedIds.size > 0 ? (
              <Button
                variant="destructive"
                size="sm"
                onClick={() => setRemoveConfirmOpen(true)}
                className="gap-1.5"
              >
                <Trash2 className="size-3.5" />
                {t("demo.remove_selected")} {selectedIds.size}
              </Button>
            ) : (
              <span />
            )}
          </div>

          {selectedLoading ? (
            <TableSkeleton rows={3} />
          ) : (
            <SessionListTable
              items={selectedSessions}
              selectedIds={selectedIds}
              onToggle={toggleSelectedId}
              onToggleAll={toggleSelectedAll}
              emptyMessage={t("demo.no_selected")}
              isMobile={isMobile}
            />
          )}

          <PaginationBar
            pageInfo={selectedPageInfo}
            onChange={(page, pageSize) => fetchSelected(page, pageSize)}
            totalLabel={t("pagination.sessions")}
          />
        </CardContent>
      </Card>

      <DeleteConfirmDialog
        open={removeConfirmOpen}
        onOpenChange={setRemoveConfirmOpen}
        title={t("demo.remove_confirm_title")}
        description={t("demo.remove_confirm_desc").replace("{count}", String(selectedIds.size))}
        confirmLabel={`${t("demo.remove_selected")} ${selectedIds.size}`}
        loadingLabel={t("common.processing")}
        loading={removing}
        onConfirm={handleRemove}
      />
    </div>
  );
}
