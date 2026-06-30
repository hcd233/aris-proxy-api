"use client";

import { useCallback, useEffect, useState } from "react";
import { usePersistentState } from "@/hooks/use-persistent-state";
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
import { Switch } from "@/components/ui/switch";
import { PaginationBar } from "@/components/pagination-bar";
import { ProviderIcon } from "@/components/provider-icon";
import ExportDialog from "@/components/export-dialog";
import ExportClaudecodeDialog from "@/components/export-claudecode-dialog";
import { OpenCode, ClaudeCode } from "@lobehub/icons";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Plus, Trash2, Pencil, Cpu, AlertTriangle, Search, FileDown, ChevronDown, ArrowLeftRight, ArrowUpFromLine } from "lucide-react";
import { toast } from "sonner";
import { useIsMobile } from "@/hooks/use-mobile";
import { useT } from "@/lib/i18n";

interface ModelForm {
  alias: string;
  modelName: string;
  endpointID: number;
  contextLength: number;
  maxOutputTokens: number;
}

const emptyForm: ModelForm = {
  alias: "",
  modelName: "",
  endpointID: 0,
  contextLength: 128000,
  maxOutputTokens: 64000,
};

// 将 token 数格式化为紧凑可读形式：128000 -> 128K，1048576 -> 1M
function formatTokens(n: number): string {
  if (!n || n <= 0) return "—";
  if (n >= 1_000_000) {
    const v = n / 1_000_000;
    return `${Number.isInteger(v) ? v : v.toFixed(1)}M`;
  }
  if (n >= 1_000) {
    const v = n / 1_000;
    return `${Number.isInteger(v) ? v : v.toFixed(1)}K`;
  }
  return String(n);
}

export default function ModelsPage() {
  const router = useRouter();
  const t = useT();
  const isMobile = useIsMobile();
  const [models, setModels] = useState<ModelItem[]>([]);
  const [endpoints, setEndpoints] = useState<EndpointItem[]>([]);
  const [persistedPage, setPersistedPage] = usePersistentState("dashboard.models.page", 1);
  const [persistedPageSize, setPersistedPageSize] = usePersistentState("dashboard.models.pageSize", 20);
  const [pageInfo, setPageInfo] = useState<PageInfo>({ page: persistedPage, pageSize: persistedPageSize, total: 0 });
  const [loading, setLoading] = useState(true);
  const [searchQuery, setSearchQuery] = useState("");
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [form, setForm] = useState<ModelForm>(emptyForm);
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState<number | null>(null);
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<{ id: number; name: string } | null>(null);
  const [exportDialogOpen, setExportDialogOpen] = useState(false);
  const [exportClaudecodeDialogOpen, setExportClaudecodeDialogOpen] = useState(false);

  const fetchData = useCallback(async (page: number, pageSize: number, query?: string) => {
    setLoading(true);
    try {
      const modelsRsp = await api.listModels(page, pageSize, query);
      setModels(modelsRsp.models ?? []);
      if (modelsRsp.pageInfo) {
        setPageInfo(modelsRsp.pageInfo);
        setPersistedPage(modelsRsp.pageInfo.page);
        setPersistedPageSize(modelsRsp.pageInfo.pageSize);
      }
    } catch {
      toast.error(t("models.load_error"));
    } finally {
      setLoading(false);
    }
  }, [setPersistedPage, setPersistedPageSize]);

  const fetchEndpoints = useCallback(async () => {
    try {
      const endpointsRsp = await api.listEndpoints(1, 100);
      const list = endpointsRsp.endpoints ?? [];
      setEndpoints(list);
      return list;
    } catch {
      toast.error(t("endpoints.load_error"));
      return [];
    }
  }, [t]);

  /* eslint-disable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps -- Data fetching requires setting state from async effects on mount */
  useEffect(() => {
    fetchData(persistedPage, persistedPageSize);
    fetchEndpoints();
  }, [fetchData, fetchEndpoints]);
  /* eslint-enable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps */

  const refresh = (page: number, pageSize?: number) =>
    fetchData(page, pageSize ?? pageInfo.pageSize, searchQuery || undefined);

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
      contextLength: model.contextLength || 128000,
      maxOutputTokens: model.maxOutputTokens || 64000,
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
      toast.error(t("models.fields_required"));
      return;
    }
    setSaving(true);
    try {
      if (editingId) {
        await api.updateModel(editingId, {
          alias: form.alias,
          modelName: form.modelName,
          endpointID: form.endpointID,
          contextLength: form.contextLength,
          maxOutputTokens: form.maxOutputTokens,
        });
        toast.success(t("models.updated_success"));
      } else {
        await api.createModel({
          alias: form.alias,
          modelName: form.modelName,
          endpointID: form.endpointID,
          contextLength: form.contextLength,
          maxOutputTokens: form.maxOutputTokens,
        });
        toast.success(t("models.created_success"));
      }
      setDialogOpen(false);
      fetchData(pageInfo.page, pageInfo.pageSize, searchQuery || undefined);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("models.save_error"));
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
      toast.success(t("models.deleted_success"));
      fetchData(pageInfo.page, pageInfo.pageSize, searchQuery || undefined);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("models.delete_error"));
    } finally {
      setDeleting(null);
      setDeleteConfirmOpen(false);
      setDeleteTarget(null);
    }
  };

  const handleToggleEnabled = async (model: ModelItem) => {
    try {
      await api.updateModel(model.id, { enabled: !model.enabled });
      toast.success(model.enabled ? t("models.disabled") : t("models.enabled"));
      fetchData(pageInfo.page, pageInfo.pageSize, searchQuery || undefined);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("models.toggle_error"));
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
            <h1 className="font-display text-2xl md:text-3xl font-semibold tracking-tight text-foreground">{t("models.title")}</h1>
            <p className="mt-1.5 text-sm text-muted-foreground">
              {t("models.subtitle")}
            </p>
          </div>
          <div className="flex gap-2">
            <DropdownMenu>
              <DropdownMenuTrigger
                render={<Button variant="outline" className="gap-1.5" />}
              >
                <FileDown className="size-4" />
                {t("models.export")}
                <ChevronDown className="size-3.5 opacity-50 transition-transform duration-150 group-aria-expanded/button:rotate-180" />
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="w-64 p-1.5">
                <DropdownMenuGroup>
                  <DropdownMenuLabel className="px-2 pb-1.5 pt-1 text-[11px] uppercase tracking-[0.08em] text-muted-foreground/70">
                    {t("models.export_target")}
                  </DropdownMenuLabel>
                  <DropdownMenuItem
                    onClick={() => setExportDialogOpen(true)}
                    className="items-start gap-2.5 rounded-lg px-2 py-2"
                  >
                    <span className="flex size-8 shrink-0 items-center justify-center rounded-lg border border-border bg-gradient-to-b from-secondary to-muted">
                      <OpenCode size={17} />
                    </span>
                    <span className="flex min-w-0 flex-col gap-0.5">
                      <span className="text-sm font-medium leading-none">
                        {t("models.export_opencode")}
                      </span>
                      <span className="truncate text-xs text-muted-foreground">
                        {t("models.export_opencode_hint")}
                      </span>
                    </span>
                  </DropdownMenuItem>
                  <DropdownMenuItem
                    onClick={() => setExportClaudecodeDialogOpen(true)}
                    className="items-start gap-2.5 rounded-lg px-2 py-2"
                  >
                    <span className="flex size-8 shrink-0 items-center justify-center rounded-lg border border-border bg-gradient-to-b from-secondary to-muted">
                      <ClaudeCode.Color size={17} />
                    </span>
                    <span className="flex min-w-0 flex-col gap-0.5">
                      <span className="text-sm font-medium leading-none">
                        {t("models.export_claudecode")}
                      </span>
                      <span className="truncate text-xs text-muted-foreground">
                        {t("models.export_claudecode_hint")}
                      </span>
                    </span>
                  </DropdownMenuItem>
                </DropdownMenuGroup>
              </DropdownMenuContent>
            </DropdownMenu>
            <Button onClick={openCreate}>
              <Plus className="mr-1 size-4" />
              {t("models.create")}
            </Button>
          </div>
        </div>

        <Card>
          <CardHeader>
            <CardTitle className="font-display">{t("models.all_models")}</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="mb-4">
              <div className="relative w-full md:max-w-sm">
                <Search className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  placeholder={t("models.search_placeholder")}
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
                  {t("models.no_models")}
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
                            <p className="flex items-center gap-1.5 text-sm font-medium">
                              <ProviderIcon protocol={model.alias} size={14} className="shrink-0" />
                              {model.alias}
                            </p>
                            <p className="mt-0.5 truncate font-mono text-xs text-muted-foreground">
                              {model.modelName}
                            </p>
                            <div className="mt-1.5 flex flex-wrap items-center gap-1.5">
                              <span className="inline-flex items-center gap-1 rounded-md bg-secondary px-1.5 py-0.5 font-mono text-[11px] tabular-nums text-secondary-foreground">
                                <ArrowLeftRight className="size-3 text-muted-foreground" />
                                {formatTokens(model.contextLength)}
                              </span>
                              <span className="inline-flex items-center gap-1 rounded-md bg-secondary px-1.5 py-0.5 font-mono text-[11px] tabular-nums text-secondary-foreground">
                                <ArrowUpFromLine className="size-3 text-muted-foreground" />
                                {formatTokens(model.maxOutputTokens)}
                              </span>
                            </div>
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
                              {t("common.delete")}
                            </Button>
                          </div>
                        </div>
                        <div className="mt-2 flex items-center gap-2">
                          <Switch
                            size="sm"
                            checked={model.enabled}
                            onCheckedChange={() => handleToggleEnabled(model)}
                          />
                              <span className="text-xs text-muted-foreground">
                                {model.enabled ? t("models.enabled") : t("models.disabled")}
                              </span>
                            </div>
                        <p className="mt-1 text-xs text-muted-foreground">
                          {t("models.endpoint")}: {getEndpointName(model)}
                        </p>
                        <p className="mt-0.5 text-xs text-muted-foreground">
                          {t("common.created")} {new Date(model.createdAt).toLocaleDateString()}
                        </p>
                      </div>
                    ))}
                  </div>
                ) : (
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>{t("models.alias")}</TableHead>
                        <TableHead>{t("models.model_name")}</TableHead>
                        <TableHead>{t("models.limits")}</TableHead>
                        <TableHead>{t("models.enabled")}</TableHead>
                        <TableHead>{t("models.endpoint")}</TableHead>
                        <TableHead>{t("common.created")}</TableHead>
                        <TableHead className="text-right">{t("common.actions")}</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {models.map((model) => (
                        <TableRow key={model.id}>
                          <TableCell>
                            <span className="flex items-center gap-1.5 font-medium">
                              <ProviderIcon protocol={model.alias} size={14} className="shrink-0" />
                              {model.alias}
                            </span>
                          </TableCell>
                          <TableCell className="font-mono text-xs">{model.modelName}</TableCell>
                          <TableCell>
                            <div className="flex items-center gap-1.5">
                              <span
                                className="inline-flex items-center gap-1 rounded-md bg-secondary px-1.5 py-0.5 font-mono text-[11px] tabular-nums text-secondary-foreground"
                                title={`${t("models.context_length")}: ${model.contextLength.toLocaleString()}`}
                              >
                                <ArrowLeftRight className="size-3 text-muted-foreground" />
                                {formatTokens(model.contextLength)}
                              </span>
                              <span
                                className="inline-flex items-center gap-1 rounded-md bg-secondary px-1.5 py-0.5 font-mono text-[11px] tabular-nums text-secondary-foreground"
                                title={`${t("models.max_output")}: ${model.maxOutputTokens.toLocaleString()}`}
                              >
                                <ArrowUpFromLine className="size-3 text-muted-foreground" />
                                {formatTokens(model.maxOutputTokens)}
                              </span>
                            </div>
                          </TableCell>
                          <TableCell>
                            <div className="flex items-center gap-2">
                              <Switch
                                size="sm"
                                checked={model.enabled}
                                onCheckedChange={() => handleToggleEnabled(model)}
                              />
                              <span className="text-xs text-muted-foreground">
                                {model.enabled ? t("models.enabled") : t("models.disabled")}
                              </span>
                            </div>
                          </TableCell>
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
                              {t("common.delete")}
                            </Button>
                          </div>
                        </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                )}

                <PaginationBar
                  pageInfo={pageInfo}
                  onChange={(page, pageSize) => refresh(page, pageSize)}
                  totalLabel="models"
                />
              </>
            )}
          </CardContent>
        </Card>

        <AlertDialog open={deleteConfirmOpen} onOpenChange={setDeleteConfirmOpen}>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle className="flex items-center gap-2">
              <AlertTriangle className="size-5 text-destructive" />
              {t("common.are_you_sure")}
              </AlertDialogTitle>
              <AlertDialogDescription>
                {t("models.delete_desc").replace("{name}", deleteTarget?.name ?? "")}
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>{t("common.cancel")}</AlertDialogCancel>
            <AlertDialogAction variant="destructive" onClick={handleDelete} disabled={deleting !== null}>
              {deleting !== null ? t("common.deleting") : t("common.delete")}
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>

        <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
          <DialogContent className="sm:max-w-md">
            <DialogHeader>
              <DialogTitle>
                {editingId ? t("models.edit") : t("models.create")}
              </DialogTitle>
              <DialogDescription>
                {editingId
                  ? t("models.edit_desc")
                  : t("models.create_desc")}
              </DialogDescription>
            </DialogHeader>
            <div className="space-y-3">
              <div className="space-y-1">
                <Label htmlFor="model-alias">{t("models.alias")}</Label>
                <Input
                  id="model-alias"
                  placeholder={t("models.alias_placeholder")}
                  value={form.alias}
                  onChange={(e) => setForm((f) => ({ ...f, alias: e.target.value }))}
                />
              </div>
              <div className="space-y-1">
                <Label htmlFor="model-name">{t("models.model_name")}</Label>
                <Input
                  id="model-name"
                  placeholder={t("models.model_name_placeholder")}
                  value={form.modelName}
                  onChange={(e) => setForm((f) => ({ ...f, modelName: e.target.value }))}
                />
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div className="space-y-1">
                  <Label htmlFor="model-context-length">{t("models.context_length")}</Label>
                  <Input
                    id="model-context-length"
                    type="number"
                    min={0}
                    step={1000}
                    inputMode="numeric"
                    placeholder="128000"
                    value={form.contextLength || ""}
                    onChange={(e) =>
                      setForm((f) => ({ ...f, contextLength: Number(e.target.value) || 0 }))
                    }
                  />
                  <p className="text-[11px] text-muted-foreground">{t("models.context_length_hint")}</p>
                </div>
                <div className="space-y-1">
                  <Label htmlFor="model-max-output">{t("models.max_output")}</Label>
                  <Input
                    id="model-max-output"
                    type="number"
                    min={0}
                    step={1000}
                    inputMode="numeric"
                    placeholder="64000"
                    value={form.maxOutputTokens || ""}
                    onChange={(e) =>
                      setForm((f) => ({ ...f, maxOutputTokens: Number(e.target.value) || 0 }))
                    }
                  />
                  <p className="text-[11px] text-muted-foreground">{t("models.max_output_hint")}</p>
                </div>
              </div>
              <div className="space-y-1">
                <Label htmlFor="model-endpoint">{t("models.endpoint")}</Label>
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
                {t("common.cancel")}
              </Button>
              <Button
                onClick={handleSave}
                disabled={!form.alias.trim() || !form.modelName.trim() || !form.endpointID || saving}
              >
                {saving ? t("common.saving") : editingId ? t("common.update") : t("common.create")}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        <ExportDialog
          open={exportDialogOpen}
          onOpenChange={setExportDialogOpen}
          models={models}
        />

        <ExportClaudecodeDialog
          open={exportClaudecodeDialogOpen}
          onOpenChange={setExportClaudecodeDialogOpen}
          models={models}
        />
      </div>
    </PermissionGuard>
  );
}