import { describe, expect, it, vi } from "vitest";
import {
  buildChatBody,
  buildCurl,
  deriveTitle,
  estimateCost,
  fetchAllPages,
  FIXED_MAX_TOKENS,
  FIXED_TEMPERATURE,
  loadParams,
  loadSessions,
  MAX_SESSIONS,
  newMessage,
  newSession,
  ownedKeys,
  PARAMS_STORAGE_KEY,
  parseSSELine,
  saveParams,
  saveSessions,
  selectableModels,
  SESSIONS_STORAGE_KEY,
  truncateAfter,
  upsertSession,
  type ChatUsage,
  type PlaygroundMessage,
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

function stubStorage() {
  const store = new Map<string, string>();
  vi.stubGlobal("localStorage", {
    getItem: (k: string) => store.get(k) ?? null,
    setItem: (k: string, v: string) => void store.set(k, v),
    removeItem: (k: string) => void store.delete(k),
  });
  return store;
}

function msg(role: PlaygroundMessage["role"], content: string, id = content): PlaygroundMessage {
  return { id, role, content };
}

describe("会话存储", () => {
  it("写读回环；损坏 JSON / 版本不符降级为空", () => {
    const store = stubStorage();
    const sessions = [newSession()];
    expect(saveSessions(sessions)).toBe(true);
    expect(loadSessions()).toEqual(sessions);

    store.set(SESSIONS_STORAGE_KEY, "{broken");
    expect(loadSessions()).toEqual([]);
    store.set(SESSIONS_STORAGE_KEY, JSON.stringify({ version: 99, sessions: [1] }));
    expect(loadSessions()).toEqual([]);
    vi.unstubAllGlobals();
  });

  it("upsertSession 更新置顶、插入新会话并淘汰最旧", () => {
    const base = Array.from({ length: MAX_SESSIONS }, (_, i) => ({
      ...newSession(),
      id: `s${i}`,
      updatedAt: 1000 + i,
    }));
    const inserted = upsertSession(base, { ...newSession(), id: "new" });
    expect(inserted).toHaveLength(MAX_SESSIONS);
    expect(inserted[0].id).toBe("new");
    expect(inserted.map((s) => s.id)).not.toContain("s0");

    const updated = upsertSession(inserted, { ...inserted[5], title: "改" });
    expect(updated[0].id).toBe(inserted[5].id);
    expect(updated[0].title).toBe("改");
  });

  it("deriveTitle 取首条非空 user 消息前 30 字", () => {
    expect(deriveTitle([msg("system", "sys"), msg("user", "你好".repeat(20))])).toHaveLength(30);
    expect(deriveTitle([msg("system", "only system")])).toBe("");
    expect(deriveTitle([])).toBe("");
  });

  it("truncateAfter 保留 [0..index]", () => {
    const msgs = [msg("user", "a"), msg("assistant", "b"), msg("user", "c")];
    expect(truncateAfter(msgs, 0)).toEqual([msgs[0]]);
    expect(truncateAfter(msgs, 2)).toEqual(msgs);
  });

  it("newMessage / newSession 生成 id 与时间戳", () => {
    const m = newMessage("user", "hi");
    expect(m.role).toBe("user");
    expect(m.id).not.toBe("");
    const s = newSession();
    expect(s.messages).toEqual([]);
    expect(s.title).toBe("");
    expect(s.createdAt).toBeGreaterThan(0);
  });
});

describe("参数存储", () => {
  it("写读回环；无数据或损坏返回 null", () => {
    const store = stubStorage();
    expect(loadParams()).toBeNull();
    const params = { model: "gpt-4o", apiKeyID: 7 };
    saveParams(params);
    expect(loadParams()).toEqual(params);
    store.set(PARAMS_STORAGE_KEY, "{broken");
    expect(loadParams()).toBeNull();
    vi.unstubAllGlobals();
  });

  it("丢弃历史版本遗留字段（已下线的采样参数）", () => {
    const store = stubStorage();
    store.set(
      PARAMS_STORAGE_KEY,
      JSON.stringify({
        model: "claude-sonnet-4-5",
        apiKeyID: 3,
        temperature: "0.2",
        maxTokens: "1024",
        stream: false,
      }),
    );
    expect(loadParams()).toEqual({ model: "claude-sonnet-4-5", apiKeyID: 3 });
    vi.unstubAllGlobals();
  });
});

describe("buildChatBody", () => {
  it("剔除空消息行，采样参数与流式开关取固定值", () => {
    const body = buildChatBody(
      [
        { role: "user", content: "hi" },
        { role: "user", content: "   " },
        { role: "assistant", content: "hello" },
      ],
      "gpt-4",
    );
    expect(body).toEqual({
      model: "gpt-4",
      messages: [
        { role: "user", content: "hi" },
        { role: "assistant", content: "hello" },
      ],
      stream: true,
      temperature: FIXED_TEMPERATURE,
      max_tokens: FIXED_MAX_TOKENS,
    });
  });

  it("固定采样参数为 0.7 / 64834", () => {
    expect(FIXED_TEMPERATURE).toBe(0.7);
    expect(FIXED_MAX_TOKENS).toBe(64834);
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

  it("提取思考增量（reasoning_content，Anthropic thinking 转译后）", () => {
    const line = 'data: {"choices":[{"delta":{"reasoning_content":"先想一下"}}]}';
    expect(parseSSELine(line)).toEqual({ type: "reasoning", text: "先想一下" });
    expect(parseSSELine('data: {"choices":[{"delta":{"reasoning_content":""}}]}')).toEqual({
      type: "skip",
    });
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
