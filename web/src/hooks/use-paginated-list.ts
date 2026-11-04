"use client";

import { useCallback, useLayoutEffect, useRef, useState } from "react";
import { showErrorToast } from "@/lib/api-error-handler";
import type { PageInfo } from "@/lib/types";
import { useRequestSeq } from "@/hooks/use-request-seq";

/** 一页数据：api-client 的业务错误以 `error` 字段返回，`extra` 携带列表之外的附加字段。 */
export interface PageResult<T, X = undefined> {
  items?: T[];
  pageInfo?: PageInfo;
  error?: unknown;
  extra?: X;
}

export interface UsePaginatedListOptions<T, Q, X> {
  fetchPage: (page: number, pageSize: number, query: Q) => Promise<PageResult<T, X>>;
  /** 页码与页大小持久化到 localStorage 的 `${persistKey}.page` / `.pageSize`；不传则不持久化 */
  persistKey?: string;
  /** 加载失败 toast 标题；不传则静默失败 */
  errorTitle?: string;
  defaultPageSize?: number;
}

export interface LoadOptions {
  /** 不切换 loading（后台刷新时保持表格不闪骨架） */
  silent?: boolean;
}

function readStored(key: string | undefined, fallback: number): number {
  if (!key || typeof window === "undefined") return fallback;
  try {
    const v: unknown = JSON.parse(localStorage.getItem(key) ?? "null");
    return typeof v === "number" ? v : fallback;
  } catch {
    return fallback;
  }
}

function writeStored(key: string | undefined, value: number) {
  if (!key) return;
  try {
    localStorage.setItem(key, JSON.stringify(value));
  } catch {
    // storage quota exceeded, ignore silently
  }
}

/**
 * 分页列表数据层：页码持久化 + 竞态守卫（慢响应不覆盖新响应）+ 错误 toast + loading。
 *
 * 查询条件由调用方经 `load(page, pageSize, query)` 显式传入，不读闭包里的 state——
 * 切换排序/时间范围时 setState 尚未生效，必须把新值直接带进请求。
 * `load` 引用永久稳定；返回本次结果，过期或失败时返回 undefined。
 */
export function usePaginatedList<T, Q = void, X = undefined>(
  options: UsePaginatedListOptions<T, Q, X>,
) {
  const { persistKey, defaultPageSize = 20 } = options;
  const pageKey = persistKey && `${persistKey}.page`;
  const pageSizeKey = persistKey && `${persistKey}.pageSize`;

  const [items, setItems] = useState<T[]>([]);
  const [pageInfo, setPageInfo] = useState<PageInfo>(() => ({
    page: readStored(pageKey, 1),
    pageSize: readStored(pageSizeKey, defaultPageSize),
    total: 0,
  }));
  const [loading, setLoading] = useState(true);

  const optionsRef = useRef(options);
  useLayoutEffect(() => {
    optionsRef.current = options;
  });
  const requestSeq = useRequestSeq();

  const load = useCallback(
    async (
      page: number,
      pageSize: number,
      query: Q,
      { silent = false }: LoadOptions = {},
    ): Promise<PageResult<T, X> | undefined> => {
      const { fetchPage, errorTitle, persistKey: key } = optionsRef.current;
      const seq = requestSeq.begin();
      if (!silent) setLoading(true);
      try {
        const rsp = await fetchPage(page, pageSize, query);
        if (!requestSeq.isCurrent(seq)) return undefined;
        if (rsp.error) {
          if (errorTitle !== undefined) showErrorToast(rsp.error, { title: errorTitle });
          return undefined;
        }
        setItems(rsp.items ?? []);
        if (rsp.pageInfo) {
          setPageInfo(rsp.pageInfo);
          writeStored(key && `${key}.page`, rsp.pageInfo.page);
          writeStored(key && `${key}.pageSize`, rsp.pageInfo.pageSize);
        }
        return rsp;
      } catch (err) {
        if (requestSeq.isCurrent(seq) && errorTitle !== undefined) {
          showErrorToast(err, { title: errorTitle });
        }
        return undefined;
      } finally {
        if (requestSeq.isCurrent(seq)) setLoading(false);
      }
    },
    [requestSeq],
  );

  return { items, setItems, pageInfo, loading, load };
}
