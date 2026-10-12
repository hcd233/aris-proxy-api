import { describe, expect, it, vi } from "vitest";
import { emitAmbientPulse, onAmbientPulse } from "./pulse";

describe("ambient pulse bus", () => {
  it("delivers pulses to every subscriber until it unsubscribes", () => {
    const a = vi.fn();
    const b = vi.fn();
    const offA = onAmbientPulse(a);
    const offB = onAmbientPulse(b);

    emitAmbientPulse({ kind: "primary", x: 10, y: 20 });
    expect(a).toHaveBeenCalledWith({ kind: "primary", x: 10, y: 20 });
    expect(b).toHaveBeenCalledTimes(1);

    offA();
    emitAmbientPulse({ kind: "route" });
    expect(a).toHaveBeenCalledTimes(1);
    expect(b).toHaveBeenLastCalledWith({ kind: "route" });
    offB();
  });

  it("is a no-op without subscribers", () => {
    expect(() => emitAmbientPulse({ kind: "intro" })).not.toThrow();
  });
});
