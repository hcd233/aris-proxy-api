/**
 * 厂商 / 客户端图标的 key 匹配，独立于图标库以便单测。
 * 顺序有意义：`find` 取第一个命中项，较长的 key（如 claudecode）必须排在其前缀（claude）之前。
 */
export const PROVIDER_KEYS = [
  "openai",
  "anthropic",
  "claudecode",
  "claude",
  "codex",
  "opencode",
  "pi",
  "deepseek",
  "google",
  "gemini",
  "azure",
  "aws",
  "bedrock",
  "zhipu",
  "chatglm",
  "glm",
  "minimax",
  "moonshot",
  "kimi",
  "meta",
  "mistral",
  "cohere",
  "perplexity",
  "grok",
  "xai",
  "qwen",
  "yi",
  "baidu",
  "tencent",
  "bytedance",
  "doubao",
  "huggingface",
  "ollama",
  "github",
  "openrouter",
  "siliconcloud",
  "silicon",
  "hunyuan",
  "hy",
  "volcengine",
  "stepfun",
  "infinigence",
  "zeroone",
  "spark",
  "wenxin",
  "baichuan",
  "internlm",
  "nvidia",
  "xiaomimimo",
  "mimo",
  "gpt",
  "o1",
  "o3",
  "dall",
  "tts",
  "whisper",
  "gemma",
  "palm",
  "command",
  "zai",
] as const;

export type ProviderKey = (typeof PROVIDER_KEYS)[number];

export function findProviderKey(protocol: string): ProviderKey | undefined {
  const normalized = protocol.toLowerCase();
  return PROVIDER_KEYS.find((key) => {
    if (normalized === key) return true;
    if (normalized.startsWith(key + "-") || normalized.startsWith(key + "_")) return true;
    // 兼容无分隔符的短前缀（如 hy3）：前缀后紧跟非小写字母（数字/结尾/分隔符）
    if (normalized.startsWith(key)) {
      const next = normalized[key.length];
      if (next === undefined || !/[a-z]/.test(next)) return true;
    }
    return false;
  });
}

/**
 * 前缀 key → 厂商组。findProviderKey 返回的是「匹配前缀」（gpt / o3 / openai 是三个 key），
 * 模型选择器按厂商分组时需先归并；未列出的 key 自成一组。
 */
const PROVIDER_GROUP_OF: Partial<Record<ProviderKey, string>> = {
  gpt: "openai",
  o1: "openai",
  o3: "openai",
  dall: "openai",
  tts: "openai",
  whisper: "openai",
  codex: "openai",
  claude: "anthropic",
  claudecode: "anthropic",
  gemini: "google",
  gemma: "google",
  palm: "google",
  chatglm: "zhipu",
  glm: "zhipu",
  zai: "zhipu",
  kimi: "moonshot",
  hunyuan: "tencent",
  hy: "tencent",
  doubao: "bytedance",
  volcengine: "bytedance",
  xiaomimimo: "xiaomi",
  mimo: "xiaomi",
  silicon: "siliconcloud",
  command: "cohere",
};

/** 「其他」组键：别名匹配不到任何厂商前缀 */
export const OTHER_PROVIDER_GROUP = "other";

export const PROVIDER_GROUP_LABELS: Record<string, string> = {
  openai: "OpenAI",
  anthropic: "Anthropic",
  google: "Google",
  zhipu: "Zhipu",
  moonshot: "Moonshot",
  tencent: "Tencent",
  bytedance: "ByteDance",
  xiaomi: "Xiaomi",
  siliconcloud: "SiliconCloud",
};

export function providerGroupOf(alias: string): string {
  const key = findProviderKey(alias);
  if (!key) return OTHER_PROVIDER_GROUP;
  return PROVIDER_GROUP_OF[key] ?? key;
}

function groupLabel(key: string): string {
  return PROVIDER_GROUP_LABELS[key] ?? key.charAt(0).toUpperCase() + key.slice(1);
}

/** 模型按厂商分组：组按模型数降序（同数按 key 字母序），other 恒在最后；组内按别名字母序 */
export function groupModels<T extends { alias: string }>(
  models: T[],
): { key: string; label: string; items: T[] }[] {
  const byKey = new Map<string, T[]>();
  for (const m of models) {
    const key = providerGroupOf(m.alias);
    byKey.set(key, [...(byKey.get(key) ?? []), m]);
  }
  return [...byKey.entries()]
    .map(([key, items]) => ({
      key,
      label: groupLabel(key),
      items: [...items].sort((a, b) => a.alias.localeCompare(b.alias)),
    }))
    .sort((a, b) => {
      if (a.key === OTHER_PROVIDER_GROUP) return 1;
      if (b.key === OTHER_PROVIDER_GROUP) return -1;
      return b.items.length - a.items.length || a.key.localeCompare(b.key);
    });
}
