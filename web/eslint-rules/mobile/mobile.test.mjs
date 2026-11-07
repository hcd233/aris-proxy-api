import { readFileSync, readdirSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";
import { RuleTester } from "eslint";
import plugin from "./index.mjs";

RuleTester.describe = describe;
RuleTester.it = it;

const ruleTester = new RuleTester({
  languageOptions: {
    ecmaVersion: "latest",
    sourceType: "module",
    parserOptions: { ecmaFeatures: { jsx: true } },
  },
});
const { rules } = plugin;

ruleTester.run("mobile/no-viewport-vh", rules["no-viewport-vh"], {
  valid: [
    { code: "<div className='h-dvh min-h-svh' />" },
    { code: "<div className='max-h-[85dvh]' />" },
    { code: "<div className={cn('h-[calc(100dvh-4rem)]')} />" },
    { code: "const s = 'screen reader';" },
  ],
  invalid: [
    { code: "<div className='h-screen overflow-hidden' />", errors: 1 },
    { code: "<div className='md:min-h-screen' />", errors: 1 },
    { code: "<div className='max-h-[85vh]' />", errors: 1 },
    { code: "<div className={`h-[calc(100vh-4rem)] ${x}`} />", errors: 1 },
    { code: "const style = { height: '100vh' };", errors: 1 },
  ],
});

ruleTester.run("mobile/hover-reveal-needs-touch", rules["hover-reveal-needs-touch"], {
  valid: [
    // 触屏常显兜底
    {
      code: "<div className='opacity-0 group-hover/msg:opacity-100 pointer-coarse:opacity-100' />",
    },
    { code: "<div className='opacity-0 hover:opacity-100 any-pointer-coarse:opacity-100' />" },
    // 隐藏只作用于精确指针
    { code: "<div className='pointer-fine:opacity-0 group-hover:opacity-100' />" },
    // 半透明而非隐藏
    { code: "<div className='opacity-60 hover:opacity-100' />" },
    // 状态驱动的隐藏（非悬停显示）
    { code: "<div className='opacity-0 data-open:opacity-100' />" },
    { code: "<div className='group-data-[collapsible=icon]:opacity-0 group-hover:opacity-100' />" },
    // 响应式显隐
    { code: "<div className='hidden md:flex' />" },
  ],
  invalid: [
    {
      code: "<div className='opacity-0 transition-opacity group-hover/msg:opacity-100' />",
      errors: 1,
    },
    // focus-visible 只照顾键盘
    {
      code: "<Button className='opacity-0 group-hover:opacity-100 focus-visible:opacity-100' />",
      errors: 1,
    },
    // 桌面才隐藏，平板（≥md 触屏）仍然看不到
    { code: "<div className={cn('md:opacity-0', 'group-hover/item:opacity-100')} />", errors: 1 },
    { code: "<div className='hidden group-hover:flex' />", errors: 1 },
    { code: "<div className='invisible peer-hover:visible' />", errors: 1 },
  ],
});

ruleTester.run("mobile/input-font-size", rules["input-font-size"], {
  valid: [
    { code: "<input className='text-sm pointer-coarse:text-base' />" },
    { code: "<textarea className='text-base md:text-sm pointer-coarse:text-base' />" },
    { code: "<input className='text-base' />" },
    { code: "<input type='checkbox' className='text-xs' />" },
    { code: "<InputPrimitive className='text-sm any-pointer-coarse:text-base' />" },
    { code: "<ComboboxPrimitive.Input className='text-sm pointer-coarse:text-base' />" },
    // 封装组件：基础字号由封装内部负责，调用方的 text-sm 不会覆盖 pointer-coarse:text-base
    { code: "<Input className='text-sm' />" },
    { code: "<InputGroupInput className='h-8 text-xs' />" },
    // 无字号类：继承
    { code: "<input className='w-full' />" },
    // 非输入元素
    { code: "<span className='text-xs' />" },
    // placeholder 等伪元素字号不影响缩放
    { code: "<input className='text-base placeholder:text-sm' />" },
  ],
  invalid: [
    { code: "<input className='text-sm' />", errors: 1 },
    { code: "<input type='search' className='text-[14px]' />", errors: 1 },
    { code: "<textarea className='text-base md:text-sm' />", errors: 1 },
    { code: "<select className='text-xs' />", errors: 1 },
    { code: "<InputPrimitive className={cn('h-9 text-sm', className)} />", errors: 1 },
    { code: "<ComboboxPrimitive.Input className='flex-1 text-sm' />", errors: 1 },
    { code: "<Input className='pointer-coarse:text-sm' />", errors: 1 },
  ],
});

ruleTester.run("mobile/no-fixed-overflow-width", rules["no-fixed-overflow-width"], {
  valid: [
    { code: "<div className='w-80' />" },
    { code: "<div className='w-[20rem]' />" },
    { code: "<div className='sm:w-[23.25rem]' />" },
    { code: "<div className='w-[min(23.25rem,85vw)]' />" },
    { code: "<div className='w-[400px] max-w-full' />" },
    { code: "<div className='w-[28rem] max-w-[calc(100vw-2rem)]' />" },
    { code: "<div className='max-w-[768px] w-full' />" },
    { code: "<div className='lg:grid-cols-[1fr_22rem]' />" },
    { code: "<div className='grid-cols-[repeat(auto-fill,minmax(8.5rem,1fr))]' />" },
    { code: "<div className='grid-cols-[5.5rem_1fr]' />" },
  ],
  invalid: [
    { code: "<div className='w-[23.25rem] shrink-0' />", errors: 1 },
    { code: "<div className='w-96' />", errors: 1 },
    { code: "<div className='w-md' />", errors: 1 },
    { code: "<PopoverContent className='w-[400px]' />", errors: 1 },
    // min-w 压不回来，max-w-full 不算兜底
    { code: "<div className='min-w-[480px] max-w-full' />", errors: 1 },
    { code: "<div className='max-md:w-[24rem]' />", errors: 1 },
    { code: "<div className='grid-cols-[1fr_22rem]' />", errors: 1 },
    { code: "<div className='grid-cols-[minmax(12rem,1fr)_12rem]' />", errors: 1 },
  ],
});

ruleTester.run("mobile/min-touch-target", rules["min-touch-target"], {
  valid: [
    { code: "<button className='h-9 px-3' />" },
    { code: "<button className='h-6' />" },
    { code: "<button className='h-5 pointer-coarse:h-6' />" },
    { code: "<button className='relative size-5 after:absolute after:-inset-2' />" },
    { code: "<button className='h-5 min-h-6' />" },
    { code: "<a className='underline' />" },
    // 非可点击元素
    { code: "<span className='size-4' />" },
    { code: "<Button size='icon-xs' />" },
  ],
  invalid: [
    { code: "<button className='inline-flex h-5 items-center' />", errors: 1 },
    { code: "<a className='size-4' />", errors: 1 },
    { code: "<Button className='size-5' />", errors: 1 },
    { code: "<button className='h-[20px]' />", errors: 1 },
    // 桌面断点上的放大不算
    { code: "<button className='h-5 md:h-9' />", errors: 1 },
  ],
});

// MOB-01 的 CSS 侧：ESLint 不扫样式表，这里直接扫 src 下所有 .css
describe("mobile/no-viewport-vh (css)", () => {
  const srcDir = join(import.meta.dirname, "../../src");
  const cssFiles = readdirSync(srcDir, { recursive: true })
    .filter((f) => f.endsWith(".css"))
    .map((f) => join(srcDir, f));

  it("扫到了样式表", () => {
    expect(cssFiles.length).toBeGreaterThan(0);
  });

  it.each(cssFiles)("%s 不使用 vh 单位", (file) => {
    const offenders = readFileSync(file, "utf8")
      .split("\n")
      .map((line, i) => [i + 1, line])
      .filter(([, line]) => /\dvh\b/.test(line));
    expect(offenders, "改用 dvh / svh（MOB-01）").toEqual([]);
  });
});
