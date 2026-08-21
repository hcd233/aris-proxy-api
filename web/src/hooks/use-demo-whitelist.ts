"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { toast } from "sonner";
import { api } from "@/lib/api-client";
import { showErrorToast } from "@/lib/api-error-handler";
import { useT } from "@/lib/i18n";

/** 与后端 DemoSessionMaxPageSize 一致（internal/common/constant/string.go） */
const DEMO_WHITELIST_PAGE_SIZE = 100;

/**
 * demo 白名单视角（admin 用）：loginEnabled + 全部白名单 ID 集合。
 * 挂载时拉取一次 getDemoConfig 与 listDemoSessions 全量；toggle(id) 单条添加/移除并同步本地集合。
 */
export function useDemoWhitelist() {
  const t = useT();
  const [loginEnabled, setLoginEnabled] = useState(false);
  const [demoIds, setDemoIds] = useState<Set<number>>(new Set());
  const [pending, setPending] = useState(false);
  const pendingRef = useRef(false);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const cfg = await api.getDemoConfig();
        if (!cancelled) setLoginEnabled(cfg.config?.loginEnabled ?? false);
      } catch (err) {
        if (!cancelled) showErrorToast(err); // 失败保持 false → 按钮不渲染（fail-closed）
      }
      try {
        const ids = new Set<number>();
        let page = 1;
        for (;;) {
          const rsp = await api.listDemoSessions(page, DEMO_WHITELIST_PAGE_SIZE);
          if (cancelled) return;
          for (const s of rsp.sessions ?? []) ids.add(s.id);
          const total = Number(rsp.pageInfo?.total ?? ids.size);
          if (page * DEMO_WHITELIST_PAGE_SIZE >= total) break;
          page += 1;
        }
        if (!cancelled) setDemoIds(ids);
      } catch (err) {
        if (!cancelled) showErrorToast(err);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  const isInDemo = useCallback((id: number) => demoIds.has(id), [demoIds]);

  const toggle = useCallback(
    async (id: number) => {
      if (pendingRef.current) return;
      pendingRef.current = true;
      setPending(true);
      const wasInDemo = demoIds.has(id);
      try {
        if (wasInDemo) {
          await api.removeDemoSessions([id]);
          toast.success(t("demo.removed"));
          setDemoIds((prev) => {
            const next = new Set(prev);
            next.delete(id);
            return next;
          });
        } else {
          await api.addDemoSessions({ sessionIds: [id] });
          toast.success(t("demo.added"));
          setDemoIds((prev) => new Set(prev).add(id));
        }
      } catch (err) {
        showErrorToast(err);
      } finally {
        pendingRef.current = false;
        setPending(false);
      }
    },
    [demoIds, t],
  );

  return { loginEnabled, pending, isInDemo, toggle };
}
