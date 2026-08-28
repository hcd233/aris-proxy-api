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
import { Plus, Layers } from "lucide-react";
import { toast } from "sonner";
import { useIsMobile } from "@/hooks/use-mobile";
import { useT } from "@/lib/i18n";
import { copyTextToClipboard } from "@/lib/clipboard";

import { emptyEndpointForm, emptyModelForm } from "./shared";
import type { EndpointForm, ModelForm } from "./shared";
import { EndpointDialog } from "./endpoint-dialog";
import { ModelDialog } from "./model-dialog";
import { GroupedView } from "./grouped-view";

const DEFAULT_PAGE_SIZE = 10;
const VALID_PAGE_SIZES = [10, 20, 50];
// admin 代建下拉的用户列表一次性拉取上限
const USER_FETCH_LIMIT = 500;


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
                  onDeleteModel={(m) => deleteModelConfirm.openDelete({ model: m })}
                  onCopyAlias={handleCopyAlias}
                  deletingEndpointID={
                    deleteEndpointConfirm.loading ? deleteEndpointConfirm.target?.id : undefined
                  }
                  deletingModelID={
                    deleteModelConfirm.loading
                      ? deleteModelConfirm.target?.model.id
                      : undefined
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

          <ModelDialog
            open={modelDialogOpen}
            onOpenChange={setModelDialogOpen}
            editing={editingModel !== null}
            form={modelForm}
            setForm={setModelForm}
            onModelIdTouched={() => setModelIdTouched(true)}
            modelIdTouched={modelIdTouched}
            saving={saving}
            onSave={handleSaveModel}
          />
        </div>
      </TooltipProvider>
    </PermissionGuard>
  );
}
