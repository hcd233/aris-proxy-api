import { describe, expect, it } from "vitest";
import type { TokenThroughputPoint } from "@/lib/types";
import { colorLuminance, toTokenVolumeRows } from "./token-volume";

function point(overrides: Partial<TokenThroughputPoint> = {}): TokenThroughputPoint {
  return {
    time: "2026-10-01T00:00:00Z",
    inputTokens: 12,
    outputTokens: 7,
    cacheReadTokens: 900,
    cacheCreationTokens: 40,
    ...overrides,
  };
}

describe("toTokenVolumeRows", () => {
  it("keeps the four mutually exclusive dimensions unmodified", () => {
    const [row] = toTokenVolumeRows([point()]);
    expect(row).toEqual(point());
  });

  it("never subtracts cache tokens when cache read exceeds input", () => {
    const [row] = toTokenVolumeRows([point({ inputTokens: 5, cacheReadTokens: 1_000 })]);
    expect(row.inputTokens).toBe(5);
    expect(row.cacheReadTokens).toBe(1_000);
  });
});

describe("colorLuminance", () => {
  it("orders the shared token palette from dark to light", () => {
    // anthropic 四维色板：output → input → cacheWrite → cacheRead
    const luminances = ["#5C4A3D", "#B8654A", "#CDBBA6", "#EFC5AC"].map(colorLuminance);
    expect(luminances).toEqual([...luminances].sort((a, b) => a - b));
  });
});
