import { describe, it } from "vitest";
import { RuleTester } from "eslint";
import rule from "./truncate-requires-tooltip.mjs";

RuleTester.describe = describe;
RuleTester.it = it;

const ruleTester = new RuleTester({
  languageOptions: {
    ecmaVersion: "latest",
    sourceType: "module",
    parserOptions: { ecmaFeatures: { jsx: true } },
  },
});

ruleTester.run("truncate-requires-tooltip", rule, {
  valid: [
    // 无截断类
    { code: "<span className='text-sm'>{x}</span>" },
    // 主流模式：Trigger render 包裹截断元素
    { code: "<TooltipTrigger render={<span className='truncate'>{x}</span>} />" },
    // Trigger children 包裹
    { code: "<TooltipTrigger><span className='truncate'>{x}</span></TooltipTrigger>" },
    // render 内嵌套包装元素
    {
      code: "<TooltipTrigger render={<div><span className='truncate'>{x}</span></div>} />",
    },
    // TooltipContent 内豁免（嵌套 tooltip 不可行）
    { code: "<TooltipContent><span className='truncate'>{x}</span></TooltipContent>" },
    // 多行截断豁免
    { code: "<p className='line-clamp-2'>{x}</p>" },
    // cn() 内 line-clamp-1 + Trigger 包裹
    {
      code: "<TooltipTrigger render={<span className={cn('line-clamp-1', cond && 'font-bold')}>{x}</span>} />",
    },
    // 模板字符串无截断字面量
    { code: "<span className={`${dynamic} text-sm`}>{x}</span>" },
    // 纯变量类名放行（静态不可见）
    { code: "<span className={dynamic}>{x}</span>" },
  ],
  invalid: [
    // 裸 truncate
    {
      code: "<span className='text-sm truncate'>{x}</span>",
      errors: [{ messageId: "needsTooltip" }],
    },
    // 裸 line-clamp-1
    { code: "<p className='line-clamp-1'>{x}</p>", errors: [{ messageId: "needsTooltip" }] },
    // TooltipRoot 内但不在 Trigger 子树
    {
      code: "<TooltipRoot><div><span className='truncate'>{x}</span></div></TooltipRoot>",
      errors: [{ messageId: "needsTooltip" }],
    },
    // TooltipProvider 不提供 tooltip
    {
      code: "<TooltipProvider><span className='truncate'>{x}</span></TooltipProvider>",
      errors: [{ messageId: "needsTooltip" }],
    },
    // cn() 字面量命中
    {
      code: "<span className={cn('truncate', cond && 'font-bold')}>{x}</span>",
      errors: [{ messageId: "needsTooltip" }],
    },
    // 模板字符串 quasis 命中
    {
      code: "<span className={`prefix truncate`}>{x}</span>",
      errors: [{ messageId: "needsTooltip" }],
    },
  ],
});
