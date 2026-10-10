import { describe, expect, it } from "vitest";
import { emptyEndpointForm } from "../endpoint-form";

describe("emptyEndpointForm", () => {
  it("默认关闭 OpenAI Decision 能力", () => {
    expect(emptyEndpointForm.supportOpenAIDecision).toBe(false);
  });

  it("保留既有三项能力默认值", () => {
    expect(emptyEndpointForm.supportOpenAIChatCompletion).toBe(true);
    expect(emptyEndpointForm.supportOpenAIResponse).toBe(false);
    expect(emptyEndpointForm.supportAnthropicMessage).toBe(false);
  });
});
