"use client";

import { useCallback, useMemo, useRef } from "react";

export interface RequestSeqGuard {
  /** 发起新请求：自增序号并作废所有在途请求，返回本次请求序号 */
  begin: () => number;
  /** 该序号是否仍是最新请求；过期响应（含错误 toast 与 setLoading）一律丢弃 */
  isCurrent: (seq: number) => boolean;
}

/**
 * 列表请求竞态守卫：连点筛选/翻页时，慢响应不得覆盖新响应。
 *
 * 语义与 upstream 页 fetchSeqRef / use-model-list loadSeqRef 一致：
 * 请求发起前 `const seq = guard.begin()`；成功、失败（含错误 toast）与
 * finally 的 setLoading 都必须先经 `guard.isCurrent(seq)` 校验，过期即 return。
 * 返回引用永久稳定，可安全写入 useCallback 依赖数组。
 *
 * 用法：
 *   const guard = useRequestSeq();
 *   const fetchX = useCallback(async () => {
 *     const seq = guard.begin();
 *     setLoading(true);
 *     try {
 *       const rsp = await api.listX();
 *       if (!guard.isCurrent(seq)) return;
 *       setItems(rsp.items ?? []);
 *     } catch (err) {
 *       if (!guard.isCurrent(seq)) return;
 *       showErrorToast(err);
 *     } finally {
 *       if (guard.isCurrent(seq)) setLoading(false);
 *     }
 *   }, [guard]);
 */
export function useRequestSeq(): RequestSeqGuard {
  const seqRef = useRef(0);
  const begin = useCallback(() => ++seqRef.current, []);
  const isCurrent = useCallback((seq: number) => seq === seqRef.current, []);
  return useMemo(() => ({ begin, isCurrent }), [begin, isCurrent]);
}
