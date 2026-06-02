"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { api } from "@/lib/api-client";
import { PermissionGuard } from "@/components/permission-guard";
import type { ModelItem, EndpointItem, PageInfo } from "@/lib/types";
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
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
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
import { Plus, Trash2, Pencil, Cpu, AlertTriangle, ChevronLeft, ChevronRight, Search } from "lucide-react";
import { toast } from "sonner";
import { useIsMobile } from "@/hooks/use-mobile";

interface ModelForm {
  alias: string;
  modelName: string;
  endpointID: number;
}

const emptyForm: ModelForm = {
  alias: "",
  modelName: "",
  endpointID: 0,
};

export default function ModelsPage() {
  const router = useRouter();
  const isMobile = useIsMobile();
  const [models, setModels] = useState<ModelItem[]>([]);
  const [endpoints, setEndpoints] = useState<EndpointItem[]>([]);
  const [pageInfo, setPageInfo] = useState<PageInfo>({ page: 1, pageSize: 1, total: 0 });
  const [loading, setLoading] = useState(true);
  const [searchQuery, setSearchQuery] = useState("");
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [form, setForm] = useState<ModelForm>(emptyForm);
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState<number | null>(null);
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<{ id: number; name: string } | null>(null);

  const fetchData = useCallback(async (page: number, pageSize: number, query?: string) => {
    setLoading(true);
    try {
      const modelsRsp = await api.listModels(page, pageSize, query);
      setModels(modelsRsp.models ?? []);
      if (modelsRsp.pageInfo) setPageInfo(modelsRsp.pageInfo);
    } catch {
      toast.error("Failed to load models");
    } finally {
      setLoading(false);
    }
  }, []);

  const fetchEndpoints = useCallback(async () => {
    try {
      const endpointsRsp = await api.listEndpoints();
      const list = endpointsRsp.endpoints ?? [];
      setEndpoints(list);
      return list;
    } catch {
      toast.error("Failed to load endpoints");
      return [];
    }
  }, []);

  /* eslint-disable react-hooks/set-state-in-effect -- Data fetching requires setting state from async effects on mount */
  useEffect(() => {
    fetchData(1, 1);
    fetchEndpoints();
  }, [fetchData, fetchEndpoints]);
  /* eslint-enable react-hooks/set-state-in-effect */

  const openCreate = () => {
    setEditingId(null);
    setForm({ ...emptyForm, endpointID: endpoints[0]?.id ?? 0 });
    setDialogOpen(true);
  };

  const openEdit = (model: ModelItem) => {
    setEditingId(model.id);
    setForm({
      alias: model.alias,
      modelName: model.modelName,
      endpointID: model.endpoint.id,
    });
    // Ensure the model's current endpoint is present in the select options,
    // even if it falls outside the first page of endpoints.
    if (model.endpoint && !endpoints.some((ep) => ep.id === model.endpoint.id)) {
      setEndpoints((prev) => [...prev, model.endpoint]);
    }
    setDialogOpen(true);
  };

  const handleSave = async () => {
    if (!form.alias.trim() || !form.modelName.trim() || !form.endpointID) {
      toast.error("All fields are required");
      return;
    }
    setSaving(true);
    try {
      if (editingId) {
        await api.updateModel(editingId, {
          alias: form.alias,
          modelName: form.modelName,
          endpointID: form.endpointID,
        });
        toast.success("Model updated");
      } else {
        await api.createModel({
          alias: form.alias,
          modelName: form.modelName,
          endpointID: form.endpointID,
        });
        toast.success("Model created");
      }
      setDialogOpen(false);
      fetchData(pageInfo.page, pageInfo.pageSize, searchQuery || undefined);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to save model");
    } finally {
      setSaving(false);
    }
  };

  const openDeleteConfirm = (model: ModelItem) => {
    setDeleteTarget({ id: model.id, name: model.alias });
    setDeleteConfirmOpen(true);
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;
    setDeleting(deleteTarget.id);
    try {
      await api.deleteModel(deleteTarget.id);
      toast.success("Model deleted");
      fetchData(pageInfo.page, pageInfo.pageSize, searchQuery || undefined);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to delete model");
    } finally {
      setDeleting(null);
      setDeleteConfirmOpen(false);
      setDeleteTarget(null);
    }
  };

  const getEndpointName = (model: ModelItem) => {
    return model.endpoint?.name ?? `Endpoint #${model.endpoint?.id}`;
  };

  return (
    <PermissionGuard adminOnly>
      <div className="space-y-8">
        <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
          <div>
            <h1 className="font-display text-2xl md:text-3xl font-semibold tracking-tight text-foreground">Models</h1>
            <p className="mt-1.5 text-sm text-muted-foreground">
              Manage model aliases and routing
            </p>
          </div>
          <Button onClick={openCreate}>
            <Plus className="mr-1 size-4" />
            Create Model
          </Button>
        </div>

        <Card>
          <CardHeader>
            <CardTitle className="font-display">All Models</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="mb-4">
              <div className="relative w-full md:max-w-sm">
                <Search className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  placeholder="Search models..."
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") fetchData(1, pageInfo.pageSize, searchQuery || undefined);
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
            ) : models.length === 0 ? (
              <div className="flex flex-col items-center justify-center py-12 text-center">
                <Cpu className="mb-3 size-10 text-muted-foreground/40" />
                <p className="text-sm text-muted-foreground">
                  No models configured. Create one to get started.
                </p>
              </div>
            ) : (
              <>
                {isMobile ? (
                  <div className="space-y-3">
                    {models.map((model) => (
                      <div
                        key={model.id}
                        className="rounded-lg border border-border bg-card p-4"
                      >
                        <div className="flex items-start justify-between gap-3">
                          <div className="min-w-0 flex-1">
                            <p className="text-sm font-medium">{model.alias}</p>
                            <p className="mt-0.5 truncate font-mono text-xs text-muted-foreground">
                              {model.modelName}
                            </p>
                          </div>
                          <div className="flex items-center gap-1">
                            <Button variant="ghost" size="icon-sm" onClick={() => openEdit(model)} className="text-muted-foreground hover:text-foreground">
                              <Pencil className="size-3.5" />
                            </Button>
                            <Button
                              variant="destructive"
                              size="xs"
                              disabled={deleting === model.id}
                              onClick={() => openDeleteConfirm(model)}
                            >
                              <Trash2 className="mr-1 size-3" />
                              Delete
                            </Button>
                          </div>
                        </div>
                        <p className="mt-2 text-xs text-muted-foreground">
                          Endpoint: {getEndpointName(model)}
                        </p>
                        <p className="mt-0.5 text-xs text-muted-foreground">
                          Created {new Date(model.createdAt).toLocaleDateString()}
                        </p>
                      </div>
                    ))}
                  </div>
                ) : (
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>Alias</TableHead>
                        <TableHead>Model Name</TableHead>
                        <TableHead>Endpoint</TableHead>
                        <TableHead>Created</TableHead>
                        <TableHead className="text-right">Actions</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {models.map((model) => (
                        <TableRow key={model.id}>
                          <TableCell className="font-medium">{model.alias}</TableCell>
                          <TableCell className="font-mono text-xs">{model.modelName}</TableCell>
                          <TableCell>
                            <button
                              onClick={() => router.push("/endpoints")}
                              className="text-primary underline-offset-2 hover:underline"
                            >
                              {getEndpointName(model)}
                            </button>
                          </TableCell>
                          <TableCell className="text-muted-foreground">
                            {new Date(model.createdAt).toLocaleDateString()}
                          </TableCell>
                          <TableCell className="text-right">
                            <div className="flex items-center justify-end gap-1">
                              <Button variant="ghost" size="icon-sm" onClick={() => openEdit(model)} className="text-muted-foreground hover:text-foreground">
                                <Pencil className="size-3.5" />
                              </Button>
                              <Button
                                variant="destructive"
                                size="xs"
                                disabled={deleting === model.id}
                                onClick={() => openDeleteConfirm(model)}
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
                  <div className="mt-4 flex items-center justify-between gap-4">
                    <p className="hidden text-sm text-muted-foreground md:block">
                      {pageInfo.total} model{pageInfo.total !== 1 ? "s" : ""} total
                    </p>
                    <div className="flex items-center gap-2">
                      <Button
                        variant="outline"
                        size="sm"
                        disabled={pageInfo.page <= 1}
                        onClick={() => fetchData(pageInfo.page - 1, pageInfo.pageSize, searchQuery || undefined)}
                      >
                        <ChevronLeft className="size-4" />
                      </Button>
                      <span className="text-sm text-muted-foreground">
                        {pageInfo.page} / {Math.max(1, Math.ceil(pageInfo.total / pageInfo.pageSize))}
                      </span>
                      <Button
                        variant="outline"
                        size="sm"
                        disabled={pageInfo.page >= Math.ceil(pageInfo.total / pageInfo.pageSize)}
                        onClick={() => fetchData(pageInfo.page + 1, pageInfo.pageSize, searchQuery || undefined)}
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
                {editingId ? "Edit Model" : "Create Model"}
              </DialogTitle>
              <DialogDescription>
                {editingId
                  ? "Update model configuration."
                  : "Add a new model alias."}
              </DialogDescription>
            </DialogHeader>
            <div className="space-y-3">
              <div className="space-y-1">
                <Label htmlFor="model-alias">Alias</Label>
                <Input
                  id="model-alias"
                  placeholder="e.g. gpt-4o"
                  value={form.alias}
                  onChange={(e) => setForm((f) => ({ ...f, alias: e.target.value }))}
                />
              </div>
              <div className="space-y-1">
                <Label htmlFor="model-name">Model ID</Label>
                <Input
                  id="model-name"
                  placeholder="e.g. gpt-4o-2024-08-06"
                  value={form.modelName}
                  onChange={(e) => setForm((f) => ({ ...f, modelName: e.target.value }))}
                />
              </div>
              <div className="space-y-1">
                <Label htmlFor="model-endpoint">Endpoint</Label>
                <Select
                  value={String(form.endpointID)}
                  onValueChange={(value) =>
                    setForm((f) => ({ ...f, endpointID: Number(value as string) }))
                  }
                >
                  <SelectTrigger id="model-endpoint">
                    <SelectValue placeholder="Select endpoint">
                      {endpoints.find((ep) => ep.id === form.endpointID)?.name ?? ""}
                    </SelectValue>
                  </SelectTrigger>
                  <SelectContent>
                    {endpoints.map((ep) => (
                      <SelectItem key={ep.id} value={String(ep.id)}>
                        {ep.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </div>
            <DialogFooter>
              <Button variant="outline" onClick={() => setDialogOpen(false)}>
                Cancel
              </Button>
              <Button
                onClick={handleSave}
                disabled={!form.alias.trim() || !form.modelName.trim() || !form.endpointID || saving}
              >
                {saving ? "Saving..." : editingId ? "Update" : "Create"}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>
    </PermissionGuard>
  );
}