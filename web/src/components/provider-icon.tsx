"use client";

import {
  OpenAI,
  DeepSeek,
  Google,
  Anthropic,
  Claude,
  ClaudeCode,
  Codex,
  OpenCode,
  Pi,
  Azure,
  Aws,
  Bedrock,
  Zhipu,
  ZAI,
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
import { findProviderKey, type ProviderKey } from "@/lib/provider-key";
import { cn } from "@/lib/utils";

type IconComponent = React.ComponentType<{
  size?: number | string;
  className?: string;
  style?: React.CSSProperties;
  "aria-hidden"?: boolean;
}>;

const providerMap: Record<ProviderKey, IconComponent> = {
  openai: OpenAI,
  anthropic: Anthropic,
  claudecode: ClaudeCode,
  claude: Claude,
  codex: Codex,
  opencode: OpenCode,
  pi: Pi,
  deepseek: DeepSeek,
  google: Google,
  gemini: Google,
  azure: Azure,
  aws: Aws,
  bedrock: Bedrock,
  zhipu: Zhipu,
  chatglm: ZAI,
  zai: ZAI,
  glm: ZAI,
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

export function ProviderIcon({
  protocol,
  size = 14,
  className,
  decorative,
}: {
  protocol: string;
  size?: number;
  className?: string;
  /** 旁边已有可见文字时传入：对读屏器隐藏图标内置的 <title>，避免重复朗读。 */
  decorative?: boolean;
}) {
  const providerKey = findProviderKey(protocol);
  const Icon = providerKey ? providerMap[providerKey] : HuggingFace;
  return (
    <Icon size={size} className={cn("shrink-0", className)} aria-hidden={decorative || undefined} />
  );
}
