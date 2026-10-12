import { describe, expect, it } from "vitest";
import { revealRadius } from "./theme-transition";

describe("revealRadius", () => {
  it("reaches the farthest corner from a bottom-left switch", () => {
    expect(revealRadius(24, 776, 1280, 800)).toBeCloseTo(Math.hypot(1256, 776), 6);
  });

  it("is half the diagonal from the center", () => {
    expect(revealRadius(640, 400, 1280, 800)).toBeCloseTo(Math.hypot(640, 400), 6);
  });
});
