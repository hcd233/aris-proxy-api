"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { usePersistentState } from "@/hooks/use-persistent-state";
import { migrateLegacyTokens, serializeTokens, type FilterToken } from "./filter-dsl";
import type { FacetDef, FilterBarQueryParams } from "./types";

export interface UseFilterBarOptions {
  /** 持久化命名空间（如 "dashboard.sessions"），token 存 "<persistKey>.filters" */
  persistKey: string;
  facets: FacetDef[];
  /** 自由文本（关键词）输入占位；不传则 bar 不接受自由文本 */
  freeTextPlaceholder?: string;
  /** 旧 localStorage key 迁移：facetKey → 旧 key（值为 string[]） */
  legacyKeys?: Record<string, string>;
  /** 旧关键词 key（值为 string，如 "dashboard.sessions.keyword"） */
  legacyFreeTextKey?: string;
  /** 异步选项缓存失效指纹（如 `${startTime}:${endTime}`），变化即清空缓存 */
  optionsCacheKey?: string;
}

export interface UseFilterBarReturn {
  tokens: FilterToken[];
  addToken: (token: FilterToken) => void;
  removeToken: (index: number) => void;
  clearTokens: () => void;
  /** 展开某 facet 值建议时调用；静态选项直接返回，异步选项带缓存 */
  loadOptions: (facet: FacetDef) => Promise<string[]>;
  /** 正在异步加载选项的 facet key（无则 null） */
  loadingKey: string | null;
  /** 结构化查询参数；identity 仅在 tokens 变化时更新（供页面 effect 依赖） */
  queryParams: FilterBarQueryParams;
}

/** 读取旧 localStorage 数据并迁移为 token（仅在新 key 不存在时作为默认值生效） */
function readLegacyTokens(opts: UseFilterBarOptions): FilterToken[] {
  if (typeof window === "undefined") return [];
  try {
    const legacy: Record<string, unknown> = {};
    for (const [facetKey, storageKey] of Object.entries(opts.legacyKeys ?? {})) {
      const raw = localStorage.getItem(storageKey);
      if (raw !== null) legacy[facetKey] = JSON.parse(raw);
    }
    if (opts.legacyFreeTextKey) {
      const raw = localStorage.getItem(opts.legacyFreeTextKey);
      if (raw !== null) legacy.keyword = JSON.parse(raw);
    }
    return migrateLegacyTokens(legacy);
  } catch {
    return [];
  }
}

export function useFilterBar(opts: UseFilterBarOptions): UseFilterBarReturn {
  /* eslint-disable-next-line react-hooks/exhaustive-deps -- 仅挂载时读取一次旧数据做迁移 */
  const initialTokens = useMemo(() => readLegacyTokens(opts), []);
  const [tokens, setTokens] = usePersistentState<FilterToken[]>(
    `${opts.persistKey}.filters`,
    initialTokens,
  );

  // facets 每渲染都是新数组，事件回调经 ref 读取最新值（react-hooks/refs 禁 render 期读写，改在 effect 同步）
  const facetsRef = useRef(opts.facets);
  useEffect(() => {
    facetsRef.current = opts.facets;
  });

  // 挂载后清理旧 key（迁移产物已由 usePersistentState 的 effect 写入新 key）
  /* eslint-disable react-hooks/exhaustive-deps -- 仅挂载时执行一次清理 */
  useEffect(() => {
    const staleKeys = [...Object.values(opts.legacyKeys ?? {})];
    if (opts.legacyFreeTextKey) staleKeys.push(opts.legacyFreeTextKey);
    for (const key of staleKeys) {
      try {
        localStorage.removeItem(key);
      } catch {
        // 隐私模式等场景忽略
      }
    }
  }, []);
  /* eslint-enable react-hooks/exhaustive-deps */

  // optionsCacheKey 变化时清空异步选项缓存
  const optionsCacheRef = useRef(new Map<string, string[]>());
  const prevCacheKeyRef = useRef(opts.optionsCacheKey);
  useEffect(() => {
    if (prevCacheKeyRef.current !== opts.optionsCacheKey) {
      prevCacheKeyRef.current = opts.optionsCacheKey;
      optionsCacheRef.current.clear();
    }
  }, [opts.optionsCacheKey]);

  const [loadingKey, setLoadingKey] = useState<string | null>(null);

  const loadOptions = useCallback(async (facet: FacetDef): Promise<string[]> => {
    if (Array.isArray(facet.options)) return facet.options;
    const cached = optionsCacheRef.current.get(facet.key);
    if (cached) return cached;
    setLoadingKey(facet.key);
    try {
      const items = (await facet.options()) ?? [];
      optionsCacheRef.current.set(facet.key, items);
      return items;
    } catch {
      return [];
    } finally {
      setLoadingKey(null);
    }
  }, []);

  const addToken = useCallback(
    (token: FilterToken) => {
      setTokens((prev) => {
        const facet = token.key ? facetsRef.current.find((f) => f.key === token.key) : undefined;
        // 自由文本与 single facet：同 key 替换而非追加
        const replace = token.key === null || facet?.single;
        const base = replace ? prev.filter((t) => t.key !== token.key) : prev;
        if (base.some((t) => t.key === token.key && t.value === token.value)) return base;
        return [...base, token];
      });
    },
    [setTokens],
  );

  const removeToken = useCallback(
    (index: number) => setTokens((prev) => prev.filter((_, i) => i !== index)),
    [setTokens],
  );

  const clearTokens = useCallback(() => setTokens([]), [setTokens]);

  // queryParams 仅依赖 facet 的 key/target/paramName（页面内静态），序列化为签名作为
  // memo 依赖——既避免 facets 数组 identity 每渲染变化导致的失效，也规避 render 期读 ref
  const facetSignature = opts.facets
    .map((f) => `${f.key}|${f.target ?? "filter"}|${f.paramName ?? ""}`)
    .join(";");

  const queryParams = useMemo<FilterBarQueryParams>(() => {
    const facetList = facetSignature
      ? facetSignature.split(";").map((s) => {
          const [key, target, paramName] = s.split("|");
          return { key, target, paramName: paramName || undefined };
        })
      : [];
    const filter = serializeTokens(
      tokens.filter((t) => {
        if (t.key === null) return false;
        return facetList.find((f) => f.key === t.key)?.target !== "param";
      }),
    );
    const params: Record<string, string> = {};
    for (const facet of facetList) {
      if (facet.target !== "param") continue;
      const token = tokens.find((t) => t.key === facet.key);
      if (token) params[facet.paramName ?? facet.key] = token.value;
    }
    const freeText = tokens.find((t) => t.key === null)?.value ?? "";
    return { filter, freeText, params };
  }, [tokens, facetSignature]);

  return { tokens, addToken, removeToken, clearTokens, loadOptions, loadingKey, queryParams };
}
