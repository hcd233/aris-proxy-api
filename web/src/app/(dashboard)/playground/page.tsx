"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { api, ApiError } from "@/lib/api-client";
import { PermissionGuard } from "@/components/permission-guard";
import { useAuth } from "@/lib/auth-context";
import type { APIKeyItem } from "@/lib/types";
import { LocaleFade } from "@/components/locale-fade";
import { PageHeader } from "@/components/page-header";
import { Button } from "@/components/ui/button";
import { Sheet, SheetContent, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { PanelLeft } from "lucide-react";
import { useT } from "@/lib/i18n";
import { toast } from "sonner";
import {
  buildChatBody,
  deriveTitle,
  estimateCost,
  fetchAllPages,
  loadParams,
  loadSessions,
  newMessage,
  newSession,
  ownedKeys,
  parseSSELine,
  saveParams,
  saveSessions,
  selectableModels,
  truncateAfter,
  upsertSession,
  type ChatUsage,
  type PlaygroundMessage,
  type PlaygroundModelOption,
  type PlaygroundParams,
  type PlaygroundSession,
  type PlaygroundTurnMeta,
} from "./playground-logic";
import { Composer } from "@/components/playground/composer";
import { ConversationView } from "@/components/playground/conversation-view";
import { ParamsDrawer } from "@/components/playground/params-drawer";
import { SessionSidebar } from "@/components/playground/session-sidebar";

const DEFAULT_PARAMS: PlaygroundParams = {
  model: "",
  apiKeyID: null,
  temperature: "",
  maxTokens: "",
  stream: true,
};

function PlaygroundPage() {
  const t = useT();
  const { user, isAdmin } = useAuth();
  const userId = user?.id;
  // 解析按用户租户隔离：管理员的模型列表含全部用户，只列本人名下的别名才能调通
  const ownerName = isAdmin() ? user?.name : undefined;

  const [sessions, setSessions] = useState<PlaygroundSession[]>([]);
  const [activeId, setActiveId] = useState<string | null>(null);
  const [params, setParams] = useState<PlaygroundParams>(DEFAULT_PARAMS);
  const [models, setModels] = useState<PlaygroundModelOption[]>([]);
  const [keys, setKeys] = useState<APIKeyItem[]>([]);
  const [keysLoaded, setKeysLoaded] = useState(false);
  const [sending, setSending] = useState(false);
  const [paramsOpen, setParamsOpen] = useState(false);
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const abortRef = useRef<AbortController | null>(null);

  // 本地会话/参数初始化（一次）
  /* eslint-disable react-hooks/set-state-in-effect -- Reading localStorage requires setting state in effect on mount */
  useEffect(() => {
    const loaded = loadSessions();
    const next = loaded.length > 0 ? loaded : [newSession()];
    setSessions(next);
    setActiveId(next[0].id);
    setParams({ ...DEFAULT_PARAMS, ...(loadParams() ?? {}) });
  }, []);
  /* eslint-enable react-hooks/set-state-in-effect */

  // 模型与 Key 拉取（分页拉全量 + 租户隔离）
  useEffect(() => {
    if (userId === undefined) return;
    let cancelled = false;
    (async () => {
      try {
        const [modelItems, keyItems] = await Promise.all([
          fetchAllPages(async (page, pageSize) => {
            const rsp = await api.listModelsPage({
              page,
              pageSize,
              status: "enabled",
              username: ownerName,
            });
            return { items: rsp.items ?? [], total: rsp.pageInfo?.total ?? 0 };
          }),
          fetchAllPages(async (page, pageSize) => {
            const rsp = await api.listAPIKeys(page, pageSize);
            return { items: rsp.keys ?? [], total: rsp.pageInfo?.total ?? 0 };
          }),
        ]);
        if (cancelled) return;
        setModels(
          selectableModels(
            modelItems.map((m) => ({ alias: m.alias, enabled: m.enabled, pricing: m.pricing })),
          ),
        );
        const mine = ownedKeys(keyItems, userId);
        setKeys(mine);
        setParams((prev) => ({ ...prev, apiKeyID: prev.apiKeyID ?? mine[0]?.id ?? null }));
      } catch (err) {
        if (!cancelled) {
          toast.error(t("playground.load_models_error"));
          console.error(err);
        }
      } finally {
        if (!cancelled) setKeysLoaded(true);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [t, userId, ownerName]);

  // 离开页面时中断进行中的流
  useEffect(() => () => abortRef.current?.abort(), []);

  const persist = useCallback(
    (next: PlaygroundSession[]) => {
      if (!saveSessions(next)) toast.error(t("playground.session.save_failed"));
    },
    [t],
  );

  const patchActive = useCallback(
    (patch: (s: PlaygroundSession) => PlaygroundSession) => {
      setSessions((prev) => prev.map((s) => (s.id === activeId ? patch(s) : s)));
    },
    [activeId],
  );

  const patchParams = useCallback((patch: Partial<PlaygroundParams>) => {
    setParams((prev) => {
      const next = { ...prev, ...patch };
      saveParams(next);
      return next;
    });
  }, []);

  /** 以给定上下文发起一次生成；终态统一写 meta 并持久化 */
  const runGenerate = useCallback(
    async (context: PlaygroundMessage[]) => {
      if (!params.model || params.apiKeyID === null) return;
      const pricing = models.find((m) => m.alias === params.model)?.pricing;
      const temp = params.temperature.trim() === "" ? undefined : Number(params.temperature);
      const maxTok = params.maxTokens.trim() === "" ? undefined : Number(params.maxTokens);
      const body = {
        ...buildChatBody(context, {
          model: params.model,
          stream: params.stream,
          temperature: temp,
          maxTokens: maxTok,
        }),
        stream_options: { include_usage: true },
      };

      const placeholder = newMessage("assistant", "");
      setSessions((prev) =>
        prev.map((s) => (s.id === activeId ? { ...s, messages: [...context, placeholder] } : s)),
      );
      setSending(true);
      const startAt = Date.now();
      let firstTokenMs: number | undefined;
      let usage: ChatUsage | undefined;
      let pending = "";
      let raf = 0;
      const flush = () => {
        raf = 0;
        const text = pending;
        patchActive((s) => ({
          ...s,
          messages: s.messages.map((m) => (m.id === placeholder.id ? { ...m, content: text } : m)),
        }));
      };
      const finalize = (extra: Partial<PlaygroundTurnMeta>) => {
        if (raf) cancelAnimationFrame(raf);
        const meta: PlaygroundTurnMeta = {
          model: params.model,
          usage,
          firstTokenMs,
          totalMs: Date.now() - startAt,
          cost: usage ? estimateCost(usage, pricing) : null,
          requestSnapshot: body,
          ...extra,
        };
        setSessions((prev) => {
          const next = prev.map((s) =>
            s.id === activeId
              ? {
                  ...s,
                  messages: s.messages.map((m) =>
                    m.id === placeholder.id ? { ...m, content: pending, meta } : m,
                  ),
                }
              : s,
          );
          persist(next);
          return next;
        });
        setSending(false);
        abortRef.current = null;
      };

      try {
        if (params.stream) {
          const controller = new AbortController();
          abortRef.current = controller;
          await api.playgroundChatStream(
            params.apiKeyID,
            body,
            (line) => {
              const ev = parseSSELine(line);
              if (ev.type === "delta") {
                if (firstTokenMs === undefined) firstTokenMs = Date.now() - startAt;
                pending += ev.text;
                if (!raf) raf = requestAnimationFrame(flush);
              } else if (ev.type === "usage") {
                usage = ev.usage;
              }
            },
            controller.signal,
          );
          finalize({});
        } else {
          const rsp = (await api.playgroundChat(params.apiKeyID, body)) as {
            choices?: { message?: { content?: string } }[];
            usage?: {
              prompt_tokens?: number;
              completion_tokens?: number;
              prompt_tokens_details?: { cached_tokens?: number };
            };
          };
          pending = rsp.choices?.[0]?.message?.content ?? "";
          if (rsp.usage) {
            usage = {
              promptTokens: rsp.usage.prompt_tokens ?? 0,
              completionTokens: rsp.usage.completion_tokens ?? 0,
              cachedTokens: rsp.usage.prompt_tokens_details?.cached_tokens ?? 0,
            };
          }
          finalize({});
        }
      } catch (err) {
        // 401 交给 api-client 全局刷新不展示；AbortError 标中断；其余内联错误卡
        const aborted = err instanceof DOMException && err.name === "AbortError";
        if (aborted) {
          finalize({ interrupted: true });
        } else if (err instanceof ApiError && err.status === 401) {
          finalize({});
        } else {
          finalize({
            error: {
              status: err instanceof ApiError ? err.status : undefined,
              message: err instanceof Error ? err.message : String(err),
            },
          });
        }
      }
    },
    [activeId, models, params, patchActive, persist],
  );

  const handleSend = useCallback(
    (content: string) => {
      if (!activeId || sending) return;
      const session = sessions.find((s) => s.id === activeId);
      if (!session) return;
      const messages = [...session.messages, newMessage("user", content)];
      const title = session.title || deriveTitle(messages);
      const next = sessions.map((s) => (s.id === activeId ? { ...s, messages, title } : s));
      setSessions(next);
      persist(next);
      void runGenerate(messages);
    },
    [activeId, sending, sessions, persist, runGenerate],
  );

  const handleEditSave = useCallback(
    (index: number, content: string) => {
      const session = sessions.find((s) => s.id === activeId);
      if (!session) return;
      const edited = truncateAfter(session.messages, index).map((m, i) =>
        i === index ? { ...m, content, meta: undefined } : m,
      );
      const next = sessions.map((s) =>
        s.id === activeId ? { ...s, messages: edited, title: deriveTitle(edited) } : s,
      );
      setSessions(next);
      persist(next);
      void runGenerate(edited);
    },
    [activeId, sessions, persist, runGenerate],
  );

  const handleRerun = useCallback(
    (index: number) => {
      const session = sessions.find((s) => s.id === activeId);
      if (!session) return;
      const kept = session.messages.filter((_, i) => i !== index);
      const next = sessions.map((s) => (s.id === activeId ? { ...s, messages: kept } : s));
      setSessions(next);
      persist(next);
      void runGenerate(kept);
    },
    [activeId, sessions, persist, runGenerate],
  );

  const handleDelete = useCallback(
    (index: number) => {
      const session = sessions.find((s) => s.id === activeId);
      if (!session) return;
      const kept = session.messages.filter((_, i) => i !== index);
      const next = sessions.map((s) => (s.id === activeId ? { ...s, messages: kept } : s));
      setSessions(next);
      persist(next);
    },
    [activeId, sessions, persist],
  );

  const handleRetry = handleRerun;

  const handleInsertRole = useCallback(
    (role: "system" | "assistant") => {
      patchActive((s) => ({ ...s, messages: [...s.messages, newMessage(role, "")] }));
    },
    [patchActive],
  );

  const handleNewSession = useCallback(() => {
    const s = newSession();
    setSessions((prev) => {
      const next = upsertSession(prev, s);
      persist(next);
      return next;
    });
    setActiveId(s.id);
    setSidebarOpen(false);
  }, [persist]);

  const handleSelectSession = useCallback((id: string) => {
    setActiveId(id);
    setSidebarOpen(false);
  }, []);

  const handleRenameSession = useCallback(
    (id: string, title: string) => {
      setSessions((prev) => {
        const next = prev.map((s) => (s.id === id ? { ...s, title } : s));
        persist(next);
        return next;
      });
    },
    [persist],
  );

  const handleDeleteSession = useCallback(
    (id: string) => {
      setSessions((prev) => {
        const rest = prev.filter((s) => s.id !== id);
        const next = rest.length > 0 ? rest : [newSession()];
        persist(next);
        return next;
      });
    },
    [persist],
  );

  // 活动会话兜底：被删后回退到首项（删除时保底重建，不会为空）
  const active = sessions.find((s) => s.id === activeId) ?? sessions[0];
  if (!active) return null;

  return (
    <LocaleFade>
      <div className="space-y-4">
        <PageHeader title={t("playground.title")} description={t("playground.description")} />
        <div className="flex h-[calc(100dvh-13rem)] gap-4">
          {/* 会话侧栏：lg+ 常驻；< lg 走 Sheet 抽屉 */}
          <aside className="hidden w-60 shrink-0 lg:block">
            <SessionSidebar
              sessions={sessions}
              activeId={active.id}
              onSelect={handleSelectSession}
              onNew={handleNewSession}
              onRename={handleRenameSession}
              onDelete={handleDeleteSession}
            />
          </aside>
          {/* 对话主区 */}
          <div className="flex min-w-0 flex-1 flex-col">
            <div className="flex items-center gap-2 lg:hidden">
              <Button
                variant="outline"
                size="icon-sm"
                aria-label={t("playground.session.open")}
                onClick={() => setSidebarOpen(true)}
              >
                <PanelLeft className="size-4" />
              </Button>
            </div>
            <ConversationView
              messages={active.messages}
              sending={sending}
              onEditSave={handleEditSave}
              onDelete={handleDelete}
              onRerun={handleRerun}
              onRetry={handleRetry}
            />
            <Composer
              model={params.model}
              stream={params.stream}
              sending={sending}
              disabled={!params.model || params.apiKeyID === null}
              onSend={handleSend}
              onStop={() => abortRef.current?.abort()}
              onInsertRole={handleInsertRole}
              onOpenParams={() => setParamsOpen(true)}
            />
          </div>
        </div>
        <ParamsDrawer
          open={paramsOpen}
          onOpenChange={setParamsOpen}
          models={models}
          keys={keys}
          keysLoaded={keysLoaded}
          params={params}
          onParamsChange={patchParams}
        />
        {/* < lg 会话抽屉 */}
        <Sheet open={sidebarOpen} onOpenChange={setSidebarOpen}>
          <SheetContent side="left" className="w-60 sm:max-w-[15rem]">
            <SheetHeader>
              <SheetTitle>{t("playground.session.open")}</SheetTitle>
            </SheetHeader>
            <SessionSidebar
              sessions={sessions}
              activeId={active.id}
              onSelect={handleSelectSession}
              onNew={handleNewSession}
              onRename={handleRenameSession}
              onDelete={handleDeleteSession}
            />
          </SheetContent>
        </Sheet>
      </div>
    </LocaleFade>
  );
}

export default function PlaygroundPageGuarded() {
  return (
    <PermissionGuard>
      <PlaygroundPage />
    </PermissionGuard>
  );
}
