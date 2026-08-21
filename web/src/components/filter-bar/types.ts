import type { FilterToken } from "./filter-dsl";

/** 列表页筛选维度的声明式配置 */
export interface FacetDef {
  /** 后端 DSL 字段名（如 "score" / "model" / "messageCount"） */
  key: string;
  /** i18n 后的展示标签（如 "评分"），参与 facet 建议匹配 */
  label: string;
  /** 静态选项，或异步加载（展开值建议时调用，结果缓存至 optionsCacheKey 变化） */
  options: string[] | (() => Promise<string[]>);
  /** 序列化目标：filter DSL（默认）或独立 query 参数（如 users 页 permission） */
  target?: "filter" | "param";
  /** target="param" 时的 query 参数名，默认同 key */
  paramName?: string;
  /** true 时同 key 只保留一个 token（选新值即替换旧值），用于单选参数 */
  single?: boolean;
  /** token 与值建议的展示格式化（如 "5" → "★5"、"unscored" → 未评分） */
  formatValue?: (value: string) => string;
}

/** 筛选状态的结构化输出，页面合并进 list 请求 */
export interface FilterBarQueryParams {
  /** filter DSL 字符串；无 filter 类 token 时 undefined */
  filter?: string;
  /** 自由文本（关键词）；无关键词 token 时 "" */
  freeText: string;
  /** target="param" facet 的 query 参数集合 */
  params: Record<string, string>;
}

export type { FilterToken };
