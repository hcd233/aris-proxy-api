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
