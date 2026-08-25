import { defineConfig, globalIgnores } from "eslint/config";
import nextVitals from "eslint-config-next/core-web-vitals";
import nextTs from "eslint-config-next/typescript";
import prettier from "eslint-config-prettier/flat";
import noNativeTitle from "./eslint-rules/no-native-title.mjs";
import truncateRequiresTooltip from "./eslint-rules/truncate-requires-tooltip.mjs";

const eslintConfig = defineConfig([
  ...nextVitals,
  ...nextTs,
  // 关闭与 Prettier 冲突的格式类规则，代码格式统一交给 prettier 管理
  prettier,
  // 自定义规则：悬停提示一律使用 Tooltip 组件，禁止原生 title 属性（浏览器默认 tooltip）
  {
    plugins: {
      "no-native-title": { rules: { "no-native-title": noNativeTitle } },
    },
    rules: {
      "no-native-title/no-native-title": "error",
    },
  },
  // 自定义规则：截断文案（truncate / line-clamp-1）必须处于 TooltipTrigger 渲染子树内，
  // 保证用户可悬停查看完整内容
  {
    plugins: {
      "truncate-requires-tooltip": {
        rules: { "truncate-requires-tooltip": truncateRequiresTooltip },
      },
    },
    rules: {
      "truncate-requires-tooltip/truncate-requires-tooltip": "error",
    },
  },
  // Override default ignores of eslint-config-next.
  globalIgnores([
    // Default ignores of eslint-config-next:
    ".next/**",
    "out/**",
    "build/**",
    "next-env.d.ts",
    // 构建产物（make web-build 输出），避免 lint 扫描压缩 JS 产生噪音
    "dist/**",
  ]),
]);

export default eslintConfig;
