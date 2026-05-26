"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api-client";
import { PermissionGuard } from "@/components/permission-guard";
import type { ModelItem, EndpointItem } from "@/lib/types";
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
import { Plus, Trash2, Pencil, Cpu } from "lucide-react";
import { toast } from "sonner";

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
  const [models, setModels] = useState<ModelItem[]>([]);
  const [endpoints, setEndpoints] = useState<EndpointItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [form, setForm] = useState<ModelForm>(emptyForm);
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState<number | null>(null);

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const [modelsRsp, endpointsRsp] = await Promise.all([
        api.listModels(),
        api.listEndpoints(),
      ]);
      setModels(modelsRsp.models ?? []);
      setEndpoints(endpointsRsp.endpoints ?? []);
    } catch {
      toast.error("Failed to load models");
    } finally {
      setLoading(false);
    }
  }, []);

  /* eslint-disable react-hooks/set-state-in-effect -- Data fetching requires setting state from async effects on mount */
  useEffect(() => {
    fetchData();
  }, [fetchData]);
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
      endpointID: model.endpointID,
    });
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
      fetchData();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to save model");
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (id: number) => {
    setDeleting(id);
    try {
      await api.deleteModel(id);
      toast.success("Model deleted");
      fetchData();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to delete model");
    } finally {
      setDeleting(null);
    }
  };

  const getEndpointName = (endpointID: number) => {
    const ep = endpoints.find((e) => e.id === endpointID);
    return ep?.name ?? `Endpoint #${endpointID}`;
  };

  return (
    <PermissionGuard adminOnly>
      <div className="space-y-8">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="font-display text-3xl font-semibold tracking-tight text-foreground">Models</h1>
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
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Alias</TableHead>
                    <TableHead>Model ID</TableHead>
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
                      <TableCell>{getEndpointName(model.endpointID)}</TableCell>
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
                            onClick={() => handleDelete(model.id)}
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
                <select
                  id="model-endpoint"
                  value={form.endpointID}
                  onChange={(e) =>
                    setForm((f) => ({ ...f, endpointID: Number(e.target.value) }))
                  }
                  className="flex h-8 w-full rounded-lg border border-input bg-transparent px-2.5 py-1 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50 dark:bg-input/30"
                >
                  <option value={0} disabled>
                    Select endpoint
                  </option>
                  {endpoints.map((ep) => (
                    <option key={ep.id} value={ep.id}>
                      {ep.name}
                    </option>
                  ))}
                </select>
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