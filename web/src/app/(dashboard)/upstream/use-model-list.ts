import { useCallback, useEffect, useMemo, useState } from "react";
import { api } from "@/lib/api-client";
import { showErrorToast } from "@/lib/api-error-handler";
import { usePersistentState } from "@/hooks/use-persistent-state";
import { useT } from "@/lib/i18n";
import type { ModelListSortField, ModelListItem, PageInfo } from "@/lib/types";
import {
  DEFAULT_MODEL_SORT_DIR,
  DEFAULT_MODEL_SORT_FIELD,
  buildModelListParams,
  nextSortState,
} from "./model-list-params";

export { nextSortState } from "./model-list-params";

/** 平铺视图数据：自己的真分页与排序状态，与分组视图互不干扰 */
export function useModelList(opts: {
  freeText: string;
  params: Record<string, string>;
  enabled: boolean;
}) {
  const t = useT();
  const { freeText, params, enabled } = opts;

  const [page, setPage] = usePersistentState("dashboard.upstream.flat.page", 1);
  const [pageSize, setPageSize] = usePersistentState("dashboard.upstream.flat.pageSize", 10);
  const [sortField, setSortField] = usePersistentState<ModelListSortField>(
    "dashboard.upstream.flat.sortField",
    DEFAULT_MODEL_SORT_FIELD,
  );
  const [sort, setSort] = usePersistentState<"asc" | "desc">(
    "dashboard.upstream.flat.sort",
    DEFAULT_MODEL_SORT_DIR,
  );

  const [items, setItems] = useState<ModelListItem[]>([]);
  const [pageInfo, setPageInfo] = useState<PageInfo>({ page, pageSize, total: 0 });
  const [loading, setLoading] = useState(true);

  // facet 签名：仅当实际生效的筛选值变化时才重拉（避免对象引用变化导致抖动）
  const facetSig = useMemo(
    () =>
      [
        params.status ?? "",
        params.capability ?? "",
        params.endpointID ?? params.endpoint ?? "",
        params.username ?? "",
      ].join("|"),
    [params],
  );

  // 显式重载计数器：CRUD 后页码可能未变，refresh 不会触发 effect，需要它
  const [reloadTick, setReloadTick] = useState(0);
  const reload = useCallback(() => setReloadTick((n) => n + 1), []);

  const load = useCallback(() => {
    if (!enabled) return;
    setLoading(true);
    api
      .listModelsPage(
        buildModelListParams({ page, pageSize, freeText, params, sortField, sort }),
      )
      .then((res) => {
        setItems(res.items ?? []);
        setPageInfo(res.pageInfo ?? { page, pageSize, total: 0 });
      })
      .catch((err) => showErrorToast(err, { title: t("upstream.load_error") }))
      .finally(() => setLoading(false));
    // facetSig 代表 params 的生效内容，替代 params 引用本身作为依赖
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [enabled, freeText, facetSig, page, pageSize, sortField, sort, t, reloadTick]);

  useEffect(() => {
    // 与上游列表页同一模式：effect 内同步 setState 会触发级联渲染，此处为刻意的数据加载入口
    // eslint-disable-next-line react-hooks/set-state-in-effect
    load();
  }, [load]);

  const toggleSort = useCallback(
    (field: ModelListSortField) => {
      const next = nextSortState({ sortField, sort }, field);
      setSortField(next.sortField);
      setSort(next.sort);
      setPage(1);
    },
    [sortField, sort, setSortField, setSort, setPage],
  );

  const refresh = useCallback(
    (p: number, ps: number) => {
      setPage(p);
      setPageSize(ps);
    },
    [setPage, setPageSize],
  );

  return {
    items,
    pageInfo,
    loading,
    page,
    pageSize,
    sortField,
    sort,
    toggleSort,
    refresh,
    reload,
    reloadTick,
  };
}
