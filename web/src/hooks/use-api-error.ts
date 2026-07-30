/**
 * use-api-error.ts — 管理异步操作状态（idle → loading → success / error）。
 *
 * 用法:
 * ```tsx
 * const { execute, loading, error, clearError } = useApiError(api.listSessions);
 * const data = await execute(params);
 * if (data) { console.log('success'); }
 * ```
 */
"use client";

import { useCallback, useRef, useState } from "react";
import { parseError, type StructuredError } from "../lib/api-error-handler";

// ── 状态类型 ──────────────────────────────────────────────────────────────────

export type AsyncStatus = "idle" | "loading" | "success" | "error";

export interface AsyncState<T = unknown> {
  /** 当前执行状态 */
  status: AsyncStatus;
  /** 是否正在加载 */
  loading: boolean;
  /** 最近一次成功的返回值 */
  data: T | null;
  /** 最近一次错误的结构化信息 */
  error: StructuredError | null;
}

// ── Hook ──────────────────────────────────────────────────────────────────────

const IDLE_STATE: AsyncState = { status: "idle", loading: false, data: null, error: null };

export interface UseApiErrorOptions {
  /** 是否在组件挂载时立即执行 */
  immediate?: boolean;
  /** immediate 模式下的参数（数组形式传入） */
  immediateArgs?: unknown[];
}

export interface UseApiErrorReturn<T> extends AsyncState<T> {
  /** 执行异步操作，自动管理 loading/error 状态 */
  execute: (...args: unknown[]) => Promise<T | null>;
  /** 清除 error 状态 */
  clearError: () => void;
  /** 重置到闲置状态 */
  reset: () => void;
}

/**
 * 管理异步 API 操作的状态机。
 * 自动拦截错误并提供结构化 error 对象。
 */
export function useApiError<T = unknown>(
  fn?: (...args: unknown[]) => Promise<T>,
  options?: UseApiErrorOptions,
): UseApiErrorReturn<T> {
  const [state, setState] = useState<AsyncState<T>>(IDLE_STATE as AsyncState<T>);
  const mountedRef = useRef(true);

  const execute = useCallback(
    async (...args: unknown[]): Promise<T | null> => {
      setState((s) => ({ ...s, status: "loading", loading: true, error: null }));

      try {
        // 支持两种调用方式:
        // 1. 传入异步函数
        // 2. 第一个参数是异步函数（当没有绑定 fn 时）
        const actualFn = fn ?? (args[0] as (...a: unknown[]) => Promise<T>);
        const actualArgs = fn ? args : (args.slice(1) as unknown[]);

        const result = await actualFn(...actualArgs);

        if (mountedRef.current) {
          setState({ status: "success", loading: false, data: result, error: null });
        }
        return result;
      } catch (err) {
        const parsed = parseError(err);
        if (mountedRef.current) {
          setState({ status: "error", loading: false, data: null, error: parsed });
        }
        return null;
      }
    },
    [fn],
  );

  const clearError = useCallback(() => {
    setState((s) => ({ ...s, error: null, status: s.data ? "success" : "idle" }));
  }, []);

  const reset = useCallback(() => {
    setState(IDLE_STATE as AsyncState<T>);
  }, []);

  return { ...state, execute, clearError, reset };
}
