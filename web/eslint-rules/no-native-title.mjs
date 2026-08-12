/**
 * 禁止在原生 HTML/JSX 元素上使用 `title` 属性。
 *
 * 背景：原生 `title` 属性渲染浏览器默认 tooltip（样式丑、延迟不可控、无法跨主题），
 * 项目统一约定悬停提示使用 shadcn/base-ui 的 Tooltip 组件：
 *   <TooltipProvider><TooltipRoot><TooltipTrigger render={<button .../>} />
 *   <TooltipContent>提示文案</TooltipContent></TooltipRoot></TooltipProvider>
 *
 * 判定：只拦截小写开头的原生元素（`<button title=...>`、`<div title=...>` 等）；
 * React 组件的 `title` prop（如 `<Dialog title=...>`、`<PageHeader title=...>`）是
 * 组件业务属性，不属于悬停提示，不在本规则范围内。
 */

const noNativeTitle = {
  meta: {
    type: "problem",
    docs: {
      description:
        "Disallow native `title` attribute on HTML elements; use the Tooltip component for hover hints.",
      recommended: true,
    },
    schema: [],
    messages: {
      useTooltip:
        "Avoid native `title` attribute on `<{{tag}}>`: it renders the browser-default tooltip. Use the Tooltip component (TooltipRoot / TooltipTrigger / TooltipContent) for hover hints instead.",
    },
  },
  create(context) {
    return {
      JSXOpeningElement(node) {
        const { name } = node;
        // 组件（JSXMemberExpression，或大写开头的 JSXIdentifier）的 title prop 不受影响
        if (name.type !== "JSXIdentifier") return;
        const tag = name.name;
        if (!/^[a-z]/.test(tag)) return;

        const titleAttr = node.attributes.find(
          (attr) =>
            attr.type === "JSXAttribute" &&
            attr.name.type === "JSXIdentifier" &&
            attr.name.name === "title",
        );
        if (titleAttr) {
          context.report({ node: titleAttr, messageId: "useTooltip", data: { tag } });
        }
      },
    };
  },
};

export default noNativeTitle;
