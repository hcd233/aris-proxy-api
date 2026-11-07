import { describe, it } from "vitest";
import { RuleTester } from "eslint";
import rule from "./no-hardcoded-yaxis-width.mjs";

RuleTester.describe = describe;
RuleTester.it = it;

const ruleTester = new RuleTester({
  languageOptions: {
    ecmaVersion: "latest",
    sourceType: "module",
    parserOptions: { ecmaFeatures: { jsx: true } },
  },
});

ruleTester.run("no-hardcoded-yaxis-width", rule, {
  valid: [
    // 规范写法：交给 recharts 按实际刻度测量
    { code: '<YAxis width="auto" tickFormatter={f} />' },
    // 不声明宽度时用 recharts 默认值，不是写死
    { code: "<YAxis tickFormatter={f} />" },
    // 动态值不可静态判定，放行（宁漏报不误报）
    { code: "<YAxis width={axisWidth} />" },
    // 只约束 Y 轴；X 轴没有 width 语义
    { code: "<XAxis width={40} />" },
  ],
  invalid: [
    { code: "<YAxis width={40} />", errors: [{ messageId: "hardcodedWidth" }] },
    {
      code: "<YAxis tickLine={false} axisLine={false} width={52} tickFormatter={f} />",
      errors: [{ messageId: "hardcodedWidth" }],
    },
    // 数字字符串同样写死（recharts 会转成数字）
    { code: '<YAxis width="44" />', errors: [{ messageId: "hardcodedWidth" }] },
  ],
});
