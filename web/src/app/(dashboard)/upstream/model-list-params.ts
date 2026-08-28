/**
 * 平铺模型列表的纯逻辑（无 React / 无项目内 import），便于 vitest 直接测。
 *
 * 刻意与 use-model-list.ts 分开：vitest 在本仓库是最小接入，没有配路径别名
 * （@/ 解析不到），任何 import "@/..." 的模块都无法被测试直接加载。
 */
import type { ModelCapability, ModelListSortField } from "@/lib/types";

/** 后端 status 筛选合法取值（与 DTO 枚举一致） */
const STATUS_VALUES = ["enabled", "disabled"] as const;
/** 后端 capability 筛选合法取值 */
const CAPABILITY_VALUES = ["text", "image"] as const;

export const DEFAULT_MODEL_SORT_FIELD: ModelListSortField = "created_at";
export const DEFAULT_MODEL_SORT_DIR = "desc" as const;

export interface ModelListParamsInput {
  page: number;
  pageSize: number;
  freeText: string;
  params: Record<string, string>;
  sortField: ModelListSortField;
  sort: "asc" | "desc";
}

export interface ModelListApiParams {
  page: number;
  pageSize: number;
  query?: string;
  sortField?: ModelListSortField;
  sort?: "asc" | "desc";
  status?: "enabled" | "disabled";
  endpointID?: number;
  capability?: ModelCapability;
  username?: string;
}

/**
 * 组装模型列表请求参数。
 *
 * 白名单外的 status/capability 一律丢弃：后端虽会忽略未知值，
 * 但前端先净化可避免"传了筛选项却什么都没过滤"的静默错觉。
 */
export function buildModelListParams(input: ModelListParamsInput): ModelListApiParams {
  const { page, pageSize, freeText, params, sortField, sort } = input;
  const out: ModelListApiParams = { page, pageSize, sortField, sort };

  if (freeText.trim()) out.query = freeText.trim();

  if (params.status && (STATUS_VALUES as readonly string[]).includes(params.status)) {
    out.status = params.status as ModelListApiParams["status"];
  }
  if (params.capability && (CAPABILITY_VALUES as readonly string[]).includes(params.capability)) {
    out.capability = params.capability as ModelCapability;
  }
  if (params.username) out.username = params.username;

  // endpoint facet 存的是端点 ID 字符串（paramName=endpointID）
  const epID = Number(params.endpointID ?? params.endpoint);
  if (Number.isInteger(epID) && epID > 0) out.endpointID = epID;

  return out;
}

/**
 * 点击列头的排序状态迁移：同列反向，换列重置为 asc。
 * 换列时不保留方向——用户换列通常意味着换关注点，沿用旧方向反直觉。
 */
export function nextSortState(
  cur: { sortField: ModelListSortField; sort: "asc" | "desc" },
  clicked: ModelListSortField,
): { sortField: ModelListSortField; sort: "asc" | "desc" } {
  return cur.sortField === clicked
    ? { sortField: clicked, sort: cur.sort === "asc" ? "desc" : "asc" }
    : { sortField: clicked, sort: "asc" };
}
