"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api-client";
import { PermissionGuard } from "@/components/permission-guard";
import type { EndpointItem } from "@/lib/types";
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
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Plus, Trash2, Pencil, Server } from "lucide-react";
import { toast } from "sonner";

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
  const [endpoints, setEndpoints] = useState<EndpointItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [form, setForm] = useState<EndpointForm>(emptyForm);
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState<number | null>(null);

  const fetchEndpoints = useCallback(async () => {
    setLoading(true);
    try {
      const rsp = await api.listEndpoints();
      setEndpoints(rsp.endpoints ?? []);
    } catch {
      toast.error("Failed to load endpoints");
    } finally {
      setLoading(false);
    }
  }, []);

  /* eslint-disable react-hooks/set-state-in-effect -- Data fetching requires setting state from async effects on mount */
  useEffect(() => {
    fetchEndpoints();
  }, [fetchEndpoints]);
  /* eslint-enable react-hooks/set-state-in-effect */

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
      fetchEndpoints();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to save endpoint");
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (id: number) => {
    setDeleting(id);
    try {
      await api.deleteEndpoint(id);
      toast.success("Endpoint deleted");
      fetchEndpoints();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to delete endpoint");
    } finally {
      setDeleting(null);
    }
  };

  return (
    <PermissionGuard adminOnly>
      <div className="space-y-6">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-bold tracking-tight">Endpoints</h1>
            <p className="text-sm text-muted-foreground">
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
            <CardTitle>All Endpoints</CardTitle>
          </CardHeader>
          <CardContent>
            {loading ? (
              <div className="space-y-3">
                {Array.from({ length: 3 }).map((_, i) => (
                  <Skeleton key={i} className="h-10 w-full" />
                ))}
              </div>
            ) : endpoints.length === 0 ? (
              <div className="flex flex-col items-center justify-center py-12 text-center">
                <Server className="mb-3 size-10 text-muted-foreground/50" />
                <p className="text-sm text-muted-foreground">
                  No endpoints configured. Create one to get started.
                </p>
              </div>
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Name</TableHead>
                    <TableHead>OpenAI Base URL</TableHead>
                    <TableHead>Capabilities</TableHead>
                    <TableHead>Created</TableHead>
                    <TableHead className="text-right">Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {endpoints.map((ep) => (
                    <TableRow key={ep.id}>
                      <TableCell className="font-medium">{ep.name}</TableCell>
                      <TableCell className="max-w-[200px] truncate font-mono text-xs">
                        {ep.openaiBaseURL || "—"}
                      </TableCell>
                      <TableCell>
                        <div className="flex flex-wrap gap-1">
                          {ep.supportOpenAIChatCompletion && (
                            <Badge variant="secondary" className="text-[10px]">Chat</Badge>
                          )}
                          {ep.supportOpenAIResponse && (
                            <Badge variant="secondary" className="text-[10px]">Response</Badge>
                          )}
                          {ep.supportAnthropicMessage && (
                            <Badge variant="secondary" className="text-[10px]">Anthropic</Badge>
                          )}
                        </div>
                      </TableCell>
                      <TableCell className="text-muted-foreground">
                        {new Date(ep.createdAt).toLocaleDateString()}
                      </TableCell>
                      <TableCell className="text-right">
                        <div className="flex items-center justify-end gap-1">
                          <Button variant="ghost" size="icon-sm" onClick={() => openEdit(ep)}>
                            <Pencil className="size-3.5" />
                          </Button>
                          <Button
                            variant="destructive"
                            size="xs"
                            disabled={deleting === ep.id}
                            onClick={() => handleDelete(ep.id)}
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
          </CardContent>
        </Card>

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