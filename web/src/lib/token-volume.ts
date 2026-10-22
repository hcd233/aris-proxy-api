import type { TokenThroughputPoint } from "@/lib/types";

/**
 * 后端四维 token 互斥：`inputTokens` 落库时已归一化为净输入（不含缓存读/写），
 * `cacheReadTokens` / `cacheCreationTokens` / `outputTokens` 各自独立。
 * 图表直接使用原值，禁止再做 `input − cacheRead` 一类的二次扣减。
 */
export function toTokenVolumeRows(data: TokenThroughputPoint[]) {
  return data.map(({ time, inputTokens, outputTokens, cacheReadTokens, cacheCreationTokens }) => ({
    time,
    inputTokens,
    outputTokens,
    cacheReadTokens,
    cacheCreationTokens,
  }));
}

/**
 * 相对亮度（WCAG），用于把堆叠层按「深色在下、浅色在上」排序。
 * 只接受 6 位十六进制颜色（图表色板即此格式）。
 */
export function colorLuminance(hex: string): number {
  const value = hex.replace("#", "");
  const [r, g, b] = [0, 2, 4].map((offset) => {
    const channel = parseInt(value.slice(offset, offset + 2), 16) / 255;
    return channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4;
  });
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}
