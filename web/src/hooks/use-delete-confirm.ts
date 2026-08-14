"use client";

import { useCallback, useEffect, useRef, useState } from "react";

interface UseDeleteConfirmOptions<T> {
  /** 执行删除（含成功后刷新列表）；应把错误抛出（或自行处理），hook 据此决定是否关闭 */
  onConfirm: (target: T) => Promise<void>;
  /** 删除失败回调（如 showErrorToast），不传则静默 */
  onError?: (err: unknown) => void;
  /** 失败后是否关闭对话框，默认 true（与多数列表页 finally 关闭一致）；trigger 等页面传 false */
  closeOnError?: boolean;
}

/**
 * 列表页统一的删除二次确认状态机：
 * 管理 open / target / loading 三个状态，替代各页面手写的
 * deleteConfirmOpen + deleteTarget + deleting 三件套。
 *
 * 用法：
 *   const del = useDeleteConfirm<APIKeyItem>({
 *     onConfirm: async (key) => {
 *       await api.deleteAPIKey(key.id);
 *       toast.success(t("apikeys.deleted_success"));
 *       fetchKeys(pageInfo.page, pageInfo.pageSize, searchQuery || undefined);
 *     },
 *     onError: (err) => showErrorToast(err, { title: t("apikeys.delete_error") }),
 *   });
 *   ...
 *   <DeleteButton
 *     disabled={del.loading && del.target?.id === key.id}
 *     onClick={() => del.openDelete(key)}
 *   />
 *   <DeleteConfirmDialog
 *     {...del.dialogProps}
 *     title={t("common.are_you_sure")}
 *     description={t("apikeys.delete_description").replace("{name}", del.target?.name ?? "")}
 *     confirmLabel={t("common.delete")}
 *   />
 */
export function useDeleteConfirm<T>(options: UseDeleteConfirmOptions<T>) {
  const [target, setTarget] = useState<T | null>(null);
  const [loading, setLoading] = useState(false);
  const optionsRef = useRef(options);
  useEffect(() => {
    optionsRef.current = options;
  });

  const open = Boolean(target);

  const openDelete = useCallback((item: T) => setTarget(item), []);

  const close = useCallback(() => setTarget(null), []);

  const confirm = useCallback(async () => {
    if (!target) return;
    setLoading(true);
    try {
      await optionsRef.current.onConfirm(target);
      setTarget(null);
    } catch (err) {
      optionsRef.current.onError?.(err);
      if (optionsRef.current.closeOnError ?? true) {
        setTarget(null);
      }
    } finally {
      setLoading(false);
    }
  }, [target]);

  return {
    open,
    target,
    loading,
    openDelete,
    close,
    confirm,
    dialogProps: {
      open,
      onOpenChange: close,
      loading,
      onConfirm: confirm,
    },
  };
}
