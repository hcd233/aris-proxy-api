import { describe, expect, it } from "vitest";
import type { ToolItem } from "@/lib/types";
import { filterToolsByName } from "./tool-filter";

const tool = (id: number, name: string): ToolItem => ({
  id,
  tool: { name, description: "", parameters: {} },
  createdAt: "",
});

const tools = [tool(1, "ask_user_question"), tool(2, "Bash"), tool(3, "read_file")];

describe("filterToolsByName", () => {
  it("空白查询返回原列表", () => {
    expect(filterToolsByName(tools, "   ")).toBe(tools);
  });

  it("按名字子串匹配且不区分大小写", () => {
    expect(filterToolsByName(tools, "bash").map((t) => t.id)).toEqual([2]);
    expect(filterToolsByName(tools, " _USER ").map((t) => t.id)).toEqual([1]);
  });

  it("只匹配名字，不匹配描述", () => {
    const described = { ...tool(4, "x"), tool: { name: "x", description: "bash", parameters: {} } };
    expect(filterToolsByName([described], "bash")).toEqual([]);
  });
});
