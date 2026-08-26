"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { usePersistentState } from "@/hooks/use-persistent-state";
import { useOptimisticUpdate } from "@/hooks/use-optimistic-update";
import { useRouter } from "next/navigation";
import { api } from "@/lib/api-client";
import { showErrorToast } from "@/lib/api-error-handler";
import { useAuth } from "@/lib/auth-context";
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
import {
  TooltipProvider,
  TooltipRoot,
  TooltipTrigger,
  TooltipContent,
} from "@/components/ui/tooltip";
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
  Popover,
  PopoverContent,
  PopoverTrigger,
  PopoverHeader,
  PopoverTitle,
  PopoverDescription,
} from "@/components/ui/popover";
import { DeleteButton } from "@/components/delete-button";
import { DeleteConfirmDialog } from "@/components/delete-confirm-dialog";
import { Switch } from "@/components/ui/switch";
import { PaginationBar } from "@/components/pagination-bar";
import { ProviderIcon } from "@/components/provider-icon";
import { PageHeader } from "@/components/page-header";
import { ListEmptyState } from "@/components/list-empty-state";
import { TableSkeleton } from "@/components/table-skeleton";
import { useDeleteConfirm } from "@/hooks/use-delete-confirm";
import { FilterBar } from "@/components/filter-bar/filter-bar";
import { useFilterBar } from "@/components/filter-bar/use-filter-bar";
import ExportDialog from "@/components/export-dialog";
import ExportClaudecodeDialog from "@/components/export-claudecode-dialog";
import ExportCodexDialog from "@/components/export-codex-dialog";
import ExportPiDialog from "@/components/export-pi-dialog";
import { OpenCode, ClaudeCode, Codex, Pi } from "@lobehub/icons";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  Plus,
  Pencil,
  Cpu,
  FileDown,
  ChevronDown,
  ArrowLeftRight,
  ArrowUpFromLine,
  Type,
  Image as ImageIcon,
  SlidersHorizontal,
} from "lucide-react";
import { toast } from "sonner";
import { useIsMobile } from "@/hooks/use-mobile";
import { useT } from "@/lib/i18n";
import { copyTextToClipboard } from "@/lib/clipboard";

interface ModelForm {
  alias: string;
  modelId: string;
  upstreamModel: string;
  endpointID: number;
  contextLength: number;
  maxOutputTokens: number;
  supportText: boolean;
  supportImage: boolean;
}

const emptyForm: ModelForm = {
  alias: "",
  modelId: "",
  upstreamModel: "",
  endpointID: 0,
  contextLength: 256000,
  maxOutputTokens: 65536,
  supportText: true,
  supportImage: false,
};

// 一次拉取全部 endpoint 供下拉选择；当前无分页 UI，取上限 100
const ENDPOINT_FETCH_LIMIT = 100;

// 能力徽标：按模型输入模态渲染图标（text / image），未知模态回退为 Type 图标
function CapabilityBadges({ capabilities }: { capabilities?: string[] }) {
  const caps = capabilities && capabilities.length > 0 ? capabilities : ["text"];
  return (
    <div className="flex items-center gap-1.5">
      {caps.map((cap) => (
        <TooltipRoot key={cap}>
          <TooltipTrigger
            render={
              <span className="inline-flex items-center gap-1 rounded-md bg-secondary px-1.5 py-0.5 font-mono text-[11px] tabular-nums text-secondary-foreground">
                {cap === "image" ? (
                  <ImageIcon className="size-3 text-muted-foreground" />
                ) : (
                  <Type className="size-3 text-muted-foreground" />
                )}
              </span>
            }
          />
          <TooltipContent side="top">{cap}</TooltipContent>
        </TooltipRoot>
      ))}
    </div>
  );
}

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

// 常用 token 预设档位：点击即写入表单，替代上下箭头微调
const CONTEXT_LENGTH_PRESETS = [256_000, 512_000, 1_000_000];
const MAX_OUTPUT_PRESETS = [4_096, 8_192, 16_384, 32_768, 65_536, 131_072];

// 预设值 Popover：锚定在输入框旁，点选预设即写入表单并关闭；当前值高亮为主色，其余为 outline，自定义值无高亮
interface TokenPresetPopoverProps {
  label: string;
  description: string;
  value: number;
  presets: number[];
  onSelect: (v: number) => void;
}

function TokenPresetPopover({
  label,
  description,
  value,
  presets,
  onSelect,
}: TokenPresetPopoverProps) {
  const [open, setOpen] = useState(false);
  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger
        render={<Button type="button" variant="outline" size="icon" aria-label={label} />}
      >
        <SlidersHorizontal className="size-4" />
      </PopoverTrigger>
      <PopoverContent align="end" sideOffset={8} className="w-64 p-2.5">
        <PopoverHeader className="px-0.5 pb-1">
          <PopoverTitle className="text-xs">{label}</PopoverTitle>
          <PopoverDescription className="text-[11px]">{description}</PopoverDescription>
        </PopoverHeader>
        <div className="grid grid-cols-3 gap-1.5">
          {presets.map((preset) => {
            const active = preset === value;
            return (
              <Button
                key={preset}
                type="button"
                size="sm"
                variant={active ? "default" : "outline"}
                aria-pressed={active}
                className="font-mono tabular-nums"
                onClick={() => {
                  onSelect(preset);
                  setOpen(false);
                }}
              >
                {formatTokens(preset)}
              </Button>
            );
          })}
        </div>
      </PopoverContent>
    </Popover>
  );
}

export default function ModelsPage() {
  const router = useRouter();
  const t = useT();
  const { isDemo } = useAuth();
  const isMobile = useIsMobile();
  const [models, setModels] = useState<ModelItem[]>([]);
  const [endpoints, setEndpoints] = useState<EndpointItem[]>([]);
  const [persistedPage, setPersistedPage] = usePersistentState("dashboard.models.page", 1);
  const [persistedPageSize, setPersistedPageSize] = usePersistentState(
    "dashboard.models.pageSize",
    20,
  );
  const [pageInfo, setPageInfo] = useState<PageInfo>({
    page: persistedPage,
    pageSize: persistedPageSize,
    total: 0,
  });
  // 导出到 agent 框架的模型列表：仅启用，且按对外别名去重
  //（同一 alias 可通过多条记录绑定多个 endpoint，导出时只保留第一条）
  const exportModels = useMemo(() => {
    const seen = new Set<string>();
    return models.filter((m) => {
      if (!m.enabled || seen.has(m.alias)) return false;
      seen.add(m.alias);
      return true;
    });
  }, [models]);
  const [loading, setLoading] = useState(true);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [form, setForm] = useState<ModelForm>(emptyForm);
  // 标记用户是否手动改过 modelId；未手动改时新建表单跟随 alias 同步输入
  const [modelIdTouched, setModelIdTouched] = useState(false);
  const [saving, setSaving] = useState(false);
  const [exportDialogOpen, setExportDialogOpen] = useState(false);
  const [exportClaudecodeDialogOpen, setExportClaudecodeDialogOpen] = useState(false);
  const [exportCodexDialogOpen, setExportCodexDialogOpen] = useState(false);
  const [exportPiDialogOpen, setExportPiDialogOpen] = useState(false);

  const filterBar = useFilterBar({
    persistKey: "dashboard.models",
    facets: [],
    freeTextPlaceholder: t("models.search_placeholder"),
  });
  const { queryParams } = filterBar;

  const fetchData = useCallback(
    async (page: number, pageSize: number, query?: string) => {
      setLoading(true);
      try {
        const modelsRsp = await api.listModels(page, pageSize, query);
        setModels(modelsRsp.models ?? []);
        if (modelsRsp.pageInfo) {
          setPageInfo(modelsRsp.pageInfo);
          setPersistedPage(modelsRsp.pageInfo.page);
          setPersistedPageSize(modelsRsp.pageInfo.pageSize);
        }
      } catch (err) {
        showErrorToast(err, { title: t("models.load_error") });
      } finally {
        setLoading(false);
      }
    },
    [setPersistedPage, setPersistedPageSize, t],
  );

  const fetchEndpoints = useCallback(async () => {
    try {
      const endpointsRsp = await api.listEndpoints(1, ENDPOINT_FETCH_LIMIT);
      const list = endpointsRsp.endpoints ?? [];
      setEndpoints(list);
      return list;
    } catch (err) {
      showErrorToast(err, { title: t("endpoints.load_error") });
      return [];
    }
  }, [t]);

  /* eslint-disable react-hooks/set-state-in-effect -- Data fetching requires setting state from async effects on mount */
  useEffect(() => {
    fetchEndpoints();
  }, [fetchEndpoints]);
  /* eslint-enable react-hooks/set-state-in-effect */

  /* eslint-disable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps -- 关键词 token 变化回到第 1 页查询；挂载时以持久化关键词发起首次查询 */
  useEffect(() => {
    fetchData(1, pageInfo.pageSize, queryParams.freeText || undefined);
  }, [queryParams]);
  /* eslint-enable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps */

  const refresh = (page: number, pageSize?: number) =>
    fetchData(page, pageSize ?? pageInfo.pageSize, queryParams.freeText || undefined);

  const openCreate = () => {
    setEditingId(null);
    setModelIdTouched(false);
    setForm({ ...emptyForm, endpointID: endpoints[0]?.id ?? 0 });
    setDialogOpen(true);
  };

  const openEdit = (model: ModelItem) => {
    setEditingId(model.id);
    setModelIdTouched(true);
    setForm({
      alias: model.alias,
      modelId: model.modelId ?? "",
      upstreamModel: model.upstreamModel,
      endpointID: model.endpoint.id,
      contextLength: model.contextLength || 256000,
      maxOutputTokens: model.maxOutputTokens || 65536,
      supportText: (model.capabilities ?? ["text"]).includes("text"),
      supportImage: (model.capabilities ?? []).includes("image"),
    });
    // Ensure the model's current endpoint is present in the select options,
    // even if it falls outside the first page of endpoints.
    if (model.endpoint && !endpoints.some((ep) => ep.id === model.endpoint.id)) {
      setEndpoints((prev) => [...prev, model.endpoint]);
    }
    setDialogOpen(true);
  };

  const handleSave = async () => {
    if (!form.alias.trim() || !form.upstreamModel.trim() || !form.endpointID) {
      toast.error(t("models.fields_required"));
      return;
    }
    if (!form.supportText) {
      toast.error(t("models.capabilities_require_text"));
      return;
    }
    const capabilities = [
      ...(form.supportText ? (["text"] as const) : []),
      ...(form.supportImage ? (["image"] as const) : []),
    ];
    setSaving(true);
    try {
      if (editingId) {
        await api.updateModel(editingId, {
          alias: form.alias,
          ...(form.modelId.trim() ? { modelId: form.modelId.trim() } : {}),
          upstreamModel: form.upstreamModel,
          endpointID: form.endpointID,
          contextLength: form.contextLength,
          maxOutputTokens: form.maxOutputTokens,
          capabilities,
        });
        toast.success(t("models.updated_success"));
      } else {
        await api.createModel({
          alias: form.alias,
          ...(form.modelId.trim() ? { modelId: form.modelId.trim() } : {}),
          upstreamModel: form.upstreamModel,
          endpointID: form.endpointID,
          contextLength: form.contextLength,
          maxOutputTokens: form.maxOutputTokens,
          capabilities,
        });
        toast.success(t("models.created_success"));
      }
      setDialogOpen(false);
      fetchData(pageInfo.page, pageInfo.pageSize, queryParams.freeText || undefined);
    } catch (err) {
      showErrorToast(err, { title: t("models.save_error") });
    } finally {
      setSaving(false);
    }
  };

  const deleteConfirm = useDeleteConfirm<ModelItem>({
    onConfirm: async (model) => {
      await api.deleteModel(model.id);
      toast.success(t("models.deleted_success"));
      fetchData(pageInfo.page, pageInfo.pageSize, queryParams.freeText || undefined);
    },
    onError: (err) => showErrorToast(err, { title: t("models.delete_error") }),
  });

  // enabled 开关：乐观更新 + 失败回滚，避免整表重拉导致闪烁
  const toggleEnabled = useOptimisticUpdate<ModelItem>({
    setItems: setModels,
    getKey: (m) => m.id,
    update: async (m) => {
      await api.updateModel(m.id, { enabled: m.enabled });
    },
    onSuccess: (m) => toast.success(m.enabled ? t("models.enabled") : t("models.disabled")),
    onError: (err) => showErrorToast(err, { title: t("models.toggle_error") }),
  });

  const getEndpointName = (model: ModelItem) => {
    return model.endpoint?.name ?? `Endpoint #${model.endpoint?.id}`;
  };

  // 与 trigger 页一致：点击 alias 文本复制到剪贴板
  const handleCopyAlias = (alias: string) => {
    if (!alias) return;
    void copyTextToClipboard(alias).then((ok) =>
      ok ? toast.success(t("common.copied_to_clipboard")) : toast.error(t("common.copy_failed")),
    );
  };

  return (
    <PermissionGuard adminOnly module="models">
      <TooltipProvider>
        <div className="space-y-8">
          <PageHeader
            title={t("models.title")}
            description={t("models.subtitle")}
            actions={
              <div className="flex gap-2">
                <DropdownMenu>
                  <DropdownMenuTrigger render={<Button variant="outline" className="gap-1.5" />}>
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
                          <TooltipRoot>
                            <TooltipTrigger
                              render={
                                <span className="truncate text-xs text-muted-foreground">
                                  {t("models.export_opencode_hint")}
                                </span>
                              }
                            />
                            <TooltipContent side="top" className="max-w-xs">
                              {t("models.export_opencode_hint")}
                            </TooltipContent>
                          </TooltipRoot>
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
                          <TooltipRoot>
                            <TooltipTrigger
                              render={
                                <span className="truncate text-xs text-muted-foreground">
                                  {t("models.export_claudecode_hint")}
                                </span>
                              }
                            />
                            <TooltipContent side="top" className="max-w-xs">
                              {t("models.export_claudecode_hint")}
                            </TooltipContent>
                          </TooltipRoot>
                        </span>
                      </DropdownMenuItem>
                      <DropdownMenuItem
                        onClick={() => setExportCodexDialogOpen(true)}
                        className="items-start gap-2.5 rounded-lg px-2 py-2"
                      >
                        <span className="flex size-8 shrink-0 items-center justify-center rounded-lg border border-border bg-gradient-to-b from-secondary to-muted">
                          <Codex.Color size={17} />
                        </span>
                        <span className="flex min-w-0 flex-col gap-0.5">
                          <span className="text-sm font-medium leading-none">
                            {t("models.export_codex")}
                          </span>
                          <TooltipRoot>
                            <TooltipTrigger
                              render={
                                <span className="truncate text-xs text-muted-foreground">
                                  {t("models.export_codex_hint")}
                                </span>
                              }
                            />
                            <TooltipContent side="top" className="max-w-xs">
                              {t("models.export_codex_hint")}
                            </TooltipContent>
                          </TooltipRoot>
                        </span>
                      </DropdownMenuItem>
                      <DropdownMenuItem
                        onClick={() => setExportPiDialogOpen(true)}
                        className="items-start gap-2.5 rounded-lg px-2 py-2"
                      >
                        <span className="flex size-8 shrink-0 items-center justify-center rounded-lg border border-border bg-gradient-to-b from-secondary to-muted">
                          <Pi size={17} />
                        </span>
                        <span className="flex min-w-0 flex-col gap-0.5">
                          <span className="text-sm font-medium leading-none">
                            {t("models.export_pi")}
                          </span>
                          <TooltipRoot>
                            <TooltipTrigger
                              render={
                                <span className="truncate text-xs text-muted-foreground">
                                  {t("models.export_pi_hint")}
                                </span>
                              }
                            />
                            <TooltipContent side="top" className="max-w-xs">
                              {t("models.export_pi_hint")}
                            </TooltipContent>
                          </TooltipRoot>
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
            }
          />

          <Card>
            <CardHeader>
              <CardTitle className="font-display">{t("models.all_models")}</CardTitle>
            </CardHeader>
            <CardContent>
              {/* Search — faceted bar */}
              <div className="mb-4 flex">
                <FilterBar
                  {...filterBar}
                  facets={[]}
                  placeholder={t("models.search_placeholder")}
                />
              </div>
              {filterBar.tokens.length > 0 && (
                <p className="-mt-2 mb-3 text-xs text-muted-foreground">
                  {t("filter_bar.applied_count").replace(
                    "{count}",
                    String(filterBar.tokens.length),
                  )}
                </p>
              )}
              {loading ? (
                <TableSkeleton />
              ) : models.length === 0 ? (
                <ListEmptyState
                  icon={<Cpu className="mb-3 size-10 text-muted-foreground/40" />}
                  message={t("models.no_models")}
                />
              ) : (
                <>
                  {isMobile ? (
                    <div className="space-y-3">
                      {models.map((model) => (
                        <div key={model.id} className="rounded-lg border border-border bg-card p-4">
                          <div className="flex items-start justify-between gap-3">
                            <div className="min-w-0 flex-1">
                              <p className="flex items-center gap-1.5 text-sm font-medium">
                                <ProviderIcon
                                  protocol={model.alias}
                                  size={14}
                                  className="shrink-0"
                                />
                                <TooltipRoot>
                                  <TooltipTrigger
                                    render={
                                      <span
                                        className="cursor-pointer underline-offset-2 hover:underline"
                                        onClick={() => handleCopyAlias(model.alias)}
                                      >
                                        {model.alias}
                                      </span>
                                    }
                                  />
                                  <TooltipContent side="top" className="max-w-xs break-all">
                                    {t("models.click_to_copy")}
                                  </TooltipContent>
                                </TooltipRoot>
                              </p>
                              <TooltipRoot>
                                <TooltipTrigger
                                  render={
                                    <p className="mt-0.5 truncate font-mono text-xs text-muted-foreground">
                                      {model.upstreamModel}
                                    </p>
                                  }
                                />
                                <TooltipContent
                                  side="top"
                                  align="start"
                                  className="max-w-xs break-all"
                                >
                                  {model.upstreamModel}
                                </TooltipContent>
                              </TooltipRoot>
                              <div className="mt-1.5 flex flex-wrap items-center gap-1.5">
                                <span className="inline-flex items-center gap-1 rounded-md bg-secondary px-1.5 py-0.5 font-mono text-[11px] tabular-nums text-secondary-foreground">
                                  <ArrowLeftRight className="size-3 text-muted-foreground" />
                                  {formatTokens(model.contextLength)}
                                </span>
                                <span className="inline-flex items-center gap-1 rounded-md bg-secondary px-1.5 py-0.5 font-mono text-[11px] tabular-nums text-secondary-foreground">
                                  <ArrowUpFromLine className="size-3 text-muted-foreground" />
                                  {formatTokens(model.maxOutputTokens)}
                                </span>
                                <CapabilityBadges capabilities={model.capabilities} />
                              </div>
                            </div>
                            <div className="flex items-center gap-1">
                              <Button
                                variant="ghost"
                                size="icon-sm"
                                onClick={() => openEdit(model)}
                                className="text-muted-foreground hover:text-foreground"
                              >
                                <Pencil className="size-3.5" />
                              </Button>
                              <DeleteButton
                                label={t("common.delete")}
                                locked={isDemo()}
                                disabled={
                                  deleteConfirm.loading && deleteConfirm.target?.id === model.id
                                }
                                onClick={() => deleteConfirm.openDelete(model)}
                              />
                            </div>
                          </div>
                          <div className="mt-2">
                            <Switch
                              size="sm"
                              checked={model.enabled}
                              disabled={toggleEnabled.updatingKey !== null}
                              onCheckedChange={() =>
                                toggleEnabled.apply(model, { enabled: !model.enabled })
                              }
                              aria-label={
                                model.enabled ? t("models.enabled") : t("models.disabled")
                              }
                            />
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
                          <TableHead>{t("models.model_id")}</TableHead>
                          <TableHead>{t("models.upstream_model")}</TableHead>
                          <TableHead>{t("models.limits")}</TableHead>
                          <TableHead>{t("models.capabilities")}</TableHead>
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
                              <TooltipRoot>
                                <TooltipTrigger
                                  render={
                                    <span className="flex max-w-[16ch] items-center gap-1.5 font-medium">
                                      <ProviderIcon
                                        protocol={model.alias}
                                        size={14}
                                        className="shrink-0"
                                      />
                                      <span
                                        className="truncate cursor-pointer underline-offset-2 hover:underline"
                                        onClick={() => handleCopyAlias(model.alias)}
                                      >
                                        {model.alias}
                                      </span>
                                    </span>
                                  }
                                />
                                <TooltipContent
                                  side="top"
                                  align="start"
                                  className="max-w-xs break-all"
                                >
                                  {model.alias}
                                </TooltipContent>
                              </TooltipRoot>
                            </TableCell>
                            <TableCell className="font-mono text-xs">
                              {model.modelId ? (
                                <TooltipRoot>
                                  <TooltipTrigger
                                    render={
                                      <span className="block max-w-[12ch] truncate">
                                        {model.modelId}
                                      </span>
                                    }
                                  />
                                  <TooltipContent
                                    side="top"
                                    align="start"
                                    className="max-w-xs break-all"
                                  >
                                    {model.modelId}
                                  </TooltipContent>
                                </TooltipRoot>
                              ) : (
                                // 恒定占位符不会截断，无需 truncate/tooltip
                                <span>—</span>
                              )}
                            </TableCell>
                            <TableCell className="font-mono text-xs">
                              <TooltipRoot>
                                <TooltipTrigger
                                  render={
                                    <span className="block max-w-[20ch] truncate">
                                      {model.upstreamModel}
                                    </span>
                                  }
                                />
                                <TooltipContent
                                  side="top"
                                  align="start"
                                  className="max-w-xs break-all"
                                >
                                  {model.upstreamModel}
                                </TooltipContent>
                              </TooltipRoot>
                            </TableCell>
                            <TableCell>
                              <div className="flex items-center gap-1.5">
                                <TooltipRoot>
                                  <TooltipTrigger
                                    render={
                                      <span className="inline-flex items-center gap-1 rounded-md bg-secondary px-1.5 py-0.5 font-mono text-[11px] tabular-nums text-secondary-foreground">
                                        <ArrowLeftRight className="size-3 text-muted-foreground" />
                                        {formatTokens(model.contextLength)}
                                      </span>
                                    }
                                  />
                                  <TooltipContent side="top" align="start">
                                    {`${t("models.context_length")}: ${model.contextLength.toLocaleString()}`}
                                  </TooltipContent>
                                </TooltipRoot>
                                <TooltipRoot>
                                  <TooltipTrigger
                                    render={
                                      <span className="inline-flex items-center gap-1 rounded-md bg-secondary px-1.5 py-0.5 font-mono text-[11px] tabular-nums text-secondary-foreground">
                                        <ArrowUpFromLine className="size-3 text-muted-foreground" />
                                        {formatTokens(model.maxOutputTokens)}
                                      </span>
                                    }
                                  />
                                  <TooltipContent side="top" align="start">
                                    {`${t("models.max_output")}: ${model.maxOutputTokens.toLocaleString()}`}
                                  </TooltipContent>
                                </TooltipRoot>
                              </div>
                            </TableCell>
                            <TableCell>
                              <CapabilityBadges capabilities={model.capabilities} />
                            </TableCell>
                            <TableCell>
                              <Switch
                                size="sm"
                                checked={model.enabled}
                                disabled={toggleEnabled.updatingKey !== null}
                                onCheckedChange={() =>
                                  toggleEnabled.apply(model, { enabled: !model.enabled })
                                }
                                aria-label={
                                  model.enabled ? t("models.enabled") : t("models.disabled")
                                }
                              />
                            </TableCell>
                            <TableCell>
                              <TooltipRoot>
                                <TooltipTrigger
                                  render={
                                    <button
                                      onClick={() => router.push("/endpoints")}
                                      className="block max-w-[14ch] truncate text-primary underline-offset-2 hover:underline"
                                    >
                                      {getEndpointName(model)}
                                    </button>
                                  }
                                />
                                <TooltipContent
                                  side="top"
                                  align="start"
                                  className="max-w-xs break-all"
                                >
                                  {getEndpointName(model)}
                                </TooltipContent>
                              </TooltipRoot>
                            </TableCell>
                            <TableCell className="text-muted-foreground">
                              {new Date(model.createdAt).toLocaleDateString()}
                            </TableCell>
                            <TableCell className="text-right">
                              <div className="flex items-center justify-end gap-1">
                                <Button
                                  variant="ghost"
                                  size="icon-sm"
                                  onClick={() => openEdit(model)}
                                  className="text-muted-foreground hover:text-foreground"
                                >
                                  <Pencil className="size-3.5" />
                                </Button>
                                <DeleteButton
                                  label={t("common.delete")}
                                  locked={isDemo()}
                                  disabled={
                                    deleteConfirm.loading && deleteConfirm.target?.id === model.id
                                  }
                                  onClick={() => deleteConfirm.openDelete(model)}
                                />
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
                    totalLabel={t("pagination.models")}
                  />
                </>
              )}
            </CardContent>
          </Card>

          <DeleteConfirmDialog
            {...deleteConfirm.dialogProps}
            title={t("common.are_you_sure")}
            description={t("models.delete_desc").replace(
              "{name}",
              deleteConfirm.target?.alias ?? "",
            )}
            confirmLabel={t("common.delete")}
            loadingLabel={t("common.deleting")}
          />

          <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
            <DialogContent className="sm:max-w-md">
              <DialogHeader>
                <DialogTitle>{editingId ? t("models.edit") : t("models.create")}</DialogTitle>
                <DialogDescription>
                  {editingId ? t("models.edit_desc") : t("models.create_desc")}
                </DialogDescription>
              </DialogHeader>
              <div className="space-y-3">
                <div className="space-y-1">
                  <Label htmlFor="model-alias">{t("models.alias")}</Label>
                  <Input
                    id="model-alias"
                    placeholder={t("models.alias_placeholder")}
                    value={form.alias}
                    onChange={(e) => {
                      const alias = e.target.value;
                      setForm((f) => ({
                        ...f,
                        alias,
                        // 未手动改过 modelId 时新建表单跟随 alias 同步输入
                        ...(modelIdTouched ? {} : { modelId: alias }),
                      }));
                    }}
                  />
                </div>
                <div className="space-y-1">
                  <Label htmlFor="model-id">{t("models.model_id")}</Label>
                  <Input
                    id="model-id"
                    placeholder={t("models.model_id_placeholder")}
                    value={form.modelId}
                    onChange={(e) => {
                      setModelIdTouched(true);
                      setForm((f) => ({ ...f, modelId: e.target.value }));
                    }}
                  />
                  <p className="text-[11px] text-muted-foreground">{t("models.model_id_hint")}</p>
                </div>
                <div className="space-y-1">
                  <Label htmlFor="model-name">{t("models.upstream_model")}</Label>
                  <Input
                    id="model-name"
                    placeholder={t("models.upstream_model_placeholder")}
                    value={form.upstreamModel}
                    onChange={(e) => setForm((f) => ({ ...f, upstreamModel: e.target.value }))}
                  />
                </div>
                <div className="grid grid-cols-2 gap-3">
                  <div className="space-y-1">
                    <Label htmlFor="model-context-length">{t("models.context_length")}</Label>
                    <div className="flex gap-2">
                      <Input
                        id="model-context-length"
                        type="number"
                        min={0}
                        step={1000}
                        inputMode="numeric"
                        placeholder="256000"
                        className="[appearance:textfield] [&::-webkit-inner-spin-button]:appearance-none [&::-webkit-outer-spin-button]:appearance-none"
                        value={form.contextLength || ""}
                        onChange={(e) =>
                          setForm((f) => ({ ...f, contextLength: Number(e.target.value) || 0 }))
                        }
                      />
                      <TokenPresetPopover
                        label={t("models.context_length_presets")}
                        description={t("models.preset_desc")}
                        value={form.contextLength}
                        presets={CONTEXT_LENGTH_PRESETS}
                        onSelect={(v) => setForm((f) => ({ ...f, contextLength: v }))}
                      />
                    </div>
                    <p className="text-[11px] text-muted-foreground">
                      {t("models.context_length_hint")}
                    </p>
                  </div>
                  <div className="space-y-1">
                    <Label htmlFor="model-max-output">{t("models.max_output")}</Label>
                    <div className="flex gap-2">
                      <Input
                        id="model-max-output"
                        type="number"
                        min={0}
                        step={1000}
                        inputMode="numeric"
                        placeholder="65536"
                        className="[appearance:textfield] [&::-webkit-inner-spin-button]:appearance-none [&::-webkit-outer-spin-button]:appearance-none"
                        value={form.maxOutputTokens || ""}
                        onChange={(e) =>
                          setForm((f) => ({ ...f, maxOutputTokens: Number(e.target.value) || 0 }))
                        }
                      />
                      <TokenPresetPopover
                        label={t("models.max_output_presets")}
                        description={t("models.preset_desc")}
                        value={form.maxOutputTokens}
                        presets={MAX_OUTPUT_PRESETS}
                        onSelect={(v) => setForm((f) => ({ ...f, maxOutputTokens: v }))}
                      />
                    </div>
                    <p className="text-[11px] text-muted-foreground">
                      {t("models.max_output_hint")}
                    </p>
                  </div>
                </div>
                <div className="space-y-1">
                  <Label>{t("models.capabilities")}</Label>
                  <div className="grid grid-cols-2 gap-3">
                    <div className="flex items-center justify-between rounded-lg border border-input px-3 py-2">
                      <span className="flex items-center gap-1.5 text-sm">
                        <Type className="size-3.5 text-muted-foreground" />
                        {t("models.capability_text")}
                      </span>
                      <Switch
                        size="sm"
                        checked={form.supportText}
                        onCheckedChange={(v) => setForm((f) => ({ ...f, supportText: v }))}
                      />
                    </div>
                    <div className="flex items-center justify-between rounded-lg border border-input px-3 py-2">
                      <span className="flex items-center gap-1.5 text-sm">
                        <ImageIcon className="size-3.5 text-muted-foreground" />
                        {t("models.capability_image")}
                      </span>
                      <Switch
                        size="sm"
                        checked={form.supportImage}
                        onCheckedChange={(v) => setForm((f) => ({ ...f, supportImage: v }))}
                      />
                    </div>
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
                  disabled={
                    !form.alias.trim() || !form.upstreamModel.trim() || !form.endpointID || saving
                  }
                >
                  {saving
                    ? t("common.saving")
                    : editingId
                      ? t("common.update")
                      : t("common.create")}
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>

          <ExportDialog
            open={exportDialogOpen}
            onOpenChange={setExportDialogOpen}
            models={exportModels}
          />

          <ExportClaudecodeDialog
            open={exportClaudecodeDialogOpen}
            onOpenChange={setExportClaudecodeDialogOpen}
            models={exportModels}
          />

          <ExportCodexDialog
            open={exportCodexDialogOpen}
            onOpenChange={setExportCodexDialogOpen}
            models={exportModels}
          />

          <ExportPiDialog
            open={exportPiDialogOpen}
            onOpenChange={setExportPiDialogOpen}
            models={exportModels}
          />
        </div>
      </TooltipProvider>
    </PermissionGuard>
  );
}
