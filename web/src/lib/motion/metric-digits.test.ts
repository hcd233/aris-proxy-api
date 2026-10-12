import { describe, expect, it } from "vitest";
import { planDigits } from "./metric-digits";

describe("planDigits", () => {
  it("first render rolls every digit up from 0", () => {
    expect(planDigits(null, 305)).toEqual({
      direction: 1,
      cells: [
        { from: "0", to: "3", changed: true },
        { from: "0", to: "0", changed: false },
        { from: "0", to: "5", changed: true },
      ],
    });
  });

  it("only the changed digits move on an update", () => {
    const plan = planDigits(1284, 1291);
    expect(plan.direction).toBe(1);
    expect(plan.cells.map((c) => c.changed)).toEqual([false, false, true, true]);
    expect(plan.cells[2]).toEqual({ from: "8", to: "9", changed: true });
  });

  it("aligns on the right when the number grows a digit", () => {
    const plan = planDigits(99, 100);
    expect(plan.cells).toEqual([
      { from: "0", to: "1", changed: true },
      { from: "9", to: "0", changed: true },
      { from: "9", to: "0", changed: true },
    ]);
  });

  it("reports a decrease and drops vanished leading digits", () => {
    const plan = planDigits(120, 7);
    expect(plan.direction).toBe(-1);
    expect(plan.cells).toEqual([{ from: "0", to: "7", changed: true }]);
  });

  it("is a no-op when the value is unchanged", () => {
    const plan = planDigits(42, 42);
    expect(plan.direction).toBe(0);
    expect(plan.cells.every((c) => !c.changed)).toBe(true);
  });

  it("keeps non-digit glyphs static", () => {
    const plan = planDigits(5, -12);
    expect(plan.cells[0]).toEqual({ from: "-", to: "-", changed: false });
  });
});
