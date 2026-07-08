"use client";

import {
  OpenAI,
  DeepSeek,
  Google,
  Anthropic,
  Claude,
  Azure,
  Aws,
  Bedrock,
  Zhipu,
  ChatGLM,
  Minimax,
  Moonshot,
  Meta,
  Mistral,
  Cohere,
  Perplexity,
  Grok,
  XAI,
  Qwen,
  Yi,
  Baidu,
  Tencent,
  ByteDance,
  Doubao,
  HuggingFace,
  Ollama,
  Github,
  OpenRouter,
  SiliconCloud,
  Hunyuan,
  Volcengine,
  Stepfun,
  Infinigence,
  ZeroOne,
  Spark,
  Wenxin,
  Baichuan,
  InternLM,
  XiaomiMiMo,
  Nvidia,
} from "@lobehub/icons";
import { cn } from "@/lib/utils";

type IconComponent = React.ComponentType<{
  size?: number | string;
  className?: string;
  style?: React.CSSProperties;
}>;

const providerMap: Record<string, IconComponent> = {
  openai: OpenAI,
  anthropic: Anthropic,
  claude: Claude,
  deepseek: DeepSeek,
  google: Google,
  gemini: Google,
  azure: Azure,
  aws: Aws,
  bedrock: Bedrock,
  zhipu: Zhipu,
  chatglm: ChatGLM,
  glm: Zhipu,
  minimax: Minimax,
  moonshot: Moonshot,
  kimi: Moonshot,
  meta: Meta,
  mistral: Mistral,
  cohere: Cohere,
  perplexity: Perplexity,
  grok: Grok,
  xai: XAI,
  qwen: Qwen,
  yi: Yi,
  baidu: Baidu,
  tencent: Tencent,
  bytedance: ByteDance,
  doubao: Doubao,
  huggingface: HuggingFace,
  ollama: Ollama,
  github: Github,
  openrouter: OpenRouter,
  siliconcloud: SiliconCloud,
  silicon: SiliconCloud,
  hunyuan: Hunyuan,
  hy: Hunyuan,
  volcengine: Volcengine,
  stepfun: Stepfun,
  infinigence: Infinigence,
  zeroone: ZeroOne,
  spark: Spark,
  wenxin: Wenxin,
  baichuan: Baichuan,
  internlm: InternLM,
  nvidia: Nvidia,
  xiaomimimo: XiaomiMiMo,
  mimo: XiaomiMiMo,
  gpt: OpenAI,
  o1: OpenAI,
  o3: OpenAI,
  dall: OpenAI,
  tts: OpenAI,
  whisper: OpenAI,
  gemma: Google,
  palm: Google,
  command: Cohere,
};

function findProviderKey(protocol: string): string | undefined {
  const normalized = protocol.toLowerCase();
  return Object.keys(providerMap).find((key) => {
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

export function ProviderIcon({
  protocol,
  size = 14,
  className,
}: {
  protocol: string;
  size?: number;
  className?: string;
}) {
  const providerKey = findProviderKey(protocol);
  if (!providerKey) return <HuggingFace size={size} className={cn("shrink-0", className)} />;

  const Icon = providerMap[providerKey];
  return <Icon size={size} className={cn("shrink-0", className)} />;
}
