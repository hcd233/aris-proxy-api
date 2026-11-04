import { describe, expect, it } from "vitest";
import type { MessageItem } from "@/lib/types";
import { groupTranscript, isToolResultMessage } from "./transcript-groups";

let nextID = 1;
function msg(role: string, extra: Partial<MessageItem["message"]> = {}): MessageItem {
  return { id: nextID++, modelId: "", createdAt: "", message: { role, ...extra } };
}
const user = () => msg("user", { content: "hi" });
const text = () => msg("assistant", { content: "done" });
const calls = (...names: string[]) =>
  msg("assistant", {
    tool_calls: names.map((name, i) => ({ id: `c${nextID}-${i}`, name, arguments: "{}" })),
  });
const tool = () => msg("tool", { tool_call_id: "x", content: "out" });

const kinds = (entries: ReturnType<typeof groupTranscript>) => entries.map((e) => e.kind);

describe("isToolResultMessage", () => {
  it("tool 角色或带 tool_call_id 的 user 视为工具结果", () => {
    expect(isToolResultMessage(tool())).toBe(true);
    expect(isToolResultMessage(msg("user", { tool_call_id: "a" }))).toBe(true);
    expect(isToolResultMessage(user())).toBe(false);
  });
});

describe("groupTranscript", () => {
  it("A 无工具调用时逐条平铺，index 为原下标", () => {
    const out = groupTranscript([user(), text()]);
    expect(out).toEqual([
      expect.objectContaining({ kind: "message", index: 0 }),
      expect.objectContaining({ kind: "message", index: 1 }),
    ]);
  });

  it("B 工具结果夹在调用之间不打断步骤段", () => {
    const a1 = calls("Read");
    const a2 = calls("Grep", "Read");
    const out = groupTranscript([user(), a1, tool(), a2, tool(), tool(), text()]);
    expect(kinds(out)).toEqual(["message", "steps", "message"]);
    const steps = out[1];
    if (steps.kind !== "steps") throw new Error("expected steps");
    expect(steps.callCount).toBe(3);
    expect(steps.toolNames).toEqual(["Read", "Grep"]);
    expect(steps.messages.map((m) => m.id)).toEqual([a1.id, a2.id]);
    expect(steps.id).toBe(a1.id);
  });

  it("C 段内只有一次调用时不折叠", () => {
    expect(kinds(groupTranscript([user(), calls("Read"), tool(), text()]))).toEqual([
      "message",
      "message",
      "message",
    ]);
  });

  it("D system 消息打断步骤段，工具结果不出现在输出中", () => {
    const out = groupTranscript([calls("X"), tool(), msg("system"), calls("Y"), tool()]);
    expect(kinds(out)).toEqual(["message", "message", "message"]);
    expect(out.every((e) => e.kind !== "message" || !isToolResultMessage(e.message))).toBe(true);
  });

  it("E 步骤段位于列表末尾（分页截断）仍成组", () => {
    const out = groupTranscript([user(), calls("A"), tool(), calls("B")]);
    expect(kinds(out)).toEqual(["message", "steps"]);
    expect(out[1].kind === "steps" && out[1].callCount).toBe(2);
  });
});
