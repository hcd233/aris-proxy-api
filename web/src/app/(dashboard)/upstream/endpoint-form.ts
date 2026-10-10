// 端点表单的类型与默认值（纯模块）。
//
// 独立于此处的 shared.tsx：后者依赖 `@/components/*`，vitest 无法解析 `@/` 别名，
// 故表单默认值放在无别名依赖的纯模块里，才能被单测直接断言。
// shared.tsx 继续 re-export 本模块，既有 `./shared` 引用无需改动。

export interface EndpointForm {
  name: string;
  openaiBaseURL: string;
  anthropicBaseURL: string;
  apiKey: string;
  supportOpenAIChatCompletion: boolean;
  supportOpenAIResponse: boolean;
  supportAnthropicMessage: boolean;
  supportOpenAIDecision: boolean;
  ownerUserID?: number;
}

export const emptyEndpointForm: EndpointForm = {
  name: "",
  openaiBaseURL: "",
  anthropicBaseURL: "",
  apiKey: "",
  supportOpenAIChatCompletion: true,
  supportOpenAIResponse: false,
  supportAnthropicMessage: false,
  supportOpenAIDecision: false,
};
