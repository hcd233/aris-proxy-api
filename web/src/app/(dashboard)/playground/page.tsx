"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { api, ApiError } from "@/lib/api-client";
import { showErrorToast } from "@/lib/api-error-handler";
import { PermissionGuard } from "@/components/permission-guard";
import { useAuth } from "@/lib/auth-context";
import type { APIKeyItem } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { PageHeader } from "@/components/page-header";
import Link from "next/link";
import { Play, Plus, Square, Trash2 } from "lucide-react";
import { useT } from "@/lib/i18n";
import {
  buildChatBody,
  fetchAllPages,
  newMessage,
  ownedKeys,
  parseSSELine,
  selectableModels,
  type PlaygroundMessage,
  type PlaygroundModelOption,
} from "./playground-logic";

function PlaygroundPage() {
  const t = useT();
  const { user, isAdmin } = useAuth();
  const [models, setModels] = useState<PlaygroundModelOption[]>([]);
  const [model, setModel] = useState("");
  const [keys, setKeys] = useState<APIKeyItem[]>([]);
  const [keysLoaded, setKeysLoaded] = useState(false);
  const [apiKeyID, setApiKeyID] = useState<number | null>(null);
  const [messages, setMessages] = useState<PlaygroundMessage[]>([newMessage("user", "")]);
  const [stream, setStream] = useState(true);
  const [temperature, setTemperature] = useState("");
  const [maxTokens, setMaxTokens] = useState("");
  const [sending, setSending] = useState(false);
  const [output, setOutput] = useState("");
  const abortRef = useRef<AbortController | null>(null);

  const userId = user?.id;
  // 解析按用户租户隔离：管理员的模型列表含全部用户，只列本人名下的别名才能调通
  const ownerName = isAdmin() ? user?.name : undefined;

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
          selectableModels(modelItems.map((m) => ({ alias: m.alias, enabled: m.enabled }))),
        );
        const mine = ownedKeys(keyItems, userId);
        setKeys(mine);
        setApiKeyID((prev) => prev ?? mine[0]?.id ?? null);
      } catch (err) {
        if (!cancelled) showErrorToast(err, { title: t("playground.load_models_error") });
      } finally {
        if (!cancelled) setKeysLoaded(true);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [t, userId, ownerName]);

  // 离开页面时中断进行中的流，避免卸载后继续 setState 与上游空跑
  useEffect(() => () => abortRef.current?.abort(), []);

  const patchMessage = useCallback((idx: number, patch: Partial<PlaygroundMessage>) => {
    setMessages((prev) => prev.map((m, i) => (i === idx ? { ...m, ...patch } : m)));
  }, []);

  const handleStop = useCallback(() => abortRef.current?.abort(), []);

  const handleSend = useCallback(async () => {
    if (!model || apiKeyID === null || sending) return;
    const temp = temperature.trim() === "" ? undefined : Number(temperature);
    const maxTok = maxTokens.trim() === "" ? undefined : Number(maxTokens);
    const body = buildChatBody(messages, {
      model,
      stream,
      temperature: temp,
      maxTokens: maxTok,
    });
    setSending(true);
    setOutput("");
    try {
      if (stream) {
        const controller = new AbortController();
        abortRef.current = controller;
        await api.playgroundChatStream(
          apiKeyID,
          body,
          (line) => {
            const ev = parseSSELine(line);
            if (ev.type === "delta") setOutput((prev) => prev + ev.text);
          },
          controller.signal,
        );
      } else {
        const rsp = (await api.playgroundChat(apiKeyID, body)) as {
          choices?: { message?: { content?: string } }[];
        };
        setOutput(rsp.choices?.[0]?.message?.content ?? "");
      }
    } catch (err) {
      const aborted = err instanceof DOMException && err.name === "AbortError";
      if (!aborted && !(err instanceof ApiError && err.status === 401)) {
        showErrorToast(err, { title: t("playground.send_error") });
      }
    } finally {
      setSending(false);
      abortRef.current = null;
    }
  }, [model, apiKeyID, sending, messages, stream, temperature, maxTokens, t]);

  return (
    <div className="space-y-4">
      <PageHeader title={t("playground.title")} description={t("playground.description")} />

      <div className="grid gap-4 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle className="text-base">{t("playground.request")}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="space-y-1">
              <Label htmlFor="pg-model">{t("playground.model")}</Label>
              <Select value={model} onValueChange={(v) => setModel(String(v))}>
                <SelectTrigger id="pg-model" className="w-full">
                  <SelectValue placeholder={t("playground.model.placeholder")} />
                </SelectTrigger>
                <SelectContent>
                  {models.map((m) => (
                    <SelectItem key={m.alias} value={m.alias}>
                      {m.alias}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-1">
              <Label htmlFor="pg-apikey">{t("playground.api_key")}</Label>
              {keysLoaded && keys.length === 0 ? (
                <p className="text-sm text-muted-foreground">
                  {t("playground.api_key.empty")}{" "}
                  <Link href="/apikeys/" className="underline underline-offset-2">
                    {t("playground.api_key.create")}
                  </Link>
                </p>
              ) : (
                <Select
                  value={apiKeyID === null ? "" : String(apiKeyID)}
                  onValueChange={(v) => setApiKeyID(Number(v))}
                >
                  <SelectTrigger id="pg-apikey" className="w-full">
                    <SelectValue placeholder={t("playground.api_key.placeholder")} />
                  </SelectTrigger>
                  <SelectContent>
                    {keys.map((k) => (
                      <SelectItem key={k.id} value={String(k.id)}>
                        {k.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              )}
              <p className="text-[11px] text-muted-foreground">{t("playground.api_key.hint")}</p>
            </div>

            <div className="grid grid-cols-2 gap-2">
              <div className="space-y-1">
                <Label htmlFor="pg-temperature">{t("playground.temperature")}</Label>
                <Input
                  id="pg-temperature"
                  type="number"
                  min={0}
                  max={2}
                  step={0.1}
                  value={temperature}
                  onChange={(e) => setTemperature(e.target.value)}
                />
              </div>
              <div className="space-y-1">
                <Label htmlFor="pg-max-tokens">{t("playground.max_tokens")}</Label>
                <Input
                  id="pg-max-tokens"
                  type="number"
                  min={1}
                  value={maxTokens}
                  onChange={(e) => setMaxTokens(e.target.value)}
                />
              </div>
            </div>

            <div className="flex items-center gap-2">
              <Switch checked={stream} onCheckedChange={setStream} id="pg-stream" />
              <Label htmlFor="pg-stream">{t("playground.stream")}</Label>
            </div>

            <div className="space-y-2">
              <Label>{t("playground.messages")}</Label>
              {messages.map((m, idx) => (
                <div key={idx} className="flex items-start gap-1.5">
                  <Select
                    value={m.role}
                    onValueChange={(v) =>
                      patchMessage(idx, { role: v as PlaygroundMessage["role"] })
                    }
                  >
                    <SelectTrigger className="w-28 shrink-0">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="system">system</SelectItem>
                      <SelectItem value="user">user</SelectItem>
                      <SelectItem value="assistant">assistant</SelectItem>
                    </SelectContent>
                  </Select>
                  <Input
                    value={m.content}
                    placeholder={t("playground.message.placeholder")}
                    onChange={(e) => patchMessage(idx, { content: e.target.value })}
                  />
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon-sm"
                    aria-label={t("playground.message.delete")}
                    disabled={messages.length === 1}
                    onClick={() => setMessages((prev) => prev.filter((_, i) => i !== idx))}
                  >
                    <Trash2 className="size-3.5 text-muted-foreground" />
                  </Button>
                </div>
              ))}
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => setMessages((prev) => [...prev, newMessage("user", "")])}
              >
                <Plus className="size-4" />
                {t("playground.message.add")}
              </Button>
            </div>

            {sending && stream ? (
              <Button onClick={handleStop} variant="outline" className="w-full">
                <Square className="size-4" />
                {t("playground.stop")}
              </Button>
            ) : (
              <Button
                onClick={handleSend}
                disabled={!model || apiKeyID === null || sending}
                className="w-full"
              >
                <Play className="size-4" />
                {sending ? t("playground.sending") : t("playground.send")}
              </Button>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="text-base">{t("playground.output")}</CardTitle>
          </CardHeader>
          <CardContent>
            <pre className="min-h-40 whitespace-pre-wrap rounded-md bg-muted p-3 font-mono text-sm">
              {output || t("playground.output.placeholder")}
            </pre>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}

export default function PlaygroundPageGuarded() {
  return (
    <PermissionGuard>
      <PlaygroundPage />
    </PermissionGuard>
  );
}
