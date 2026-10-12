import { describe, expect, it } from "vitest";
import { magnetTarget, tiltFor } from "./interaction";

const BTN = { left: 100, top: 100, width: 80, height: 32 }; // center (140, 116)

describe("magnetTarget", () => {
  it("is zero beyond the 60px range", () => {
    expect(magnetTarget(BTN, 300, 116)).toEqual({ x: 0, y: 0 });
    expect(magnetTarget(BTN, 140, 116 + 16 + 60)).toEqual({ x: 0, y: 0 });
  });

  it("pulls toward the pointer, clamped to 4px", () => {
    const t = magnetTarget(BTN, 175, 116); // inside, 35px right of center
    expect(t.x).toBeCloseTo(4, 5);
    expect(t.y).toBe(0);
  });

  it("falls off linearly with the distance to the edge", () => {
    const near = magnetTarget(BTN, 180 + 15, 116); // 15px outside the right edge
    const far = magnetTarget(BTN, 180 + 45, 116); // 45px outside
    expect(near.x).toBeCloseTo(4 * (1 - 15 / 60), 5);
    expect(far.x).toBeCloseTo(4 * (1 - 45 / 60), 5);
  });

  it("points up-left when the pointer is up-left", () => {
    const t = magnetTarget(BTN, 110, 104);
    expect(t.x).toBeLessThan(0);
    expect(t.y).toBeLessThan(0);
  });
});

describe("tiltFor", () => {
  const CARD = { left: 0, top: 0, width: 200, height: 100 };

  it("is flat at the center", () => {
    const t = tiltFor(CARD, 100, 50);
    expect(t.rx).toBeCloseTo(0, 5);
    expect(t.ry).toBeCloseTo(0, 5);
  });

  it("presses the edge under the pointer away, up to 4°", () => {
    expect(tiltFor(CARD, 100, 0).rx).toBeCloseTo(4, 5); // top edge away
    expect(tiltFor(CARD, 200, 50).ry).toBeCloseTo(4, 5); // right edge away
    expect(tiltFor(CARD, 0, 100)).toEqual({ rx: -4, ry: -4 });
  });

  it("clamps when the pointer is outside the card", () => {
    expect(tiltFor(CARD, 400, -50)).toEqual({ rx: 4, ry: 4 });
  });
});
