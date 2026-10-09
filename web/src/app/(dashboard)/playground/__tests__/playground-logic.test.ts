import { describe, expect, it } from "vitest";
import {
  buildChatBody,
  buildCurl,
  estimateCost,
  fetchAllPages,
  ownedKeys,
  parseSSELine,
  selectableModels,
  type ChatUsage,
} from "../playground-logic";
import type { PricingDTO } from "@/lib/types";

const usage: ChatUsage = { promptTokens: 812, completionTokens: 408, cachedTokens: 512 };

describe("estimateCost", () => {
  it("按无条件默认规则估算：(prompt-cached)×input + cached×cacheRead + completion×output，单位每 1M", () => {
    const pricing: PricingDTO = {
      currency: "USD",
      rules: [
        {
          time_windows: [{ start: "09:00", end: "18:00" }],
          context_min: 128000,
          input_price: 99,
          output_price: 99,
          cache_creation_price: 99,
          cache_read_price: 99,
        },
        {
          input_price: 2.5,
          output_price: 10,
          cache_creation_price: 3.75,
          cache_read_price: 0.5,
        },
      ],
    };
    // (812-512)*2.5 + 512*0.5 + 408*10 = 750 + 256 + 4080 = 5086 / 1e6
    expect(estimateCost(usage, pricing)).toBeCloseTo(0.005086, 8);
  });

  it("无条件默认规则缺失时返回 null（不按其他规则猜）", () => {
    const pricing: PricingDTO = {
      currency: "USD",
      rules: [
        {
          context_min: 0,
          context_max: 128000,
          input_price: 1,
          output_price: 2,
          cache_creation_price: 1,
          cache_read_price: 1,
        },
      ],
    };
    expect(estimateCost(usage, pricing)).toBeNull();
  });

  it("未计价（无 currency）或无 pricing 返回 null", () => {
    expect(estimateCost(usage, undefined)).toBeNull();
    expect(
      estimateCost(usage, {
        rules: [{ input_price: 1, output_price: 1, cache_creation_price: 1, cache_read_price: 1 }],
      }),
    ).toBeNull();
  });

  it("cached 大于 prompt 时负输入按 0 计", () => {
    const pricing: PricingDTO = {
      currency: "USD",
      rules: [{ input_price: 1, output_price: 1, cache_creation_price: 1, cache_read_price: 1 }],
    };
    expect(
      estimateCost({ promptTokens: 10, completionTokens: 0, cachedTokens: 50 }, pricing),
    ).toBeCloseTo(0.00005, 8);
  });
});

describe("buildCurl", () => {
  it("生成指向 OpenAI 兼容端点的 curl，Key 脱敏占位", () => {
    const curl = buildCurl(
      { model: "gpt-4o", messages: [{ role: "user", content: "hi" }] },
      "https://api.example.com",
    );
    expect(curl).toContain("curl https://api.example.com/api/openai/v1/chat/completions");
    expect(curl).toContain("Authorization: Bearer <API_KEY>");
    expect(curl).toContain("-d '");
    expect(curl).toContain('"model": "gpt-4o"');
  });
});

describe("buildChatBody", () => {
  it("剔除空消息行并组装基础请求体", () => {
    const body = buildChatBody(
      [
        { role: "user", content: "hi" },
        { role: "user", content: "   " },
        { role: "assistant", content: "hello" },
      ],
      { model: "gpt-4", stream: false },
    );
    expect(body).toEqual({
      model: "gpt-4",
      messages: [
        { role: "user", content: "hi" },
        { role: "assistant", content: "hello" },
      ],
      stream: false,
    });
  });

  it("可选参数仅在有效时携带", () => {
    const withParams = buildChatBody([{ role: "user", content: "hi" }], {
      model: "gpt-4",
      stream: true,
      temperature: 0.7,
      maxTokens: 100,
    });
    expect(withParams.temperature).toBe(0.7);
    expect(withParams.max_tokens).toBe(100);

    const without = buildChatBody([{ role: "user", content: "hi" }], {
      model: "gpt-4",
      stream: false,
      temperature: undefined,
      maxTokens: 0,
    });
    expect("temperature" in without).toBe(false);
    expect("max_tokens" in without).toBe(false);
  });
});

describe("selectableModels", () => {
  it("仅保留 enabled 项并按 alias 去重", () => {
    const got = selectableModels([
      { alias: "a", enabled: true },
      { alias: "b", enabled: false },
      { alias: "a", enabled: true },
      { alias: "c", enabled: true },
    ]);
    expect(got.map((m) => m.alias)).toEqual(["a", "c"]);
  });
});

describe("fetchAllPages", () => {
  it("逐页拉取直到不足一页，不截断", async () => {
    const data = Array.from({ length: 5 }, (_, i) => i);
    const pages: number[] = [];
    const got = await fetchAllPages(async (page, size) => {
      pages.push(page);
      return { items: data.slice((page - 1) * size, page * size), total: data.length };
    }, 2);
    expect(got).toEqual(data);
    expect(pages).toEqual([1, 2, 3]);
  });

  it("恰好整页时按 total 停止，不多请求空页", async () => {
    const pages: number[] = [];
    const got = await fetchAllPages(async (page) => {
      pages.push(page);
      return { items: [page, page], total: 4 };
    }, 2);
    expect(got).toEqual([1, 1, 2, 2]);
    expect(pages).toEqual([1, 2]);
  });
});

describe("ownedKeys", () => {
  it("只保留当前用户名下的 Key（管理员列表含他人 Key）", () => {
    const got = ownedKeys(
      [
        { id: 1, name: "mine", user: { id: 7 } },
        { id: 2, name: "other", user: { id: 8 } },
        { id: 3, name: "no-user" },
      ],
      7,
    );
    expect(got.map((k) => k.id)).toEqual([1, 3]);
  });
});

describe("parseSSELine", () => {
  it("提取内容增量", () => {
    const line = 'data: {"choices":[{"delta":{"content":"你好"}}]}';
    expect(parseSSELine(line)).toEqual({ type: "delta", text: "你好" });
  });

  it("识别 usage 尾帧（choices 空 + usage）", () => {
    const line =
      'data: {"choices":[],"usage":{"prompt_tokens":812,"completion_tokens":408,"prompt_tokens_details":{"cached_tokens":512}}}';
    expect(parseSSELine(line)).toEqual({
      type: "usage",
      usage: { promptTokens: 812, completionTokens: 408, cachedTokens: 512 },
    });
  });

  it("usage 缺 cached_tokens 时归零", () => {
    const line = 'data: {"choices":[],"usage":{"prompt_tokens":10,"completion_tokens":5}}';
    expect(parseSSELine(line)).toEqual({
      type: "usage",
      usage: { promptTokens: 10, completionTokens: 5, cachedTokens: 0 },
    });
  });

  it("[DONE] 与空 data 行分别为 done / skip", () => {
    expect(parseSSELine("data: [DONE]")).toEqual({ type: "done" });
    expect(parseSSELine("data:")).toEqual({ type: "skip" });
  });

  it("role 帧、空行、非 data 行、畸形 JSON 均为 skip", () => {
    expect(parseSSELine('data: {"choices":[{"delta":{"role":"assistant"}}]}')).toEqual({
      type: "skip",
    });
    expect(parseSSELine("")).toEqual({ type: "skip" });
    expect(parseSSELine("event: message")).toEqual({ type: "skip" });
    expect(parseSSELine("data: {not-json")).toEqual({ type: "skip" });
  });

  it("delta.content 为空串时为 skip", () => {
    expect(parseSSELine('data: {"choices":[{"delta":{"content":""}}]}')).toEqual({
      type: "skip",
    });
  });
});
