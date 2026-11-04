"use client";

import { useCallback, useState } from "react";
import { copyTextToClipboard } from "@/lib/clipboard";

/**
 * 复制到剪贴板 + 「已复制」短暂态。toast 文案各处不同，由调用方根据返回值自行提示。
 *
 * `key` 区分同一组件里的多个复制目标（如表格每行一个按钮），默认取被复制的文本；
 * 单目标场景直接用 `copiedKey !== null` 判断。
 */
export function useCopyFeedback(resetMs = 2000) {
  const [copiedKey, setCopiedKey] = useState<string | null>(null);

  const copy = useCallback(
    async (text: string, key: string = text): Promise<boolean> => {
      const ok = await copyTextToClipboard(text);
      if (ok) {
        setCopiedKey(key);
        window.setTimeout(() => setCopiedKey((cur) => (cur === key ? null : cur)), resetMs);
      }
      return ok;
    },
    [resetMs],
  );

  const reset = useCallback(() => setCopiedKey(null), []);

  return { copiedKey, copy, reset };
}
