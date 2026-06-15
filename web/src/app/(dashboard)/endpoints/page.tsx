"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { usePersistentState } from "@/hooks/use-persistent-state";
import { api } from "@/lib/api-client";
import { PermissionGuard } from "@/components/permission-guard";
import type { EndpointItem, PageInfo } from "@/lib/types";
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
import { Badge } from "@/components/ui/badge";
import { TooltipProvider, TooltipRoot, TooltipTrigger, TooltipContent } from "@/components/ui/tooltip";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Plus, Trash2, Pencil, Server, AlertTriangle, ChevronLeft, ChevronRight, Search, ListFilter, Check } from "lucide-react";
import { ProviderIcon } from "@/components/provider-icon";
import { toast } from "sonner";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useIsMobile } from "@/hooks/use-mobile";

interface EndpointForm {
  name: string;
  openaiBaseURL: string;
  anthropicBaseURL: string;
  apiKey: string;
  supportOpenAIChatCompletion: boolean;
  supportOpenAIResponse: boolean;
  supportAnthropicMessage: boolean;
}

const emptyForm: EndpointForm = {
  name: "",
  openaiBaseURL: "",
  anthropicBaseURL: "",
  apiKey: "",
  supportOpenAIChatCompletion: true,
  supportOpenAIResponse: false,
  supportAnthropicMessage: false,
};

export default function EndpointsPage() {
  const isMobile = useIsMobile();
  const [endpoints, setEndpoints] = useState<EndpointItem[]>([]);
  const [persistedPage, setPersistedPage] = usePersistentState("dashboard.endpoints.page", 1);
  const [persistedPageSize, setPersistedPageSize] = usePersistentState("dashboard.endpoints.pageSize", 20);
  const [pageInfo, setPageInfo] = useState<PageInfo>({ page: persistedPage, pageSize: persistedPageSize, total: 0 });
  const [loading, setLoading] = useState(true);
  const [searchQuery, setSearchQuery] = useState("");
  const [pageInputValue, setPageInputValue] = useState(String(persistedPage));
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [form, setForm] = useState<EndpointForm>(emptyForm);
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState<number | null>(null);
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<{ id: number; name: string } | null>(null);

  const fetchEndpoints = useCallback(async (page: number, pageSize: number, query?: string) => {
    const validSizes = [20, 50, 100];
    const safeSize = validSizes.includes(pageSize) ? pageSize : 20;
    setLoading(true);
    try {
      const rsp = await api.listEndpoints(page, safeSize, query);
      setEndpoints(rsp.endpoints ?? []);
      if (rsp.pageInfo) {
        setPageInfo(rsp.pageInfo);
        setPageInputValue(String(rsp.pageInfo.page));
        setPersistedPage(rsp.pageInfo.page);
        if (validSizes.includes(rsp.pageInfo.pageSize)) {
          setPersistedPageSize(rsp.pageInfo.pageSize);
        }
      }
    } catch {
      toast.error("Failed to load endpoints");
    } finally {
      setLoading(false);
    }
  }, [setPersistedPage, setPersistedPageSize]);

  /* eslint-disable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps -- Data fetching requires setting state from async effects on mount */
  useEffect(() => {
    fetchEndpoints(persistedPage, persistedPageSize);
  }, [fetchEndpoints]);
  /* eslint-enable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps */

  const totalPages = useMemo(
    () => Math.max(1, Math.ceil(pageInfo.total / pageInfo.pageSize)),
    [pageInfo],
  );

  const refresh = (page: number, pageSize?: number) =>
    fetchEndpoints(page, pageSize ?? pageInfo.pageSize, searchQuery || undefined);

  const openCreate = () => {
    setEditingId(null);
    setForm(emptyForm);
    setDialogOpen(true);
  };

  const openEdit = (ep: EndpointItem) => {
    setEditingId(ep.id);
    setForm({
      name: ep.name,
      openaiBaseURL: ep.openaiBaseURL,
      anthropicBaseURL: ep.anthropicBaseURL,
      apiKey: "",
      supportOpenAIChatCompletion: ep.supportOpenAIChatCompletion,
      supportOpenAIResponse: ep.supportOpenAIResponse,
      supportAnthropicMessage: ep.supportAnthropicMessage,
    });
    setDialogOpen(true);
  };

  const handleSave = async () => {
    if (!form.name.trim()) {
      toast.error("Name is required");
      return;
    }
    setSaving(true);
    try {
      if (editingId) {
        await api.updateEndpoint(editingId, {
          name: form.name,
          openaiBaseURL: form.openaiBaseURL || undefined,
          anthropicBaseURL: form.anthropicBaseURL || undefined,
          apiKey: form.apiKey || undefined,
          supportOpenAIChatCompletion: form.supportOpenAIChatCompletion,
          supportOpenAIResponse: form.supportOpenAIResponse,
          supportAnthropicMessage: form.supportAnthropicMessage,
        });
        toast.success("Endpoint updated");
      } else {
        await api.createEndpoint({
          name: form.name,
          openaiBaseURL: form.openaiBaseURL || undefined,
          anthropicBaseURL: form.anthropicBaseURL || undefined,
          apiKey: form.apiKey,
          supportOpenAIChatCompletion: form.supportOpenAIChatCompletion,
          supportOpenAIResponse: form.supportOpenAIResponse,
          supportAnthropicMessage: form.supportAnthropicMessage,
        });
        toast.success("Endpoint created");
      }
      setDialogOpen(false);
      fetchEndpoints(pageInfo.page, pageInfo.pageSize, searchQuery || undefined);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to save endpoint");
    } finally {
      setSaving(false);
    }
  };

  const openDeleteConfirm = (ep: EndpointItem) => {
    setDeleteTarget({ id: ep.id, name: ep.name });
    setDeleteConfirmOpen(true);
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;
    setDeleting(deleteTarget.id);
    try {
      await api.deleteEndpoint(deleteTarget.id);
      toast.success("Endpoint deleted");
      fetchEndpoints(pageInfo.page, pageInfo.pageSize, searchQuery || undefined);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to delete endpoint");
    } finally {
      setDeleting(null);
      setDeleteConfirmOpen(false);
      setDeleteTarget(null);
    }
  };

  return (
    <PermissionGuard adminOnly>
      <div className="space-y-8">
        <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
          <div>
            <h1 className="font-display text-2xl md:text-3xl font-semibold tracking-tight text-foreground">Endpoints</h1>
            <p className="mt-1.5 text-sm text-muted-foreground">
              Manage LLM provider endpoints
            </p>
          </div>
          <Button onClick={openCreate}>
            <Plus className="mr-1 size-4" />
            Create Endpoint
          </Button>
        </div>

        <Card>
          <CardHeader>
            <CardTitle className="font-display">All Endpoints</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="mb-4">
              <div className="relative w-full md:max-w-sm">
                <Search className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  placeholder="Search endpoints..."
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") fetchEndpoints(1, pageInfo.pageSize, searchQuery || undefined);
                  }}
                  className="pl-9"
                />
              </div>
            </div>
            {loading ? (
              <div className="space-y-3">
                {Array.from({ length: 3 }).map((_, i) => (
                  <Skeleton key={i} className="h-12 w-full" />
                ))}
              </div>
            ) : endpoints.length === 0 ? (
              <div className="flex flex-col items-center justify-center py-12 text-center">
                <Server className="mb-3 size-10 text-muted-foreground/40" />
                <p className="text-sm text-muted-foreground">
                  No endpoints configured. Create one to get started.
                </p>
              </div>
            ) : (
              <>
                {isMobile ? (
                  <div className="space-y-3">
                    {endpoints.map((ep) => (
                      <div
                        key={ep.id}
                        className="rounded-lg border border-border bg-card p-4"
                      >
                        <div className="flex items-start justify-between gap-3">
                          <div className="min-w-0 flex-1">
                            <p className="text-sm font-medium">{ep.name}</p>
                            <p className="mt-0.5 font-mono text-[11px] text-muted-foreground">
                              Key: <code className="rounded bg-muted px-1 py-0.5">{ep.maskedAPIKey}</code>
                            </p>
                            {ep.openaiBaseURL && (
                              <p className="mt-0.5 truncate font-mono text-xs text-muted-foreground">
                                O: {ep.openaiBaseURL}
                              </p>
                            )}
                            {ep.anthropicBaseURL && (
                              <p className="mt-0.5 truncate font-mono text-xs text-muted-foreground">
                                A: {ep.anthropicBaseURL}
                              </p>
                            )}
                          </div>
                          <div className="flex items-center gap-1">
                            <Button variant="ghost" size="icon-sm" onClick={() => openEdit(ep)} className="text-muted-foreground hover:text-foreground">
                              <Pencil size={14} />
                            </Button>
                            <Button
                              variant="destructive"
                              size="xs"
                              disabled={deleting === ep.id}
                              onClick={() => openDeleteConfirm(ep)}
                            >
                              <Trash2 className="mr-1 size-3" />
                              Delete
                            </Button>
                          </div>
                        </div>
                        <div className="mt-2 flex flex-wrap gap-1.5">
                          {ep.supportOpenAIChatCompletion && (
                            <Badge variant="secondary" className="gap-1.5 text-[11px] font-normal">
                              <ProviderIcon protocol="openai-chat-completion" size={14} />
                              OpenAI / Chat
                            </Badge>
                          )}
                          {ep.supportOpenAIResponse && (
                            <Badge variant="secondary" className="gap-1.5 text-[11px] font-normal">
                              <ProviderIcon protocol="openai-response" size={14} />
                              OpenAI / Response
                            </Badge>
                          )}
                          {ep.supportAnthropicMessage && (
                            <Badge variant="secondary" className="gap-1.5 text-[11px] font-normal">
                              <ProviderIcon protocol="anthropic-message" size={14} />
                              Anthropic / Messages
                            </Badge>
                          )}
                        </div>
                        <p className="mt-2 text-xs text-muted-foreground">
                          Created {new Date(ep.createdAt).toLocaleDateString()}
                        </p>
                      </div>
                    ))}
                  </div>
                ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Name</TableHead>
                      <TableHead>Key</TableHead>
                      <TableHead>Base URL</TableHead>
                      <TableHead>Supported APIs</TableHead>
                      <TableHead>Created</TableHead>
                      <TableHead className="text-right">Actions</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {endpoints.map((ep) => (
                      <TableRow key={ep.id}>
                        <TableCell className="font-medium">{ep.name}</TableCell>
                        <TableCell>
                          <code className="rounded bg-muted px-1.5 py-0.5 text-[11px] font-mono">{ep.maskedAPIKey}</code>
                        </TableCell>
                        <TableCell className="max-w-[400px]">
                          <TooltipProvider>
                            <TooltipRoot>
                              <TooltipTrigger
                                render={
                                  <button type="button" className="w-full cursor-default text-left">
                                    <div className="space-y-0.5">
                                      <div className="flex items-center gap-1.5 truncate font-mono text-xs text-muted-foreground">
                                        <ProviderIcon protocol="openai-chat-completion" size={14} />
                                        {ep.openaiBaseURL || "—"}
                                      </div>
                                      <div className="flex items-center gap-1.5 truncate font-mono text-xs text-muted-foreground">
                                        <ProviderIcon protocol="anthropic-message" size={14} />
                                        {ep.anthropicBaseURL || "—"}
                                      </div>
                                    </div>
                                  </button>
                                }
                              />
                              <TooltipContent side="top" align="start" className="max-w-md">
                                <div className="space-y-1 font-mono text-xs">
                                  <p className="flex items-center gap-1.5">
                                    <ProviderIcon protocol="openai-chat-completion" size={14} />
                                    {ep.openaiBaseURL || "—"}
                                  </p>
                                  <p className="flex items-center gap-1.5">
                                    <ProviderIcon protocol="anthropic-message" size={14} />
                                    {ep.anthropicBaseURL || "—"}
                                  </p>
                                </div>
                              </TooltipContent>
                            </TooltipRoot>
                          </TooltipProvider>
                        </TableCell>
                        <TableCell>
                          <div className="flex flex-wrap gap-1.5">
                            {ep.supportOpenAIChatCompletion && (
                              <Badge variant="secondary" className="gap-1.5 text-[11px] font-normal">
                                <ProviderIcon protocol="openai-chat-completion" size={14} />
                                Chat Completions
                              </Badge>
                            )}
                            {ep.supportOpenAIResponse && (
                              <Badge variant="secondary" className="gap-1.5 text-[11px] font-normal">
                                <ProviderIcon protocol="openai-response" size={14} />
                                Response
                              </Badge>
                            )}
                            {ep.supportAnthropicMessage && (
                              <Badge variant="secondary" className="gap-1.5 text-[11px] font-normal">
                                <ProviderIcon protocol="anthropic-message" size={14} />
                                Messages
                              </Badge>
                            )}
                          </div>
                        </TableCell>
                        <TableCell className="text-muted-foreground">
                          {new Date(ep.createdAt).toLocaleDateString()}
                        </TableCell>
                        <TableCell className="text-right">
                          <div className="flex items-center justify-end gap-1">
                            <Button variant="ghost" size="icon-sm" onClick={() => openEdit(ep)} className="text-muted-foreground hover:text-foreground">
                              <Pencil size={14} />
                            </Button>
                            <Button
                              variant="destructive"
                              size="xs"
                              disabled={deleting === ep.id}
                              onClick={() => openDeleteConfirm(ep)}
                            >
                              <Trash2 className="mr-1 size-3" />
                              Delete
                            </Button>
                          </div>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
                )}

                {pageInfo.total > 0 && (
                  <div className="mt-4 flex flex-wrap items-center justify-between gap-4">
                    <div className="hidden items-center gap-3 md:flex">
                      <DropdownMenu>
                        <DropdownMenuTrigger
                          render={<Button variant="outline" size="sm" className="gap-1.5" />}
                        >
                          <ListFilter size={14} />
                          {pageInfo.pageSize} / page
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="start">
                          {[20, 50, 100].map((size) => (
                            <DropdownMenuItem key={size} onClick={() => refresh(1, size)}>
                              {size === pageInfo.pageSize && <Check className="size-4" />}
                              <span className={size === pageInfo.pageSize ? "ml-0" : "ml-6"}>
                                {size} per page
                              </span>
                            </DropdownMenuItem>
                          ))}
                        </DropdownMenuContent>
                      </DropdownMenu>
                      <p className="hidden text-sm text-muted-foreground md:block">
                        {pageInfo.total} endpoint{pageInfo.total !== 1 ? "s" : ""} total
                      </p>
                    </div>
                    <div className="flex items-center gap-2">
                      <Button
                        variant="outline"
                        size="sm"
                        disabled={pageInfo.page <= 1}
                        onClick={() => fetchEndpoints(pageInfo.page - 1, pageInfo.pageSize, searchQuery || undefined)}
                      >
                        <ChevronLeft className="size-4" />
                      </Button>
                      <div className="flex items-center gap-1.5 text-sm">
                        <span className="text-muted-foreground">Page</span>
                        <input
                          type="number"
                          min={1}
                          max={totalPages}
                          value={pageInputValue}
                          onChange={(e) => setPageInputValue(e.target.value)}
                          className="h-8 w-14 rounded-md border border-input bg-transparent px-2 py-1 text-center text-sm tabular-nums focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring/40 focus-visible:outline-none dark:bg-input/30"
                          onKeyDown={(e) => {
                            if (e.key === "Enter") {
                              let page = parseInt(pageInputValue, 10);
                              if (Number.isNaN(page)) page = 1;
                              page = Math.max(1, Math.min(page, totalPages));
                              refresh(page);
                            }
                          }}
                          onBlur={() => {
                            let page = parseInt(pageInputValue, 10);
                            if (Number.isNaN(page)) page = 1;
                            page = Math.max(1, Math.min(page, totalPages));
                            refresh(page);
                          }}
                        />
                        <span className="text-muted-foreground">/ {totalPages}</span>
                      </div>
                      <Button
                        variant="outline"
                        size="sm"
                        disabled={pageInfo.page >= totalPages}
                        onClick={() => fetchEndpoints(pageInfo.page + 1, pageInfo.pageSize, searchQuery || undefined)}
                      >
                        <ChevronRight className="size-4" />
                      </Button>
                    </div>
                  </div>
                )}
              </>
            )}
          </CardContent>
        </Card>

        <AlertDialog open={deleteConfirmOpen} onOpenChange={setDeleteConfirmOpen}>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle className="flex items-center gap-2">
                <AlertTriangle className="size-5 text-destructive" />
                Are you sure?
              </AlertDialogTitle>
              <AlertDialogDescription>
                This will permanently delete <strong>{deleteTarget?.name}</strong>. This action cannot be undone.
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>Cancel</AlertDialogCancel>
              <AlertDialogAction variant="destructive" onClick={handleDelete} disabled={deleting !== null}>
                {deleting !== null ? "Deleting..." : "Delete"}
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>

        <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
          <DialogContent className="sm:max-w-md">
            <DialogHeader>
              <DialogTitle>
                {editingId ? "Edit Endpoint" : "Create Endpoint"}
              </DialogTitle>
              <DialogDescription>
                {editingId
                  ? "Update endpoint configuration."
                  : "Add a new LLM provider endpoint."}
              </DialogDescription>
            </DialogHeader>
            <div className="space-y-3">
              <div className="space-y-1">
                <Label htmlFor="ep-name">Name</Label>
                <Input
                  id="ep-name"
                  placeholder="e.g. OpenAI Production"
                  value={form.name}
                  onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
                />
              </div>
              <div className="space-y-1">
                <Label htmlFor="ep-openai-url">OpenAI Base URL</Label>
                <Input
                  id="ep-openai-url"
                  placeholder="https://api.openai.com/v1"
                  value={form.openaiBaseURL}
                  onChange={(e) => setForm((f) => ({ ...f, openaiBaseURL: e.target.value }))}
                />
              </div>
              <div className="space-y-1">
                <Label htmlFor="ep-anthropic-url">Anthropic Base URL</Label>
                <Input
                  id="ep-anthropic-url"
                  placeholder="https://api.anthropic.com"
                  value={form.anthropicBaseURL}
                  onChange={(e) => setForm((f) => ({ ...f, anthropicBaseURL: e.target.value }))}
                />
              </div>
              <div className="space-y-1">
                <Label htmlFor="ep-apikey">API Key</Label>
                <Input
                  id="ep-apikey"
                  type="password"
                  placeholder={editingId ? "Leave empty to keep current" : "Enter API key"}
                  value={form.apiKey}
                  onChange={(e) => setForm((f) => ({ ...f, apiKey: e.target.value }))}
                />
              </div>
              <div className="space-y-2">
                <Label>Capabilities</Label>
                <div className="flex flex-col gap-2">
                  <label className="flex items-center gap-2 text-sm">
                    <input
                      type="checkbox"
                      checked={form.supportOpenAIChatCompletion}
                      onChange={(e) =>
                        setForm((f) => ({ ...f, supportOpenAIChatCompletion: e.target.checked }))
                      }
                      className="rounded"
                    />
                    OpenAI Chat Completion
                  </label>
                  <label className="flex items-center gap-2 text-sm">
                    <input
                      type="checkbox"
                      checked={form.supportOpenAIResponse}
                      onChange={(e) =>
                        setForm((f) => ({ ...f, supportOpenAIResponse: e.target.checked }))
                      }
                      className="rounded"
                    />
                    OpenAI Response API
                  </label>
                  <label className="flex items-center gap-2 text-sm">
                    <input
                      type="checkbox"
                      checked={form.supportAnthropicMessage}
                      onChange={(e) =>
                        setForm((f) => ({ ...f, supportAnthropicMessage: e.target.checked }))
                      }
                      className="rounded"
                    />
                    Anthropic Messages
                  </label>
                </div>
              </div>
            </div>
            <DialogFooter>
              <Button variant="outline" onClick={() => setDialogOpen(false)}>
                Cancel
              </Button>
              <Button onClick={handleSave} disabled={!form.name.trim() || saving}>
                {saving ? "Saving..." : editingId ? "Update" : "Create"}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>
    </PermissionGuard>
  );
}