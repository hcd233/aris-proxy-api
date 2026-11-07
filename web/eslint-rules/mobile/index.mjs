/**
 * 移动端适配规范（MOB-01 ~ MOB-05）的 ESLint 插件。
 *
 * 规范全文：docs/superpowers/specs/2026-11-07-web-mobile-lint-design.md；
 * 速查：docs/agents/web-frontend.md「移动端适配契约」。
 *
 * 共同约定：只分析 className / xxxClassName 里静态可见的字符串字面量，动态值放行
 * （宁漏报不误报）；cva() 变体表、模块级类名常量不在 JSX 属性上，规则看不到。
 * 确属例外时用 `// eslint-disable-next-line mobile/<rule> -- <理由>`，理由必填。
 */

import { jsxElementName, literalAttribute } from "../class-literals.mjs";
import {
  MIN_INPUT_FONT_PX,
  MIN_TOUCH_TARGET_PX,
  PHONE_CONTENT_MAX_PX,
  appliesOnPhone,
  elementTokens,
  fontSizePx,
  isCoarse,
  isFine,
  isHover,
  isResponsive,
  lengthToPx,
  parseToken,
  sizeValueToPx,
} from "./tokens.mjs";

const SPEC = "docs/superpowers/specs/2026-11-07-web-mobile-lint-design.md";
const meta = (id, description, message) => ({
  type: "problem",
  docs: { description, url: SPEC },
  schema: [],
  messages: { [id]: message },
});

/* ------------------------------------------------------------------ */
/* MOB-01 视口高度用动态单位                                            */
/* ------------------------------------------------------------------ */

const SCREEN_HEIGHT = new Set(["h-screen", "min-h-screen", "max-h-screen", "size-screen"]);
// `100vh`、`calc(100vh-4rem)`；`100dvh` / `100svh` 中 vh 前不是数字，不命中
const VH_UNIT = /\dvh\b/;

/**
 * 移动浏览器的 `100vh` 按「地址栏收起」时的最大视口计算，地址栏展开时底部会被裁掉
 * 一截（iOS Safari 约 80px）。整屏布局用 `dvh`（随地址栏变化）或 `svh`（取最小视口）。
 * 字符串字面量全量扫描：类名、内联 style 值、模板字符串都覆盖。
 */
const noViewportVh = {
  meta: meta(
    "useDynamicViewport",
    "Disallow 100vh-based heights; use dvh/svh so mobile browser chrome does not clip content.",
    "MOB-01：`{{found}}` 在移动端会被地址栏裁掉底部，改用动态视口单位（`h-dvh` / `min-h-dvh` / `max-h-[80dvh]`，需要稳定最小高度时用 `svh`）。",
  ),
  create(context) {
    function check(node, text) {
      const screen = text.split(/\s+/).find((raw) => SCREEN_HEIGHT.has(parseToken(raw).utility));
      const found =
        screen ?? (VH_UNIT.test(text) ? text.match(/[^\s"'`]*\dvh\b[^\s"'`]*/)[0] : null);
      if (found) context.report({ node, messageId: "useDynamicViewport", data: { found } });
    }
    return {
      Literal(node) {
        if (typeof node.value === "string") check(node, node.value);
      },
      TemplateElement(node) {
        check(node, node.value.raw);
      },
    };
  },
};

/* ------------------------------------------------------------------ */
/* MOB-02 悬停才出现的内容必须有触屏兜底                                */
/* ------------------------------------------------------------------ */

const HIDE = new Set(["opacity-0", "invisible", "hidden"]);
const REVEAL = new Set([
  "opacity-100",
  "visible",
  "block",
  "inline-block",
  "flex",
  "inline-flex",
  "grid",
  "inline",
]);

/**
 * Tailwind v4 的 `hover:` / `group-hover:` 包在 `@media (hover: hover)` 里，触屏设备
 * 永远不会命中——「默认隐藏 + 悬停显示」的操作按钮在手机/平板上等于不存在。
 *
 * 达标写法（二选一）：
 *  - 追加 `pointer-coarse:opacity-100`（触屏常显，与 code-block 一致）；
 *  - 隐藏只作用于精确指针：`pointer-fine:opacity-0`。
 * `focus-visible` / `group-focus-within` 只照顾键盘，不算触屏兜底。
 */
const hoverRevealNeedsTouch = {
  meta: meta(
    "needsTouchFallback",
    "Require hover-revealed content to stay visible on touch devices.",
    "MOB-02：`{{hide}}` + `{{reveal}}` 只能靠悬停显示，触屏设备永远看不到。追加 `pointer-coarse:{{show}}`，或把隐藏限定为 `pointer-fine:{{hide}}`。",
  ),
  create(context) {
    return {
      JSXOpeningElement(node) {
        const tokens = elementTokens(node);
        const hides = tokens.filter(
          (t) =>
            HIDE.has(t.utility) &&
            t.variants.every(
              (v) => isResponsive(v) || isFine(v) || v === "dark" || v.startsWith("["),
            ),
        );
        if (hides.length === 0) return;
        const reveal = tokens.find((t) => REVEAL.has(t.utility) && t.variants.some(isHover));
        if (!reveal) return;
        if (tokens.some((t) => REVEAL.has(t.utility) && t.variants.some(isCoarse))) return;
        if (hides.every((t) => t.variants.some(isFine))) return;
        context.report({
          node,
          messageId: "needsTouchFallback",
          data: { hide: hides[0].raw, reveal: reveal.raw, show: reveal.utility },
        });
      },
    };
  },
};

/* ------------------------------------------------------------------ */
/* MOB-03 输入框在触屏上字号 ≥ 16px                                     */
/* ------------------------------------------------------------------ */

const NON_TEXT_INPUT = new Set([
  "checkbox",
  "radio",
  "range",
  "color",
  "file",
  "hidden",
  "submit",
  "button",
  "reset",
  "image",
]);

function inputKind(node) {
  const name = jsxElementName(node);
  if (!name) return null;
  if (name === "input") return NON_TEXT_INPUT.has(literalAttribute(node, "type")) ? null : "leaf";
  if (name === "textarea" || name === "select") return "leaf";
  // Base UI 原语直接渲染原生 input：成员形式（ComboboxPrimitive.Input）或
  // 按项目约定重命名导入（`import { Input as InputPrimitive }`）
  if (/\.(Input|Textarea)$|^(Input|Textarea)Primitive$/.test(name)) return "leaf";
  // 业务封装（Input / InputGroupInput / ComboboxInput……）：基础字号由封装内部的原生元素负责
  if (/^([A-Z]\w*)?(Input|Textarea)$/.test(name)) return "wrapper";
  return null;
}

/**
 * iOS Safari 聚焦字号 < 16px 的 input / textarea / select 会自动放大整页且不回缩。
 * 原生元素（及直接渲染原生元素的原语）上：只要手机上生效的字号 < 16px，就必须同时给出
 * `pointer-coarse:text-base`（粗指针变体在产物 CSS 中排在断点之后，能压过 `md:text-sm`）。
 * 业务封装组件经 tailwind-merge 合并，调用方传 `text-sm` 不会删掉封装里的
 * `pointer-coarse:text-base`，所以只拦截显式写小的粗指针字号。
 */
const inputFontSize = {
  meta: meta(
    "tooSmall",
    "Require text inputs to use >=16px font on touch devices to avoid iOS focus zoom.",
    "MOB-03：`<{{tag}}>` 在触屏上字号为 `{{cls}}`（< 16px），iOS 聚焦时会放大整页。追加 `pointer-coarse:text-base`。",
  ),
  create(context) {
    return {
      JSXOpeningElement(node) {
        const kind = inputKind(node);
        if (!kind) return;
        const sized = elementTokens(node)
          .map((t) => ({ t, px: fontSizePx(t.utility) }))
          .filter(({ px }) => px !== null);
        const coarse = sized.filter(({ t }) => t.variants.some(isCoarse));
        let bad;
        if (kind === "wrapper") {
          bad = coarse.find(({ px }) => px < MIN_INPUT_FONT_PX);
        } else if (!coarse.some(({ px }) => px >= MIN_INPUT_FONT_PX)) {
          bad = sized.find(
            ({ t, px }) =>
              px < MIN_INPUT_FONT_PX && t.variants.every((v) => isResponsive(v) || v === "dark"),
          );
        }
        if (bad) {
          context.report({
            node,
            messageId: "tooSmall",
            data: { tag: jsxElementName(node), cls: bad.t.raw },
          });
        }
      },
    };
  },
};

/* ------------------------------------------------------------------ */
/* MOB-04 手机上生效的固定宽度不得超过 320px                             */
/* ------------------------------------------------------------------ */

const FLUID_MAX_W = /^max-w-(full|screen|dvw|svw|lvw|\(.+\)|\[.*(%|vw|calc|min\(|var\().*\])$/;

/** `grid-cols-[1fr_22rem]` 里定长轨道之和（minmax 取下限；含 repeat() 的放行）。 */
function gridFixedPx(template) {
  if (/repeat\(/.test(template)) return 0;
  let sum = 0;
  for (const track of template.split(/_(?![^(]*\))/)) {
    const min = /^minmax\(([^,]+),/.exec(track);
    sum += lengthToPx(min ? min[1] : track) ?? 0;
  }
  return sum;
}

/**
 * 360px 手机去掉左右 16px 页面内边距后只剩约 328px。不带 `sm:` 等断点前缀的定宽、
 * 最小宽度、定长网格轨道超过 320px，就会在手机上撑出横向滚动或把内容裁掉。
 * `w-*` 搭配流式 `max-w-full` / `max-w-[calc(100vw-2rem)]` 视为已兜底；`min-w-*` 无法被
 * max-w 压回，没有豁免。数据表需要横向滚动时把定宽放进 `overflow-x-auto` 容器并显式 disable。
 */
const noFixedOverflowWidth = {
  meta: meta(
    "tooWide",
    "Disallow fixed widths wider than a phone viewport unless scoped to a breakpoint.",
    "MOB-04：`{{cls}}`（≈{{px}}px）在手机上生效，超过 320px 会撑出横向滚动。加断点前缀（如 `sm:{{utility}}`）、改流式宽度，或配 `max-w-full`。",
  ),
  create(context) {
    return {
      JSXOpeningElement(node) {
        const tokens = elementTokens(node).filter(appliesOnPhone);
        const fluid = tokens.some((t) => FLUID_MAX_W.test(t.utility));
        for (const t of tokens) {
          let px = null;
          let m;
          if ((m = /^w-(.+)$/.exec(t.utility)) && !fluid) {
            px = sizeValueToPx(m[1], { allowContainer: true });
          } else if ((m = /^min-w-(.+)$/.exec(t.utility))) {
            px = sizeValueToPx(m[1], { allowContainer: true });
          } else if ((m = /^grid-cols-\[(.+)\]$/.exec(t.utility))) {
            px = gridFixedPx(m[1]);
          }
          if (px !== null && px > PHONE_CONTENT_MAX_PX) {
            context.report({
              node,
              messageId: "tooWide",
              data: { cls: t.raw, px: Math.round(px), utility: t.utility },
            });
          }
        }
      },
    };
  },
};

/* ------------------------------------------------------------------ */
/* MOB-05 可点击元素高度 ≥ 24px                                         */
/* ------------------------------------------------------------------ */

const CLICKABLE = new Set(["button", "a", "summary", "Button"]);

/**
 * WCAG 2.2 SC 2.5.8（AA）要求触控目标至少 24×24 CSS px。显式把按钮/链接高度写到
 * 24px 以下（`h-5`、`size-4`、`h-[20px]`）时，必须给出触屏放大：
 *  - `pointer-coarse:h-6` / `pointer-coarse:size-9` 等粗指针下 ≥ 24px 的尺寸，或
 *  - `after:absolute after:-inset-2` 一类伪元素热区扩展（视觉不变、可点区域变大）。
 * 只检查显式高度；`Button` 的 size 变体（最小 icon-xs = 28px）已满足下限。
 */
const minTouchTarget = {
  meta: meta(
    "tooSmall",
    "Require clickable elements to be at least 24px tall on touch devices.",
    "MOB-05：`<{{tag}}>` 高度 `{{cls}}`（{{px}}px）低于 24px 触控下限。追加 `pointer-coarse:h-6`（或更大），或用 `after:absolute after:-inset-2` 扩展热区。",
  ),
  create(context) {
    return {
      JSXOpeningElement(node) {
        const tag = jsxElementName(node);
        if (!CLICKABLE.has(tag)) return;
        const tokens = elementTokens(node);
        const heightPx = (t) => {
          const m = /^(?:min-)?(?:h|size)-(.+)$/.exec(t.utility);
          return m ? sizeValueToPx(m[1]) : null;
        };
        const small = tokens.find((t) => {
          const px = heightPx(t);
          return (
            px !== null &&
            px < MIN_TOUCH_TARGET_PX &&
            appliesOnPhone(t) &&
            !t.variants.some(isCoarse)
          );
        });
        if (!small) return;
        const enlarged = tokens.some(
          (t) =>
            (appliesOnPhone(t) && (heightPx(t) ?? 0) >= MIN_TOUCH_TARGET_PX) ||
            (/^(after|before)$/.test(t.variants[0]) && /^-?inset/.test(t.utility)),
        );
        if (enlarged) return;
        context.report({
          node,
          messageId: "tooSmall",
          data: { tag, cls: small.raw, px: heightPx(small) },
        });
      },
    };
  },
};

const mobilePlugin = {
  rules: {
    "no-viewport-vh": noViewportVh,
    "hover-reveal-needs-touch": hoverRevealNeedsTouch,
    "input-font-size": inputFontSize,
    "no-fixed-overflow-width": noFixedOverflowWidth,
    "min-touch-target": minTouchTarget,
  },
};

export default mobilePlugin;
