import { describe, expect, it } from "vitest";
import { buildModelListParams } from "../model-list-params";

/**
 * 视图切换的核心不变量：平铺专属维度不得出现在分组请求里。
 *
 * 分组接口（/upstream/list）只接受 query/username；若把平铺的 status/capability/
 * endpointID 串进去，后端会静默忽略（进而让人误以为筛选生效）。
 * 这里守住"参数构造是纯函数、由调用方决定传什么"这一边界。
 */
describe("view isolation", () => {
  it("grouped params never carry flat-only dimensions", () => {
    const grouped = buildModelListParams({
      page: 1,
      pageSize: 10,
      freeText: "gpt",
      params: { username: "alice" },
      sortField: "created_at",
      sort: "desc",
    });
    expect(Object.keys(grouped).sort()).toEqual([
      "page",
      "pageSize",
      "query",
      "sort",
      "sortField",
      "username",
    ]);
    expect(grouped).not.toHaveProperty("status");
    expect(grouped).not.toHaveProperty("capability");
    expect(grouped).not.toHaveProperty("endpointID");
  });

  it("flat params carry only the dimensions that are actually set", () => {
    const flat = buildModelListParams({
      page: 1,
      pageSize: 10,
      freeText: "",
      params: { status: "disabled" },
      sortField: "alias",
      sort: "asc",
    });
    expect(flat.status).toBe("disabled");
    expect(flat.capability).toBeUndefined();
    expect(flat.endpointID).toBeUndefined();
  });
});
