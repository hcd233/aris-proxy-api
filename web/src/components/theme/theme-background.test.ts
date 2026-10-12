import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

const globals = readFileSync(new URL("../../app/globals.css", import.meta.url), "utf8");
const layout = readFileSync(new URL("../../app/layout.tsx", import.meta.url), "utf8");
const interactionLayer = readFileSync(
  new URL("../motion/interaction-layer.tsx", import.meta.url),
  "utf8",
);
const login = readFileSync(new URL("../../app/login/page.tsx", import.meta.url), "utf8");

describe("theme backgrounds", () => {
  it("uses the solid theme token as the only page backdrop", () => {
    expect(globals).toContain("--background: #faf8f5;");
    expect(globals).toContain("--background: #000000;");
    expect(globals).toMatch(/body\s*\{\s*@apply bg-background text-foreground;/);
    expect(globals).toMatch(/:where\(\.page-surface\)\s*\{\s*background-color:\s*transparent;/);
    expect(layout).not.toContain("AmbientStage");
    expect(globals).not.toContain(".ambient-stage");
    expect(globals).not.toContain("feTurbulence");
    expect(interactionLayer).not.toContain("AmbientPulse");
    expect(login).not.toContain("AmbientPulse");
  });
});
