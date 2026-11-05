import { isSafeUrl } from "../../lib/utils";

/** message 中已外置图片的引用前缀（与后端 constant.BlobRefPrefix 一致） */
export const BLOB_REF_PREFIX = "aris-blob:sha256:";

const SHA256_HEX = /^[0-9a-f]{64}$/;

/** 解析引用形态，返回 sha256；非引用或格式不对返回 null。 */
export function parseBlobRef(url: string | undefined): string | null {
  if (!url?.startsWith(BLOB_REF_PREFIX)) return null;
  const sha = url.slice(BLOB_REF_PREFIX.length);
  return SHA256_HEX.test(sha) ? sha : null;
}

function formatBytes(size: number): string {
  if (size < 1024) return `${size} B`;
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`;
  return `${(size / 1024 / 1024).toFixed(1)} MB`;
}

/** 占位卡片上的「类型 · 大小」，如 PNG · 1.2 MB；缺失的部分省略。 */
export function formatImageMeta(mime?: string, size?: number): string {
  const kind = mime?.split("/")[1]?.toUpperCase();
  const bytes = typeof size === "number" && size > 0 ? formatBytes(size) : "";
  return [kind, bytes].filter(Boolean).join(" · ");
}

/** 只放行接口返回的 http(s) 链接或 data:image/ 开头的 data URL。 */
export function pickImageSrc(rsp: { url?: string; dataUrl?: string }): string | null {
  if (rsp.url && isSafeUrl(rsp.url)) return rsp.url;
  if (rsp.dataUrl?.startsWith("data:image/")) return rsp.dataUrl;
  return null;
}
