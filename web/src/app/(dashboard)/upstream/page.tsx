"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { usePersistentState } from "@/hooks/use-persistent-state";
import { useOptimisticUpdate } from "@/hooks/use-optimistic-update";
import { api } from "@/lib/api-client";
import { showErrorToast } from "@/lib/api-error-handler";
import { useAuth } from "@/lib/auth-context";
import { PermissionGuard } from "@/components/permission-guard";
import type {
  UpstreamGroupItem,
  UpstreamModelItem,
  UpstreamEndpointItem,
  PageInfo,
} from "@/lib/types";
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
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
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
import TraceInstallPopover from "@/components/trace-install-popover";
import { PageHeader } from "@/components/page-header";
import { ListEmptyState } from "@/components/list-empty-state";
import { TableSkeleton } from "@/components/table-skeleton";
import { useDeleteConfirm } from "@/hooks/use-delete-confirm";
import { FilterBar } from "@/components/filter-bar/filter-bar";
import { useFilterBar } from "@/components/filter-bar/use-filter-bar";
import type { FacetDef } from "@/components/filter-bar/types";
import {
  Plus,
  Pencil,
  ArrowLeftRight,
  ArrowUpFromLine,
  Type,
  Image as ImageIcon,
  SlidersHorizontal,
  Layers,
} from "lucide-react";
import { toast } from "sonner";
import { useIsMobile } from "@/hooks/use-mobile";
import { useT } from "@/lib/i18n";
import { copyTextToClipboard } from "@/lib/clipboard";

const DEFAULT_PAGE_SIZE = 10;
const VALID_PAGE_SIZES = [10, 20, 50];
// admin 代建下拉的用户列表一次性拉取上限
const USER_FETCH_LIMIT = 500;

// 常用 token 预设档位：点击即写入表单，替代上下箭头微调
const CONTEXT_LENGTH_PRESETS = [256_000, 512_000, 1_000_000];
const MAX_OUTPUT_PRESETS = [4_096, 8_192, 16_384, 32_768, 65_536, 131_072];

interface EndpointForm {
  name: string;
  openaiBaseURL: string;
  anthropicBaseURL: string;
  apiKey: string;
  supportOpenAIChatCompletion: boolean;
  supportOpenAIResponse: boolean;
  supportAnthropicMessage: boolean;
  ownerUserID?: number;
}

interface ModelForm {
  alias: string;
  modelId: string;
  upstreamModel: string;
  contextLength: number;
  maxOutputTokens: number;
  supportText: boolean;
  supportImage: boolean;
}

const emptyEndpointForm: EndpointForm = {
  name: "",
  openaiBaseURL: "",
  anthropicBaseURL: "",
  apiKey: "",
  supportOpenAIChatCompletion: true,
  supportOpenAIResponse: false,
  supportAnthropicMessage: false,
};

const emptyModelForm: ModelForm = {
  alias: "",
  modelId: "",
  upstreamModel: "",
  contextLength: 256000,
  maxOutputTokens: 65536,
  supportText: true,
  supportImage: false,
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

// 归属用户展示单元：头像 + 用户名；user 缺省显示占位 —（恒定短占位不加 tooltip）
function OwnerCell({ user }: { user?: UpstreamEndpointItem["user"] }) {
  if (!user) {
    return <span className="text-muted-foreground">—</span>;
  }
  return (
    <TooltipRoot>
      <TooltipTrigger
        render={
          <span className="flex max-w-[14ch] items-center gap-1.5">
            <Avatar size="sm">
              {user.avatar && <AvatarImage src={user.avatar} alt={user.name} />}
              <AvatarFallback className="text-[10px]">
                {user.name.charAt(0).toUpperCase() || "?"}
              </AvatarFallback>
            </Avatar>
            <span className="truncate text-xs text-muted-foreground">{user.name}</span>
          </span>
        }
      />
      <TooltipContent side="top" align="start" className="max-w-xs break-all">
        {user.name}
      </TooltipContent>
    </TooltipRoot>
  );
}

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

// 预设值 Popover：锚定在输入框旁，点选预设即写入表单并关闭；当前值高亮为主色，其余为 outline
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

export default function UpstreamPage() {
  const t = useT();
  const { isDemo, isAdmin } = useAuth();
  const isMobile = useIsMobile();
  const [groups, setGroups] = useState<UpstreamGroupItem[]>([]);
  const [modelTotal, setModelTotal] = useState(0);
  const [persistedPage, setPersistedPage] = usePersistentState("dashboard.upstream.page", 1);
  const [persistedPageSize, setPersistedPageSize] = usePersistentState(
    "dashboard.upstream.pageSize",
    DEFAULT_PAGE_SIZE,
  );
  const [pageInfo, setPageInfo] = useState<PageInfo>({
    page: persistedPage,
    pageSize: persistedPageSize,
    total: 0,
  });
  const [loading, setLoading] = useState(true);

  // 端点弹窗状态
  const [endpointDialogOpen, setEndpointDialogOpen] = useState(false);
  const [editingEndpointId, setEditingEndpointId] = useState<number | null>(null);
  const [endpointForm, setEndpointForm] = useState<EndpointForm>(emptyEndpointForm);
  // admin 代建下拉的候选用户（懒加载）
  const [userOptions, setUserOptions] = useState<{ id: number; name: string }[]>([]);
  const [saving, setSaving] = useState(false);

  // 模型弹窗状态：模型绑定所在组 endpoint，不可跨组移动
  const [modelDialogOpen, setModelDialogOpen] = useState(false);
  const [editingModel, setEditingModel] = useState<UpstreamModelItem | null>(null);
  const [targetEndpointID, setTargetEndpointID] = useState(0);
  const [modelForm, setModelForm] = useState<ModelForm>(emptyModelForm);
  // 标记用户是否手动改过 modelId；未手动改时新建表单跟随 alias 同步输入
  const [modelIdTouched, setModelIdTouched] = useState(false);

  // 仅 admin 可按归属用户名过滤（后端对普通用户忽略该参数）
  const facets = useMemo<FacetDef[]>(
    () =>
      isAdmin()
        ? [
            {
              key: "username",
              label: t("endpoints.filter_by_username"),
              options: [],
              target: "param",
              single: true,
            },
          ]
        : [],
    // eslint-disable-next-line react-hooks/exhaustive-deps -- isAdmin/t 均为稳定引用，仅需挂载时判定
    [],
  );

  const filterBar = useFilterBar({
    persistKey: "dashboard.upstream",
    facets,
    freeTextPlaceholder: t("upstream.search_placeholder"),
  });
  const { queryParams } = filterBar;

  const fetchUpstream = useCallback(
    async (page: number, pageSize: number, query?: string) => {
      const safeSize = VALID_PAGE_SIZES.includes(pageSize) ? pageSize : DEFAULT_PAGE_SIZE;
      setLoading(true);
      try {
        const rsp = await api.listUpstream(page, safeSize, query, queryParams.params.username);
        setGroups(rsp.groups ?? []);
        if (rsp.modelTotal !== undefined) {
          setModelTotal(rsp.modelTotal);
        }
        if (rsp.pageInfo) {
          setPageInfo(rsp.pageInfo);
          setPersistedPage(rsp.pageInfo.page);
          if (VALID_PAGE_SIZES.includes(rsp.pageInfo.pageSize)) {
            setPersistedPageSize(rsp.pageInfo.pageSize);
          }
        }
      } catch (err) {
        showErrorToast(err, { title: t("upstream.load_error") });
      } finally {
        setLoading(false);
      }
    },
    [t, setPersistedPage, setPersistedPageSize, queryParams.params.username],
  );

  /* eslint-disable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps -- 关键词 token 变化回到第 1 页查询；挂载时以持久化关键词发起首次查询 */
  useEffect(() => {
    fetchUpstream(1, pageInfo.pageSize, queryParams.freeText || undefined);
  }, [queryParams]);
  /* eslint-enable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps */

  const refresh = (page: number, pageSize?: number) =>
    fetchUpstream(page, pageSize ?? pageInfo.pageSize, queryParams.freeText || undefined);

  const groupByEndpointID = useMemo(() => {
    const map = new Map<number, UpstreamGroupItem>();
    for (const g of groups) map.set(g.endpoint.id, g);
    return map;
  }, [groups]);

  const ensureUserOptions = useCallback(async () => {
    if (userOptions.length > 0) return;
    try {
      const rsp = await api.listUsers(1, USER_FETCH_LIMIT);
      setUserOptions((rsp.items ?? []).map((u) => ({ id: u.id, name: u.name })));
    } catch {
      // 下拉加载失败不阻断创建，退化为缺省归属当前用户
    }
  }, [userOptions.length]);

  /* ─── 端点 CRUD ─────────────────────────────────────────────── */

  const openCreateEndpoint = () => {
    setEditingEndpointId(null);
    setEndpointForm(emptyEndpointForm);
    if (isAdmin()) void ensureUserOptions();
    setEndpointDialogOpen(true);
  };

  const openEditEndpoint = (ep: UpstreamEndpointItem) => {
    setEditingEndpointId(ep.id);
    setEndpointForm({
      name: ep.name,
      openaiBaseURL: ep.openaiBaseURL,
      anthropicBaseURL: ep.anthropicBaseURL,
      apiKey: "",
      supportOpenAIChatCompletion: ep.supportOpenAIChatCompletion,
      supportOpenAIResponse: ep.supportOpenAIResponse,
      supportAnthropicMessage: ep.supportAnthropicMessage,
    });
    setEndpointDialogOpen(true);
  };

  const handleSaveEndpoint = async () => {
    if (!endpointForm.name.trim()) {
      toast.error(t("endpoints.name_required"));
      return;
    }
    setSaving(true);
    try {
      if (editingEndpointId) {
        await api.updateEndpoint(editingEndpointId, {
          name: endpointForm.name,
          openaiBaseURL: endpointForm.openaiBaseURL || undefined,
          anthropicBaseURL: endpointForm.anthropicBaseURL || undefined,
          apiKey: endpointForm.apiKey || undefined,
          supportOpenAIChatCompletion: endpointForm.supportOpenAIChatCompletion,
          supportOpenAIResponse: endpointForm.supportOpenAIResponse,
          supportAnthropicMessage: endpointForm.supportAnthropicMessage,
        });
        toast.success(t("endpoints.updated_success"));
      } else {
        await api.createEndpoint({
          name: endpointForm.name,
          ownerUserID: isAdmin() && endpointForm.ownerUserID ? endpointForm.ownerUserID : undefined,
          openaiBaseURL: endpointForm.openaiBaseURL || undefined,
          anthropicBaseURL: endpointForm.anthropicBaseURL || undefined,
          apiKey: endpointForm.apiKey,
          supportOpenAIChatCompletion: endpointForm.supportOpenAIChatCompletion,
          supportOpenAIResponse: endpointForm.supportOpenAIResponse,
          supportAnthropicMessage: endpointForm.supportAnthropicMessage,
        });
        toast.success(t("endpoints.created_success"));
      }
      setEndpointDialogOpen(false);
      refresh(pageInfo.page);
    } catch (err) {
      showErrorToast(err, { title: t("endpoints.save_error") });
    } finally {
      setSaving(false);
    }
  };

  const deleteEndpointConfirm = useDeleteConfirm<UpstreamEndpointItem>({
    onConfirm: async (ep) => {
      await api.deleteEndpoint(ep.id);
      toast.success(t("endpoints.deleted_success"));
      refresh(pageInfo.page);
    },
    onError: (err) => showErrorToast(err, { title: t("endpoints.delete_error") }),
  });

  /* ─── 模型 CRUD ─────────────────────────────────────────────── */

  const openCreateModel = (ep: UpstreamEndpointItem) => {
    setEditingModel(null);
    setModelIdTouched(false);
    setTargetEndpointID(ep.id);
    setModelForm(emptyModelForm);
    setModelDialogOpen(true);
  };

  const openEditModel = (model: UpstreamModelItem, ep: UpstreamEndpointItem) => {
    setEditingModel(model);
    setModelIdTouched(true);
    setTargetEndpointID(ep.id);
    setModelForm({
      alias: model.alias,
      modelId: model.modelId ?? "",
      upstreamModel: model.upstreamModel,
      contextLength: model.contextLength || 256000,
      maxOutputTokens: model.maxOutputTokens || 65536,
      supportText: (model.capabilities ?? ["text"]).includes("text"),
      supportImage: (model.capabilities ?? []).includes("image"),
    });
    setModelDialogOpen(true);
  };

  const handleSaveModel = async () => {
    if (!modelForm.alias.trim() || !modelForm.upstreamModel.trim() || !targetEndpointID) {
      toast.error(t("models.fields_required"));
      return;
    }
    if (!modelForm.supportText) {
      toast.error(t("models.capabilities_require_text"));
      return;
    }
    const capabilities = [
      ...(modelForm.supportText ? (["text"] as const) : []),
      ...(modelForm.supportImage ? (["image"] as const) : []),
    ];
    setSaving(true);
    try {
      if (editingModel) {
        await api.updateModel(editingModel.id, {
          alias: modelForm.alias,
          ...(modelForm.modelId.trim() ? { modelId: modelForm.modelId.trim() } : {}),
          upstreamModel: modelForm.upstreamModel,
          contextLength: modelForm.contextLength,
          maxOutputTokens: modelForm.maxOutputTokens,
          capabilities,
        });
        toast.success(t("models.updated_success"));
      } else {
        await api.createModel({
          alias: modelForm.alias,
          ...(modelForm.modelId.trim() ? { modelId: modelForm.modelId.trim() } : {}),
          upstreamModel: modelForm.upstreamModel,
          endpointID: targetEndpointID,
          contextLength: modelForm.contextLength,
          maxOutputTokens: modelForm.maxOutputTokens,
          capabilities,
        });
        toast.success(t("models.created_success"));
      }
      setModelDialogOpen(false);
      refresh(pageInfo.page);
    } catch (err) {
      showErrorToast(err, { title: t("models.save_error") });
    } finally {
      setSaving(false);
    }
  };

  interface ModelConfirmTarget {
    model: UpstreamModelItem;
  }

  const deleteModelConfirm = useDeleteConfirm<ModelConfirmTarget>({
    onConfirm: async ({ model }) => {
      await api.deleteModel(model.id);
      toast.success(t("models.deleted_success"));
      refresh(pageInfo.page);
    },
    onError: (err) => showErrorToast(err, { title: t("models.delete_error") }),
  });

  // enabled 开关：乐观更新 + 失败回滚，避免整表重拉导致闪烁。
  // hook 的 setItems 是扁平数组 setState；这里桥接为「按模型 id 回写各组的 models」。
  const toggleEnabled = useOptimisticUpdate<UpstreamModelItem>({
    setItems: (action) => {
      setGroups((prev) => {
        const nextFlat =
          typeof action === "function" ? action(prev.flatMap((g) => g.models)) : action;
        const byID = new Map(nextFlat.map((m) => [m.id, m]));
        return prev.map((g) => ({
          ...g,
          models: g.models.map((m) => byID.get(m.id) ?? m),
        }));
      });
    },
    getKey: (m) => m.id,
    update: async (m) => {
      await api.updateModel(m.id, { enabled: m.enabled });
    },
    onSuccess: (m) => toast.success(m.enabled ? t("models.enabled") : t("models.disabled")),
    onError: (err) => showErrorToast(err, { title: t("models.toggle_error") }),
  });

  // 与 trigger 页一致：点击 alias 文本复制到剪贴板
  const handleCopyAlias = (alias: string) => {
    if (!alias) return;
    void copyTextToClipboard(alias).then((ok) =>
      ok ? toast.success(t("common.copied_to_clipboard")) : toast.error(t("common.copy_failed")),
    );
  };

  /* ─── 共享渲染片段 ──────────────────────────────────────────── */

  // 组头行内容：归属人、端点名、协议 badges、计数、截断提示、操作区
  const renderGroupHead = (
    group: UpstreamGroupItem,
    actions: { editEp: () => void; deleteEp: () => void; addModel: () => void },
  ) => {
    const ep = group.endpoint;
    return (
      <>
        <OwnerCell user={ep.user} />
        <span className="flex min-w-0 items-center gap-2">
          <TooltipRoot>
            <TooltipTrigger
              render={<span className="max-w-[16ch] truncate font-medium">{ep.name}</span>}
            />
            <TooltipContent side="top" align="start" className="max-w-xs break-all">
              {ep.name}
            </TooltipContent>
          </TooltipRoot>
          <span className="flex shrink-0 items-center gap-1">
            {ep.supportOpenAIChatCompletion && (
              <Badge variant="secondary" className="gap-1 px-1 py-0 text-[10px] font-normal">
                <ProviderIcon protocol="openai-chat-completion" size={12} />
              </Badge>
            )}
            {ep.supportOpenAIResponse && (
              <Badge variant="secondary" className="gap-1 px-1 py-0 text-[10px] font-normal">
                <ProviderIcon protocol="openai-response" size={12} />
              </Badge>
            )}
            {ep.supportAnthropicMessage && (
              <Badge variant="secondary" className="gap-1 px-1 py-0 text-[10px] font-normal">
                <ProviderIcon protocol="anthropic-message" size={12} />
              </Badge>
            )}
          </span>
          {group.truncated && (
            <Badge variant="outline" className="shrink-0 text-[10px] font-normal">
              {t("upstream.truncated")}
            </Badge>
          )}
        </span>
        <span className="ml-auto flex items-center gap-1.5">
          <span className="font-mono text-xs tabular-nums text-muted-foreground">
            {t("upstream.model_count").replace("{count}", String(group.modelCount))}
          </span>
          <Button
            variant="ghost"
            size="icon-sm"
            onClick={actions.editEp}
            aria-label={t("common.edit")}
            className="text-muted-foreground hover:text-foreground"
          >
            <Pencil className="size-3.5" />
          </Button>
          <DeleteButton
            label={t("common.delete")}
            locked={isDemo()}
            disabled={deleteEndpointConfirm.loading && deleteEndpointConfirm.target?.id === ep.id}
            onClick={actions.deleteEp}
          />
          <Button
            size="sm"
            variant="outline"
            className="h-7 gap-1 px-2 text-xs"
            onClick={actions.addModel}
          >
            <Plus className="size-3" />
            {t("upstream.add_model")}
          </Button>
        </span>
      </>
    );
  };

  // 模型行单元格（桌面表）：alias/modelId/upstream/limits/caps/enabled/created/actions
  const renderModelRow = (m: UpstreamModelItem) => (
    <TableRow key={m.id}>
      <TableCell>
        <TooltipRoot>
          <TooltipTrigger
            render={
              <span className="flex max-w-[16ch] items-center gap-1.5 font-medium">
                <ProviderIcon protocol={m.alias} size={14} className="shrink-0" />
                <span
                  className="cursor-pointer truncate underline-offset-2 hover:underline"
                  onClick={() => handleCopyAlias(m.alias)}
                >
                  {m.alias}
                </span>
              </span>
            }
          />
          <TooltipContent side="top" align="start" className="max-w-xs break-all">
            {t("models.click_to_copy")}
          </TooltipContent>
        </TooltipRoot>
      </TableCell>
      <TableCell className="font-mono text-xs">
        {m.modelId ? (
          <TooltipRoot>
            <TooltipTrigger
              render={<span className="block max-w-[12ch] truncate">{m.modelId}</span>}
            />
            <TooltipContent side="top" align="start" className="max-w-xs break-all">
              {m.modelId}
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
            render={<span className="block max-w-[20ch] truncate">{m.upstreamModel}</span>}
          />
          <TooltipContent side="top" align="start" className="max-w-xs break-all">
            {m.upstreamModel}
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
                  {formatTokens(m.contextLength)}
                </span>
              }
            />
            <TooltipContent side="top" align="start">
              {`${t("models.context_length")}: ${m.contextLength.toLocaleString()}`}
            </TooltipContent>
          </TooltipRoot>
          <TooltipRoot>
            <TooltipTrigger
              render={
                <span className="inline-flex items-center gap-1 rounded-md bg-secondary px-1.5 py-0.5 font-mono text-[11px] tabular-nums text-secondary-foreground">
                  <ArrowUpFromLine className="size-3 text-muted-foreground" />
                  {formatTokens(m.maxOutputTokens)}
                </span>
              }
            />
            <TooltipContent side="top" align="start">
              {`${t("models.max_output")}: ${m.maxOutputTokens.toLocaleString()}`}
            </TooltipContent>
          </TooltipRoot>
          <CapabilityBadges capabilities={m.capabilities} />
        </div>
      </TableCell>
      <TableCell>{null}</TableCell>
      <TableCell>
        <Switch
          size="sm"
          checked={m.enabled}
          disabled={toggleEnabled.updatingKey !== null}
          onCheckedChange={() => toggleEnabled.apply(m, { enabled: !m.enabled })}
          aria-label={m.enabled ? t("models.enabled") : t("models.disabled")}
        />
      </TableCell>
      <TableCell className="text-muted-foreground">
        {new Date(m.createdAt).toLocaleDateString()}
      </TableCell>
      <TableCell className="text-right">
        <div className="flex items-center justify-end gap-1">
          <Button
            variant="ghost"
            size="icon-sm"
            onClick={() => openEditModelFromRow(m)}
            aria-label={t("common.edit")}
            className="text-muted-foreground hover:text-foreground"
          >
            <Pencil className="size-3.5" />
          </Button>
          <DeleteButton
            label={t("common.delete")}
            locked={isDemo()}
            disabled={deleteModelConfirm.loading && deleteModelConfirm.target?.model.id === m.id}
            onClick={() => deleteModelConfirm.openDelete({ model: m })}
          />
        </div>
      </TableCell>
    </TableRow>
  );

  // 由模型 ID 反查所属组的 endpoint（当前页数据内）
  const findModelOwner = (modelId: number): UpstreamEndpointItem | null => {
    for (const g of groups) {
      if (g.models.some((m) => m.id === modelId)) return g.endpoint;
    }
    return null;
  };

  // 行内编辑：反查所属组以注入 targetEndpointID
  const openEditModelFromRow = (m: UpstreamModelItem) => {
    const owner = findModelOwner(m.id);
    if (!owner) return;
    openEditModel(m, owner);
  };

  return (
    <PermissionGuard module="upstream">
      <TooltipProvider>
        <div className="space-y-8">
          <PageHeader
            title={t("upstream.title")}
            description={t("upstream.subtitle")}
            actions={
              <div className="flex gap-2">
                <TraceInstallPopover />
                <Button onClick={openCreateEndpoint}>
                  <Plus className="mr-1 size-4" />
                  {t("upstream.create_endpoint")}
                </Button>
              </div>
            }
          />

          <Card>
            <CardHeader>
              <CardTitle className="font-display">{t("upstream.all_upstreams")}</CardTitle>
            </CardHeader>
            <CardContent>
              {/* Search — faceted bar */}
              <div className="mb-4 flex">
                <FilterBar
                  {...filterBar}
                  facets={[]}
                  placeholder={t("upstream.search_placeholder")}
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
              ) : groups.length === 0 ? (
                <ListEmptyState
                  icon={<Layers className="mb-3 size-10 text-muted-foreground/40" />}
                  message={t("upstream.empty")}
                />
              ) : (
                <>
                  {isMobile ? (
                    <div className="space-y-4">
                      {groups.map((group) => (
                        <div
                          key={group.endpoint.id}
                          className="rounded-lg border border-border bg-card p-4"
                        >
                          <div className="flex items-start justify-between gap-3">
                            <div className="min-w-0 space-y-1.5">
                              <OwnerCell user={group.endpoint.user} />
                              <TooltipRoot>
                                <TooltipTrigger
                                  render={
                                    <p className="line-clamp-1 text-sm font-medium">
                                      {group.endpoint.name}
                                    </p>
                                  }
                                />
                                <TooltipContent
                                  side="top"
                                  align="start"
                                  className="max-w-xs break-all"
                                >
                                  {group.endpoint.name}
                                </TooltipContent>
                              </TooltipRoot>
                              <p className="text-xs text-muted-foreground">
                                {t("upstream.model_count").replace(
                                  "{count}",
                                  String(group.modelCount),
                                )}
                              </p>
                            </div>
                            <div className="flex shrink-0 items-center gap-1">
                              <Button
                                variant="ghost"
                                size="icon-sm"
                                onClick={() => openEditEndpoint(group.endpoint)}
                                className="text-muted-foreground hover:text-foreground"
                              >
                                <Pencil className="size-3.5" />
                              </Button>
                              <DeleteButton
                                label={t("common.delete")}
                                locked={isDemo()}
                                disabled={
                                  deleteEndpointConfirm.loading &&
                                  deleteEndpointConfirm.target?.id === group.endpoint.id
                                }
                                onClick={() => deleteEndpointConfirm.openDelete(group.endpoint)}
                              />
                              <Button
                                size="sm"
                                variant="outline"
                                className="h-7 gap-1 px-2 text-xs"
                                onClick={() => openCreateModel(group.endpoint)}
                              >
                                <Plus className="size-3" />
                                {t("upstream.add_model")}
                              </Button>
                            </div>
                          </div>
                          <div className="mt-3 divide-y divide-border">
                            {group.models.map((m) => (
                              <div
                                key={m.id}
                                className="flex items-center justify-between gap-2 py-2"
                              >
                                <div className="min-w-0">
                                  <p className="flex items-center gap-1.5 text-sm font-medium">
                                    <ProviderIcon
                                      protocol={m.alias}
                                      size={14}
                                      className="shrink-0"
                                    />
                                    <TooltipRoot>
                                      <TooltipTrigger
                                        render={
                                          <span
                                            className="cursor-pointer truncate underline-offset-2 hover:underline"
                                            onClick={() => handleCopyAlias(m.alias)}
                                          >
                                            {m.alias}
                                          </span>
                                        }
                                      />
                                      <TooltipContent
                                        side="top"
                                        align="start"
                                        className="max-w-xs break-all"
                                      >
                                        {m.alias}
                                      </TooltipContent>
                                    </TooltipRoot>
                                  </p>
                                  <TooltipRoot>
                                    <TooltipTrigger
                                      render={
                                        <p className="mt-0.5 truncate font-mono text-xs text-muted-foreground">
                                          {m.upstreamModel}
                                        </p>
                                      }
                                    />
                                    <TooltipContent
                                      side="top"
                                      align="start"
                                      className="max-w-xs break-all"
                                    >
                                      {m.upstreamModel}
                                    </TooltipContent>
                                  </TooltipRoot>
                                  <div className="mt-1 flex items-center gap-1.5">
                                    <span className="inline-flex items-center gap-1 rounded-md bg-secondary px-1.5 py-0.5 font-mono text-[10px] tabular-nums text-secondary-foreground">
                                      <ArrowLeftRight className="size-3 text-muted-foreground" />
                                      {formatTokens(m.contextLength)}
                                    </span>
                                    <CapabilityBadges capabilities={m.capabilities} />
                                  </div>
                                </div>
                                <div className="flex shrink-0 flex-col items-end gap-1.5">
                                  <Switch
                                    size="sm"
                                    checked={m.enabled}
                                    disabled={toggleEnabled.updatingKey !== null}
                                    onCheckedChange={() =>
                                      toggleEnabled.apply(m, { enabled: !m.enabled })
                                    }
                                    aria-label={
                                      m.enabled ? t("models.enabled") : t("models.disabled")
                                    }
                                  />
                                  <div className="flex items-center gap-1">
                                    <Button
                                      variant="ghost"
                                      size="icon-sm"
                                      onClick={() => openEditModel(m, group.endpoint)}
                                      className="text-muted-foreground hover:text-foreground"
                                    >
                                      <Pencil className="size-3.5" />
                                    </Button>
                                    <DeleteButton
                                      label={t("common.delete")}
                                      locked={isDemo()}
                                      disabled={
                                        deleteModelConfirm.loading &&
                                        deleteModelConfirm.target?.model.id === m.id
                                      }
                                      onClick={() => deleteModelConfirm.openDelete({ model: m })}
                                    />
                                  </div>
                                </div>
                              </div>
                            ))}
                          </div>
                        </div>
                      ))}
                    </div>
                  ) : (
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead colSpan={9} className="bg-transparent" />
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {groups.map((group) => (
                          <TableRowGroup
                            key={group.endpoint.id}
                            group={group}
                            head={renderGroupHead(group, {
                              editEp: () => openEditEndpoint(group.endpoint),
                              deleteEp: () => deleteEndpointConfirm.openDelete(group.endpoint),
                              addModel: () => openCreateModel(group.endpoint),
                            })}
                            renderModelRow={renderModelRow}
                          />
                        ))}
                      </TableBody>
                    </Table>
                  )}

                  <PaginationBar
                    pageInfo={pageInfo}
                    onChange={(page, pageSize) => refresh(page, pageSize)}
                    totalLabel={t("pagination.endpoints")}
                  />
                  {modelTotal > 0 && (
                    <p className="mt-2 text-right text-xs text-muted-foreground">
                      {t("upstream.model_total").replace("{count}", String(modelTotal))}
                    </p>
                  )}
                </>
              )}
            </CardContent>
          </Card>

          {/* 端点删除确认：提示影响组内模型数 */}
          <DeleteConfirmDialog
            {...deleteEndpointConfirm.dialogProps}
            title={t("common.are_you_sure")}
            description={t("upstream.delete_endpoint_desc")
              .replace("{name}", deleteEndpointConfirm.target?.name ?? "")
              .replace(
                "{count}",
                String(
                  groupByEndpointID.get(deleteEndpointConfirm.target?.id ?? -1)?.modelCount ?? 0,
                ),
              )}
            confirmLabel={t("common.delete")}
            loadingLabel={t("common.deleting")}
          />

          {/* 模型删除确认 */}
          <DeleteConfirmDialog
            {...deleteModelConfirm.dialogProps}
            title={t("common.are_you_sure")}
            description={t("models.delete_desc").replace(
              "{name}",
              deleteModelConfirm.target?.model.alias ?? "",
            )}
            confirmLabel={t("common.delete")}
            loadingLabel={t("common.deleting")}
          />

          {/* 端点新建/编辑弹窗（admin 新建支持代建） */}
          <Dialog open={endpointDialogOpen} onOpenChange={setEndpointDialogOpen}>
            <DialogContent className="sm:max-w-md">
              <DialogHeader>
                <DialogTitle>
                  {editingEndpointId ? t("endpoints.edit") : t("endpoints.create")}
                </DialogTitle>
                <DialogDescription className="min-h-[2.5rem]">
                  {editingEndpointId
                    ? t("endpoints.edit_description")
                    : t("endpoints.create_description")}
                </DialogDescription>
              </DialogHeader>
              <div className="space-y-3">
                <div className="space-y-1">
                  <Label htmlFor="ep-name">{t("endpoints.name")}</Label>
                  <Input
                    id="ep-name"
                    placeholder={t("endpoints.name") + "..."}
                    value={endpointForm.name}
                    onChange={(e) => setEndpointForm((f) => ({ ...f, name: e.target.value }))}
                  />
                </div>
                {!editingEndpointId && isAdmin() && (
                  <div className="space-y-1">
                    <Label htmlFor="ep-owner">{t("upstream.owner")}</Label>
                    <Select
                      value={endpointForm.ownerUserID ? String(endpointForm.ownerUserID) : ""}
                      onValueChange={(value) =>
                        setEndpointForm((f) => ({
                          ...f,
                          ownerUserID: value ? Number(value) : undefined,
                        }))
                      }
                    >
                      <SelectTrigger id="ep-owner">
                        <SelectValue placeholder={t("upstream.owner_default")} />
                      </SelectTrigger>
                      <SelectContent>
                        {userOptions.map((u) => (
                          <SelectItem key={u.id} value={String(u.id)}>
                            {u.name}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    <p className="text-[11px] text-muted-foreground">
                      {t("upstream.owner_default")}
                    </p>
                  </div>
                )}
                <div className="space-y-1">
                  <Label htmlFor="ep-openai-url">{t("endpoints.openai_base_url")}</Label>
                  <Input
                    id="ep-openai-url"
                    placeholder="https://api.openai.com/v1"
                    value={endpointForm.openaiBaseURL}
                    onChange={(e) =>
                      setEndpointForm((f) => ({ ...f, openaiBaseURL: e.target.value }))
                    }
                  />
                </div>
                <div className="space-y-1">
                  <Label htmlFor="ep-anthropic-url">{t("endpoints.anthropic_base_url")}</Label>
                  <Input
                    id="ep-anthropic-url"
                    placeholder="https://api.anthropic.com/v1"
                    value={endpointForm.anthropicBaseURL}
                    onChange={(e) =>
                      setEndpointForm((f) => ({ ...f, anthropicBaseURL: e.target.value }))
                    }
                  />
                </div>
                <div className="space-y-1">
                  <Label htmlFor="ep-apikey">{t("endpoints.api_key")}</Label>
                  <Input
                    id="ep-apikey"
                    type="password"
                    placeholder={
                      editingEndpointId ? t("endpoints.keep_current") : t("endpoints.enter_api_key")
                    }
                    value={endpointForm.apiKey}
                    onChange={(e) => setEndpointForm((f) => ({ ...f, apiKey: e.target.value }))}
                  />
                </div>
                <div className="space-y-2">
                  <Label>{t("endpoints.capabilities")}</Label>
                  <div className="flex flex-col gap-2">
                    <label className="flex items-center gap-2 text-sm">
                      <input
                        type="checkbox"
                        checked={endpointForm.supportOpenAIChatCompletion}
                        onChange={(e) =>
                          setEndpointForm((f) => ({
                            ...f,
                            supportOpenAIChatCompletion: e.target.checked,
                          }))
                        }
                        className="rounded"
                      />
                      {t("endpoints.openai_chat_label")}
                    </label>
                    <label className="flex items-center gap-2 text-sm">
                      <input
                        type="checkbox"
                        checked={endpointForm.supportOpenAIResponse}
                        onChange={(e) =>
                          setEndpointForm((f) => ({
                            ...f,
                            supportOpenAIResponse: e.target.checked,
                          }))
                        }
                        className="rounded"
                      />
                      {t("endpoints.openai_response_label")}
                    </label>
                    <label className="flex items-center gap-2 text-sm">
                      <input
                        type="checkbox"
                        checked={endpointForm.supportAnthropicMessage}
                        onChange={(e) =>
                          setEndpointForm((f) => ({
                            ...f,
                            supportAnthropicMessage: e.target.checked,
                          }))
                        }
                        className="rounded"
                      />
                      {t("endpoints.anthropic_messages_label")}
                    </label>
                  </div>
                </div>
              </div>
              <DialogFooter>
                <Button variant="outline" onClick={() => setEndpointDialogOpen(false)}>
                  {t("common.cancel")}
                </Button>
                <Button onClick={handleSaveEndpoint} disabled={!endpointForm.name.trim() || saving}>
                  {saving
                    ? t("common.saving")
                    : editingEndpointId
                      ? t("endpoints.update")
                      : t("common.create")}
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>

          {/* 模型新建/编辑弹窗：无 endpoint 选择器（绑定不可移动） */}
          <Dialog open={modelDialogOpen} onOpenChange={setModelDialogOpen}>
            <DialogContent className="sm:max-w-md">
              <DialogHeader>
                <DialogTitle>{editingModel ? t("models.edit") : t("models.create")}</DialogTitle>
                <DialogDescription className="min-h-[2.5rem]">
                  {editingModel ? t("models.edit_desc") : t("upstream.create_model_desc")}
                </DialogDescription>
              </DialogHeader>
              <div className="space-y-3">
                <div className="space-y-1">
                  <Label htmlFor="model-alias">{t("models.alias")}</Label>
                  <Input
                    id="model-alias"
                    placeholder={t("models.alias_placeholder")}
                    value={modelForm.alias}
                    onChange={(e) => {
                      const alias = e.target.value;
                      setModelForm((f) => ({
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
                    value={modelForm.modelId}
                    onChange={(e) => {
                      setModelIdTouched(true);
                      setModelForm((f) => ({ ...f, modelId: e.target.value }));
                    }}
                  />
                  <p className="text-[11px] text-muted-foreground">{t("models.model_id_hint")}</p>
                </div>
                <div className="space-y-1">
                  <Label htmlFor="model-upstream">{t("models.upstream_model")}</Label>
                  <Input
                    id="model-upstream"
                    placeholder={t("models.upstream_model_placeholder")}
                    value={modelForm.upstreamModel}
                    onChange={(e) => setModelForm((f) => ({ ...f, upstreamModel: e.target.value }))}
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
                        value={modelForm.contextLength || ""}
                        onChange={(e) =>
                          setModelForm((f) => ({
                            ...f,
                            contextLength: Number(e.target.value) || 0,
                          }))
                        }
                      />
                      <TokenPresetPopover
                        label={t("models.context_length_presets")}
                        description={t("models.preset_desc")}
                        value={modelForm.contextLength}
                        presets={CONTEXT_LENGTH_PRESETS}
                        onSelect={(v) => setModelForm((f) => ({ ...f, contextLength: v }))}
                      />
                    </div>
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
                        value={modelForm.maxOutputTokens || ""}
                        onChange={(e) =>
                          setModelForm((f) => ({
                            ...f,
                            maxOutputTokens: Number(e.target.value) || 0,
                          }))
                        }
                      />
                      <TokenPresetPopover
                        label={t("models.max_output_presets")}
                        description={t("models.preset_desc")}
                        value={modelForm.maxOutputTokens}
                        presets={MAX_OUTPUT_PRESETS}
                        onSelect={(v) => setModelForm((f) => ({ ...f, maxOutputTokens: v }))}
                      />
                    </div>
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
                        checked={modelForm.supportText}
                        onCheckedChange={(v) => setModelForm((f) => ({ ...f, supportText: v }))}
                      />
                    </div>
                    <div className="flex items-center justify-between rounded-lg border border-input px-3 py-2">
                      <span className="flex items-center gap-1.5 text-sm">
                        <ImageIcon className="size-3.5 text-muted-foreground" />
                        {t("models.capability_image")}
                      </span>
                      <Switch
                        size="sm"
                        checked={modelForm.supportImage}
                        onCheckedChange={(v) => setModelForm((f) => ({ ...f, supportImage: v }))}
                      />
                    </div>
                  </div>
                </div>
              </div>
              <DialogFooter>
                <Button variant="outline" onClick={() => setModelDialogOpen(false)}>
                  {t("common.cancel")}
                </Button>
                <Button
                  onClick={handleSaveModel}
                  disabled={!modelForm.alias.trim() || !modelForm.upstreamModel.trim() || saving}
                >
                  {saving
                    ? t("common.saving")
                    : editingModel
                      ? t("common.update")
                      : t("common.create")}
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        </div>
      </TooltipProvider>
    </PermissionGuard>
  );
}

// TableRowGroup 渲染一个组头行（bg-muted）+ 该组全部模型行
function TableRowGroup({
  group,
  head,
  renderModelRow,
}: {
  group: UpstreamGroupItem;
  head: React.ReactNode;
  renderModelRow: (m: UpstreamModelItem) => React.ReactNode;
}) {
  return (
    <>
      <TableRow className="bg-muted/60 hover:bg-muted/60">
        <TableCell colSpan={9}>
          <div className="flex min-w-0 items-center gap-2.5 py-0.5">{head}</div>
        </TableCell>
      </TableRow>
      {group.models.map((m) => renderModelRow(m))}
    </>
  );
}
