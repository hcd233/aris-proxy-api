import { describe, expect, it } from "vitest";
import { formatImageMeta, parseBlobRef, pickImageSrc } from "./blob-image";
import { buildToolResultsByID } from "./content-extract";
import type { MessageItem } from "../../lib/types";

const SHA = "ab".repeat(32);

describe("parseBlobRef", () => {
  it("只接受 aris-blob:sha256: 加 64 位小写 hex", () => {
    expect(parseBlobRef(`aris-blob:sha256:${SHA}`)).toBe(SHA);
    expect(parseBlobRef(`aris-blob:sha256:${SHA.toUpperCase()}`)).toBeNull();
    expect(parseBlobRef(`aris-blob:sha256:${SHA.slice(1)}`)).toBeNull();
    expect(parseBlobRef(SHA)).toBeNull();
    expect(parseBlobRef("data:image/png;base64,AAAA")).toBeNull();
    expect(parseBlobRef(undefined)).toBeNull();
  });
});

describe("formatImageMeta", () => {
  it("类型取子类型大写，大小按 B/KB/MB 换算", () => {
    expect(formatImageMeta("image/png", 1258291)).toBe("PNG · 1.2 MB");
    expect(formatImageMeta("image/jpeg", 512)).toBe("JPEG · 512 B");
    expect(formatImageMeta("image/webp", 2048)).toBe("WEBP · 2.0 KB");
    expect(formatImageMeta(undefined, undefined)).toBe("");
  });
});

describe("pickImageSrc", () => {
  it("url 必须是 http(s)，dataUrl 必须以 data:image/ 开头", () => {
    expect(pickImageSrc({ url: "https://s.example.com/a.png" })).toBe(
      "https://s.example.com/a.png",
    );
    expect(pickImageSrc({ url: "javascript:alert(1)" })).toBeNull();
    expect(pickImageSrc({ dataUrl: "data:image/png;base64,AAAA" })).toBe(
      "data:image/png;base64,AAAA",
    );
    expect(pickImageSrc({ dataUrl: "data:text/html;base64,AAAA" })).toBeNull();
    expect(pickImageSrc({})).toBeNull();
  });
});

describe("buildToolResultsByID", () => {
  it("tool_result 中的图片进入 parts，文本仍在 text", () => {
    const message = {
      id: 1,
      modelId: "",
      createdAt: "",
      message: {
        role: "user",
        tool_call_id: "call_1",
        content: [
          { type: "text", text: "截图" },
          { type: "image_url", image_url: `aris-blob:sha256:${SHA}` },
        ],
      },
    } as unknown as MessageItem;
    const result = buildToolResultsByID([message]).call_1;
    expect(result.text).toBe("截图");
    expect(result.parts).toHaveLength(1);
    expect(result.parts?.[0].image_url).toBe(`aris-blob:sha256:${SHA}`);
  });
});
