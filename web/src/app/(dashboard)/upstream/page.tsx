"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
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
  ModelListItem,
  PageInfo,
} from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { TooltipProvider } from "@/components/ui/tooltip";
import { DeleteConfirmDialog } from "@/components/delete-confirm-dialog";
import { PaginationBar } from "@/components/pagination-bar";
import TraceInstallPopover from "@/components/trace-install-popover";
import { PageHeader } from "@/components/page-header";
import { ListEmptyState } from "@/components/list-empty-state";
import { TableSkeleton } from "@/components/table-skeleton";
import { useDeleteConfirm } from "@/hooks/use-delete-confirm";
import { FilterBar } from "@/components/filter-bar/filter-bar";
import { useFilterBar } from "@/components/filter-bar/use-filter-bar";
import type { FacetDef } from "@/components/filter-bar/types";
import { Plus, Layers, Lock } from "lucide-react";
import { toast } from "sonner";
import { useIsMobile } from "@/hooks/use-mobile";
import { useI18n } from "@/lib/i18n";
import { copyTextToClipboard } from "@/lib/clipboard";

import {
  DEFAULT_CONTEXT_LENGTH,
  DEFAULT_MAX_OUTPUT,
  MODEL_CAPABILITIES,
  emptyEndpointForm,
  emptyModelForm,
  emptyPricing,
} from "./shared";
import type { EndpointForm, ModelForm } from "./shared";
import type { ModelCapability, PricingDTO } from "@/lib/types";
import { EndpointDialog } from "./endpoint-dialog";
import { ModelDialog } from "./model-dialog";
import { PricingDialog } from "./pricing-dialog";
import { GroupedView } from "./grouped-view";
import { FlatView } from "./flat-view";
import { useModelList } from "./use-model-list";
import { ViewSwitch } from "@/components/view-switch";

const DEFAULT_PAGE_SIZE = 10;
const VALID_PAGE_SIZES = [10, 20, 50];
// admin 代建下拉的用户列表一次性拉取上限
const USER_FETCH_LIMIT = 500;

/** 定价弹窗目标：分组行与平铺行都满足的最小形状（导入用上游真名） */
interface PricingTarget {
  id: number;
  alias: string;
  upstreamModel: string;
}

export default function UpstreamPage() {
  const { t, locale } = useI18n();
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
  // 只用到 id（更新路径），故收窄为最小形状，两视图共用
  const [editingModel, setEditingModel] = useState<{ id: number } | null>(null);
  const [targetEndpointID, setTargetEndpointID] = useState(0);
  const [modelForm, setModelForm] = useState<ModelForm>(emptyModelForm);
  // 标记用户是否手动改过 modelId；未手动改时新建表单跟随 alias 同步输入
  const [modelIdTouched, setModelIdTouched] = useState(false);
  // modelId 相对打开弹窗时的原值变化时，展示「同步更新历史记录」开关。
  const [originalModelId, setOriginalModelId] = useState("");
  const [syncHistory, setSyncHistory] = useState(false);
  // 必须用 trim 后的非空值判定：提交时 modelId 会 trim，纯空白/尾随空格若按原值比较，
  // 会出现「开关显示但后端未收到 modelId、未实际同步」的误导。
  const trimmedModelId = modelForm.modelId.trim();
  const showSyncHistory =
    editingModel !== null && trimmedModelId !== "" && trimmedModelId !== originalModelId.trim();

  // 定价弹窗状态：定价是模型行的平级操作，只作用于已保存模型（新建弹窗不再含定价）
  const [pricingDialogOpen, setPricingDialogOpen] = useState(false);
  const [pricingTarget, setPricingTarget] = useState<PricingTarget | null>(null);
  const [pricingForm, setPricingForm] = useState<PricingDTO>(emptyPricing);
  const [savingPricing, setSavingPricing] = useState(false);

  // 当前视图：分组（端点为组）/ 平铺（模型为行）
  const [view, setView] = usePersistentState<"grouped" | "flat">(
    "dashboard.upstream.view",
    "grouped",
  );

  // 仅 admin 可按归属用户名过滤（后端对普通用户忽略该参数）。
  // 选项异步加载（与 ensureUserOptions 同源的 api.listUsers，取用户名）：
  // FilterBar 只从 options 出候选、无自由输入，空 options 会让该维度完全不可用。
  const loadUsernameOptions = useCallback(async () => {
    try {
      const rsp = await api.listUsers(1, USER_FETCH_LIMIT);
      return (rsp.items ?? []).map((u) => u.name);
    } catch {
      return [];
    }
  }, []);

  // 配置缺失筛选（两个视图共有）：选中后 missingOnly=true 参数（仅看未定价/未填规格）
  const missingFacet = useMemo<FacetDef[]>(
    () => [
      {
        key: "missingOnly",
        label: t("upstream.filter_config_missing"),
        options: ["true"],
        formatValue: () => t("upstream.filter_config_missing_value"),
        target: "param",
        single: true,
      },
    ],
    // locale 必须在依赖里：t 引用已稳定（见 lib/i18n.tsx），翻译文本刷新只能靠 locale 驱动重算
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [locale],
  );

  const usernameFacet = useMemo<FacetDef[]>(
    () =>
      isAdmin()
        ? [
            {
              key: "username",
              label: t("endpoints.filter_by_username"),
              options: loadUsernameOptions,
              target: "param",
              single: true,
            },
          ]
        : [],
    // locale 必须在依赖里：t 引用已稳定（见 lib/i18n.tsx），翻译文本刷新只能靠 locale 驱动重算
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [locale, loadUsernameOptions],
  );

  // 平铺视图独有：状态 / 能力 / 端点三个筛选维度（后端各自有对应参数）
  const flatFacets = useMemo<FacetDef[]>(
    () => [
      {
        key: "status",
        label: t("upstream.filter_status"),
        options: ["enabled", "disabled"],
        formatValue: (v) =>
          v === "enabled" ? t("upstream.status_enabled") : t("upstream.status_disabled"),
        target: "param",
        single: true,
      },
      {
        key: "capability",
        label: t("upstream.filter_capability"),
        options: ["text", "image", "pdf", "video", "audio"],
        formatValue: (v) => t(`models.capability_${v}`),
        target: "param",
        single: true,
      },
      {
        key: "endpointID",
        label: t("upstream.filter_endpoint"),
        // 选项取自已加载的端点分组（平铺接口按 ID 精确过滤，值为 ID 字符串）
        options: groups.map((g) => String(g.endpoint.id)),
        formatValue: (v) => groups.find((g) => String(g.endpoint.id) === v)?.endpoint.name ?? v,
        target: "param",
        single: true,
      },
      ...usernameFacet,
    ],
    // locale 必须在依赖里：t 引用已稳定（见 lib/i18n.tsx），翻译文本刷新只能靠 locale 驱动重算
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [groups, usernameFacet, locale],
  );

  // 两个 filterBar 实例：共有维度（关键词/username）在切换时单向同步，
  // 各自特有维度互不污染（单实例方案会让平铺的 status 串进分组请求）。
  const groupedFilterBar = useFilterBar({
    persistKey: "dashboard.upstream.grouped",
    facets: usernameFacet,
    freeTextPlaceholder: t("upstream.search_placeholder"),
  });
  const flatFilterBar = useFilterBar({
    persistKey: "dashboard.upstream.flat",
    facets: flatFacets,
    freeTextPlaceholder: t("upstream.search_placeholder"),
  });

  // 工具条渲染用当前视图的实例；数据请求各自绑定自己的实例——
  // 若分组请求跟随 activeFilterBar，平铺视图改筛选会连带重拉分组接口。
  const activeFilterBar = view === "grouped" ? groupedFilterBar : flatFilterBar;
  const groupedQueryParams = groupedFilterBar.queryParams;

  // 切换视图：带走共有维度（关键词 + username），各自特有维度留在原实例
  const handleViewChange = (next: "grouped" | "flat") => {
    const from = view === "grouped" ? groupedFilterBar : flatFilterBar;
    const to = next === "grouped" ? groupedFilterBar : flatFilterBar;
    for (const key of [null, "username"]) {
      const token = from.tokens.find((tk) => (key === null ? tk.key === null : tk.key === key));
      if (token) to.addToken(token);
    }
    setView(next);
  };

  // 平铺视图数据（真分页 + SQL 级排序），仅在平铺视图激活时拉取
  const flat = useModelList({
    freeText: flatFilterBar.queryParams.freeText,
    params: flatFilterBar.queryParams.params,
    enabled: view === "flat",
  });

  // 竞态守卫：关键词/筛选连点时慢响应不得覆盖新响应
  const fetchSeqRef = useRef(0);
  const fetchUpstream = useCallback(
    async (page: number, pageSize: number, query?: string) => {
      const safeSize = VALID_PAGE_SIZES.includes(pageSize) ? pageSize : DEFAULT_PAGE_SIZE;
      const seq = ++fetchSeqRef.current;
      setLoading(true);
      try {
        const rsp = await api.listUpstream(
          page,
          safeSize,
          query,
          groupedQueryParams.params.username,
        );
        if (seq !== fetchSeqRef.current) return;
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
        if (seq !== fetchSeqRef.current) return;
        showErrorToast(err, { title: t("upstream.load_error") });
      } finally {
        if (seq === fetchSeqRef.current) setLoading(false);
      }
    },
    [t, setPersistedPage, setPersistedPageSize, groupedQueryParams.params.username],
  );

  /* eslint-disable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps -- 关键词 token 变化回到第 1 页查询；挂载时以持久化关键词发起首次查询 */
  useEffect(() => {
    fetchUpstream(1, pageInfo.pageSize, groupedQueryParams.freeText || undefined);
  }, [groupedQueryParams]);
  /* eslint-enable react-hooks/set-state-in-effect, react-hooks/exhaustive-deps */

  const refresh = (page: number, pageSize?: number) =>
    fetchUpstream(page, pageSize ?? pageInfo.pageSize, groupedQueryParams.freeText || undefined);

  // 删除末页最后一条后按当前页刷新会取回空页：本页行数为 1 且不在第 1 页时回退一页
  const pageAfterRemoval = (rowsOnPage: number, page: number) =>
    rowsOnPage <= 1 && page > 1 ? page - 1 : Math.max(1, page);

  // CRUD 成功后统一刷新两条数据链路（分组 + 平铺）：两视图互为镜像，只刷一侧会让
  // 另一视图显示旧数据。平铺视图未激活时 flat.reload() 不发请求，无额外开销。
  const refreshAll = (pages?: { grouped?: number; flat?: number }) => {
    refresh(pages?.grouped ?? pageInfo.page);
    flat.refresh(pages?.flat ?? flat.page, flat.pageSize);
    flat.reload();
  };

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
      supportOpenAIDecision: ep.supportOpenAIDecision,
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
          supportOpenAIDecision: endpointForm.supportOpenAIDecision,
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
          supportOpenAIDecision: endpointForm.supportOpenAIDecision,
        });
        toast.success(t("endpoints.created_success"));
      }
      setEndpointDialogOpen(false);
      refreshAll();
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
      // 端点是分组视图的行：删除末页最后一条会把当前页刷成空页，需钳制页码
      refreshAll({ grouped: pageAfterRemoval(groups.length, pageInfo.page) });
    },
    onError: (err) => showErrorToast(err, { title: t("endpoints.delete_error") }),
  });

  /* ─── 模型 CRUD ─────────────────────────────────────────────── */

  const openCreateModel = (ep: UpstreamEndpointItem) => {
    setEditingModel(null);
    setModelIdTouched(false);
    setOriginalModelId("");
    setSyncHistory(false);
    setTargetEndpointID(ep.id);
    setModelForm(emptyModelForm);
    setModelDialogOpen(true);
  };

  const openEditModel = (
    model: {
      id: number;
      alias: string;
      modelId?: string;
      upstreamModel: string;
      contextLength: number;
      maxOutputTokens: number;
      capabilities?: string[];
      pricing?: PricingDTO;
      priority?: number;
      weight?: number;
    },
    ep: UpstreamEndpointItem,
  ) => {
    setEditingModel({ id: model.id });
    setModelIdTouched(true);
    setOriginalModelId(model.modelId ?? "");
    setSyncHistory(false);
    setTargetEndpointID(ep.id);
    setModelForm({
      alias: model.alias,
      modelId: model.modelId ?? "",
      upstreamModel: model.upstreamModel,
      contextLength: model.contextLength || DEFAULT_CONTEXT_LENGTH,
      maxOutputTokens: model.maxOutputTokens || DEFAULT_MAX_OUTPUT,
      capabilities: (model.capabilities?.length
        ? [...model.capabilities]
        : ["text"]) as ModelCapability[],
      priority: model.priority ?? 0,
      weight: model.weight ?? 1,
    });
    setModelDialogOpen(true);
  };

  const handleSaveModel = async () => {
    // 端点绑定仅创建必填：更新路径不改绑定（绑定不可移动）；孤儿模型（所属端点
    // 已删、endpoint 缺省）编辑保存时 targetEndpointID 为 0，不应被误判为缺字段
    if (
      !modelForm.alias.trim() ||
      !modelForm.upstreamModel.trim() ||
      (!editingModel && !targetEndpointID)
    ) {
      toast.error(t("models.fields_required"));
      return;
    }
    if (!modelForm.capabilities.includes("text")) {
      toast.error(t("models.capabilities_require_text"));
      return;
    }
    const capabilities = MODEL_CAPABILITIES.filter((c) => modelForm.capabilities.includes(c));
    setSaving(true);
    try {
      if (editingModel) {
        const rsp = await api.updateModel(editingModel.id, {
          alias: modelForm.alias,
          ...(trimmedModelId ? { modelId: trimmedModelId } : {}),
          ...(showSyncHistory && syncHistory ? { syncHistory: true } : {}),
          upstreamModel: modelForm.upstreamModel,
          contextLength: modelForm.contextLength,
          maxOutputTokens: modelForm.maxOutputTokens,
          capabilities,
          priority: modelForm.priority,
          weight: modelForm.weight,
        });
        if (showSyncHistory && syncHistory) {
          toast.success(
            `${t("models.history_synced")}: ${rsp.auditCount} / ${rsp.sessionCount} / ${rsp.messageCount}`,
          );
        } else {
          toast.success(t("models.updated_success"));
        }
      } else {
        await api.createModel({
          alias: modelForm.alias,
          ...(modelForm.modelId.trim() ? { modelId: modelForm.modelId.trim() } : {}),
          upstreamModel: modelForm.upstreamModel,
          endpointID: targetEndpointID,
          contextLength: modelForm.contextLength,
          maxOutputTokens: modelForm.maxOutputTokens,
          capabilities,
          priority: modelForm.priority,
          weight: modelForm.weight,
        });
        toast.success(t("models.created_success"));
      }
      setModelDialogOpen(false);
      refreshAll();
    } catch (err) {
      showErrorToast(err, { title: t("models.save_error") });
    } finally {
      setSaving(false);
    }
  };

  // 收窄为最小形状：分组行（UpstreamModelItem）与平铺行（ModelListItem）都满足，
  // 无需在调用处做类型断言
  interface ModelConfirmTarget {
    model: { id: number; alias: string };
  }

  const deleteModelConfirm = useDeleteConfirm<ModelConfirmTarget>({
    onConfirm: async ({ model }) => {
      await api.deleteModel(model.id);
      toast.success(t("models.deleted_success"));
      // 模型是平铺视图的行：删除末页最后一条会把当前页刷成空页，需钳制页码
      refreshAll({ flat: pageAfterRemoval(flat.items.length, flat.page) });
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

  /* ─── 定价 ──────────────────────────────────────────────────── */

  // 打开定价弹窗：两个视图的行形状不同，这里收窄为导入/展示所需字段
  const openPricing = (m: PricingTarget & { pricing?: PricingDTO }) => {
    setPricingTarget({
      id: m.id,
      alias: m.alias,
      upstreamModel: m.upstreamModel,
    });
    setPricingForm(m.pricing ?? emptyPricing);
    setPricingDialogOpen(true);
  };

  const handleSavePricing = async () => {
    if (!pricingTarget) return;
    setSavingPricing(true);
    try {
      // 只发 pricing 字段：其余字段缺省=不修改，避免用行内旧快照覆盖并发的规格编辑
      await api.updateModel(pricingTarget.id, { pricing: pricingForm });
      toast.success(t("upstream.pricing.saved"));
      setPricingDialogOpen(false);
      refreshAll();
    } catch (err) {
      showErrorToast(err, { title: t("upstream.pricing.save_error") });
    } finally {
      setSavingPricing(false);
    }
  };

  /* ─── 平铺视图专用操作 ──────────────────────────────────────── */

  // 平铺行的开关：没有分组结构可乐观回填，直接改后端后整页重载
  const handleFlatToggle = async (m: ModelListItem) => {
    try {
      await api.updateModel(m.id, { enabled: !m.enabled });
      toast.success(!m.enabled ? t("models.enabled") : t("models.disabled"));
      flat.reload();
    } catch (err) {
      showErrorToast(err, { title: t("models.toggle_error") });
    }
  };

  // 平铺行的编辑：所属端点从行内 endpoint.id 取（更新不改绑定，无需完整端点对象）
  const handleFlatEdit = (m: ModelListItem) => {
    setEditingModel({ id: m.id });
    setModelIdTouched(true);
    setTargetEndpointID(m.endpoint?.id ?? 0);
    setModelForm({
      alias: m.alias,
      modelId: m.modelId ?? "",
      upstreamModel: m.upstreamModel,
      contextLength: m.contextLength || DEFAULT_CONTEXT_LENGTH,
      maxOutputTokens: m.maxOutputTokens || DEFAULT_MAX_OUTPUT,
      capabilities: (m.capabilities?.length ? [...m.capabilities] : ["text"]) as ModelCapability[],
      priority: m.priority ?? 0,
      weight: m.weight ?? 1,
    });
    setModelDialogOpen(true);
  };

  // 与 trigger 页一致：点击 alias 文本复制到剪贴板
  const handleCopyAlias = (alias: string) => {
    if (!alias) return;
    void copyTextToClipboard(alias).then((ok) =>
      ok ? toast.success(t("common.copied_to_clipboard")) : toast.error(t("common.copy_failed")),
    );
  };

  /* ─── 共享渲染片段 ──────────────────────────────────────────── */

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
                {/* demo 只读账户写入口统一锁定（DeleteButton 的 locked 模式） */}
                <Button onClick={openCreateEndpoint} disabled={isDemo()}>
                  {isDemo() ? <Lock className="mr-1 size-4" /> : <Plus className="mr-1 size-4" />}
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
              {/* 视图切换 + faceted bar */}
              <div className="mb-4 flex items-center gap-2">
                <ViewSwitch
                  value={view}
                  onChange={handleViewChange}
                  options={[
                    { value: "grouped", label: t("upstream.view_grouped") },
                    { value: "flat", label: t("upstream.view_flat") },
                  ]}
                />
                <FilterBar
                  {...activeFilterBar}
                  facets={
                    view === "grouped"
                      ? [...missingFacet, ...usernameFacet]
                      : [...missingFacet, ...flatFacets]
                  }
                  placeholder={t("upstream.search_placeholder")}
                />
              </div>
              {activeFilterBar.tokens.length > 0 && (
                <p className="-mt-2 mb-3 text-xs text-muted-foreground">
                  {t("filter_bar.applied_count").replace(
                    "{count}",
                    String(activeFilterBar.tokens.length),
                  )}
                </p>
              )}

              {view === "grouped" ? (
                loading ? (
                  <TableSkeleton />
                ) : groups.length === 0 ? (
                  <ListEmptyState
                    icon={<Layers className="mb-3 size-10 text-muted-foreground/40" />}
                    message={t("upstream.empty")}
                  />
                ) : (
                  <>
                    <GroupedView
                      groups={groups}
                      isMobile={isMobile}
                      isDemo={isDemo()}
                      togglePending={toggleEnabled.updatingKey !== null}
                      onToggleEnabled={(m) => toggleEnabled.apply(m, { enabled: !m.enabled })}
                      onEditEndpoint={openEditEndpoint}
                      onDeleteEndpoint={(ep) => deleteEndpointConfirm.openDelete(ep)}
                      onAddModel={openCreateModel}
                      onEditModel={openEditModel}
                      onPricingModel={openPricing}
                      onDeleteModel={(m) => deleteModelConfirm.openDelete({ model: m })}
                      onCopyAlias={handleCopyAlias}
                      deletingEndpointID={
                        deleteEndpointConfirm.loading ? deleteEndpointConfirm.target?.id : undefined
                      }
                      deletingModelID={
                        deleteModelConfirm.loading ? deleteModelConfirm.target?.model.id : undefined
                      }
                    />

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
                )
              ) : (
                <>
                  <FlatView
                    items={flat.items}
                    loading={flat.loading}
                    isMobile={isMobile}
                    isDemo={isDemo()}
                    sortField={flat.sortField}
                    sort={flat.sort}
                    onSort={flat.toggleSort}
                    onToggleEnabled={handleFlatToggle}
                    onEditModel={handleFlatEdit}
                    onPricingModel={openPricing}
                    onDeleteModel={(m) => deleteModelConfirm.openDelete({ model: m })}
                    onCopyAlias={handleCopyAlias}
                    deletingModelID={
                      deleteModelConfirm.loading ? deleteModelConfirm.target?.model.id : undefined
                    }
                  />

                  <PaginationBar
                    pageInfo={flat.pageInfo}
                    onChange={(page, pageSize) => flat.refresh(page, pageSize)}
                    totalLabel={t("pagination.models")}
                  />
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

          <EndpointDialog
            open={endpointDialogOpen}
            onOpenChange={setEndpointDialogOpen}
            editingId={editingEndpointId}
            form={endpointForm}
            setForm={setEndpointForm}
            userOptions={userOptions}
            isAdmin={isAdmin()}
            saving={saving}
            onSave={handleSaveEndpoint}
          />

          {/* key 重挂载：每次打开弹窗重置 ModelDialog 本地状态（dirty/提示/折叠） */}
          <ModelDialog
            key={`${modelDialogOpen}-${editingModel?.id ?? 0}`}
            open={modelDialogOpen}
            onOpenChange={setModelDialogOpen}
            editing={editingModel !== null}
            form={modelForm}
            setForm={setModelForm}
            onModelIdTouched={() => setModelIdTouched(true)}
            modelIdTouched={modelIdTouched}
            showSyncHistory={showSyncHistory}
            syncHistory={syncHistory}
            onSyncHistoryChange={setSyncHistory}
            saving={saving}
            onSave={handleSaveModel}
          />

          {/* 定价弹窗：与「编辑模型配置」平级的独立入口；key 重挂载重置导入提示 */}
          <PricingDialog
            key={`${pricingDialogOpen}-${pricingTarget?.id ?? 0}`}
            open={pricingDialogOpen}
            onOpenChange={setPricingDialogOpen}
            alias={pricingTarget?.alias ?? ""}
            upstreamModel={pricingTarget?.upstreamModel ?? ""}
            value={pricingForm}
            onChange={setPricingForm}
            saving={savingPricing}
            onSave={handleSavePricing}
          />
        </div>
      </TooltipProvider>
    </PermissionGuard>
  );
}
