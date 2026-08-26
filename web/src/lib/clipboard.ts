/**
 * 复制文本到剪贴板（全站统一入口）。
 *
 * 优先使用 Clipboard API；非安全上下文（如 HTTP 调试环境）中
 * `navigator.clipboard` 为 undefined，此时降级 `document.execCommand("copy")`。
 * 各页面不得直接调用 `navigator.clipboard.writeText`——裸调用在
 * navigator.clipboard 不可用时会在 `.then` 之前抛 TypeError。
 *
 * @param text 要复制的文本
 * @returns 是否复制成功（调用方据此提示 toast）
 */
export async function copyTextToClipboard(text: string): Promise<boolean> {
  if (typeof navigator !== "undefined" && navigator.clipboard) {
    try {
      await navigator.clipboard.writeText(text);
      return true;
    } catch {
      return false;
    }
  }

  // 降级路径：临时 textarea + execCommand（deprecated 但兼容非安全上下文）
  try {
    const el = document.createElement("textarea");
    el.value = text;
    el.setAttribute("readonly", "");
    el.style.position = "fixed";
    el.style.opacity = "0";
    document.body.appendChild(el);
    el.select();
    const ok = document.execCommand("copy");
    document.body.removeChild(el);
    return ok;
  } catch {
    return false;
  }
}
