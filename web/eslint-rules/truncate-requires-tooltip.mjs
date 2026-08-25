/**
 * 截断文案必须配 Tooltip。
 *
 * 背景：`truncate` / `line-clamp-1` 会让文本视觉截断，用户拿不到完整内容。
 * 项目契约（docs/agents/web-frontend.md）：截断文本用 Tooltip 组件展示完整内容：
 *   <TooltipRoot>
 *     <TooltipTrigger render={<span className="… truncate">…</span>} />
 *     <TooltipContent>完整内容</TooltipContent>
 *   </TooltipRoot>
 *
 * 检测：className 的字符串字面量片段（直接字符串、模板字符串 quasis、
 * cn()/三元/逻辑/数组表达式内的字面量）按空白切 token 精确命中
 * `truncate` 或 `line-clamp-1`。纯变量传递的类名静态不可见，放行
 * （宁漏报不误报）。line-clamp-2+（卡片描述）按设计豁免。
 *
 * 判定：沿 parent 链向上，遇 TooltipTrigger（渲染子树 = hover 热区）或
 * TooltipContent（嵌套 tooltip 不可行，边角豁免）视为达标；遇
 * TooltipRoot / TooltipProvider（在 Root 内但不在 Trigger 子树，hover
 * 不触发）或到 AST 顶则报错。中间的原生元素/其他组件正常穿越。
 */

const TRUNCATING_CLASSES = new Set(["truncate", "line-clamp-1"]);
const TOOLTIP_PASS_COMPONENTS = new Set(["TooltipTrigger", "TooltipContent"]);
const TOOLTIP_FAIL_COMPONENTS = new Set(["TooltipRoot", "TooltipProvider"]);

function jsxElementName(opening) {
  if (opening.name.type === "JSXIdentifier") return opening.name.name;
  return null; // JSXMemberExpression（Foo.Bar）不参与判定
}

function collectStringLiterals(node, out) {
  if (!node) return;
  switch (node.type) {
    case "Literal":
      if (typeof node.value === "string") out.push(node.value);
      return;
    case "TemplateLiteral":
      for (const quasi of node.quasis) out.push(quasi.value.raw);
      return;
    case "JSXExpressionContainer":
      collectStringLiterals(node.expression, out);
      return;
    case "ConditionalExpression":
      collectStringLiterals(node.consequent, out);
      collectStringLiterals(node.alternate, out);
      return;
    case "LogicalExpression":
      collectStringLiterals(node.left, out);
      collectStringLiterals(node.right, out);
      return;
    case "CallExpression":
      for (const arg of node.arguments) collectStringLiterals(arg, out);
      return;
    case "ArrayExpression":
      for (const el of node.elements) collectStringLiterals(el, out);
      return;
    default:
      return; // Identifier / MemberExpression / SpreadElement 等动态值：不可见
  }
}

function findTruncatingClass(classNameAttr) {
  const literals = [];
  collectStringLiterals(classNameAttr.value, literals);
  for (const literal of literals) {
    for (const token of literal.split(/\s+/)) {
      if (TRUNCATING_CLASSES.has(token)) return token;
    }
  }
  return null;
}

function coveredByTooltipTrigger(node) {
  for (let p = node.parent; p; p = p.parent) {
    // parent 链上组件可能以两种形态出现：
    // - JSXOpeningElement：截断元素经 render prop（JSXAttribute.parent）进入
    // - JSXElement：截断元素经 children 进入（JSXElement.parent = 外层 JSXElement）
    let opening = null;
    if (p.type === "JSXOpeningElement") opening = p;
    else if (p.type === "JSXElement") opening = p.openingElement;
    if (!opening) continue;
    const name = jsxElementName(opening);
    if (!name) continue;
    if (TOOLTIP_PASS_COMPONENTS.has(name)) return true;
    if (TOOLTIP_FAIL_COMPONENTS.has(name)) return false;
  }
  return false;
}

const truncateRequiresTooltip = {
  meta: {
    type: "problem",
    docs: {
      description:
        "Require visually truncated text (truncate / line-clamp-1) to be hoverable via the Tooltip component.",
      recommended: true,
    },
    schema: [],
    messages: {
      needsTooltip:
        "`<{{tag}}>` uses `{{cls}}` which visually truncates text. Wrap it in a Tooltip: `<TooltipTrigger render={<.../>}>` + `<TooltipContent>` with the full text so users can hover to read it.",
    },
  },
  create(context) {
    return {
      JSXOpeningElement(node) {
        const classNameAttr = node.attributes.find(
          (attr) =>
            attr.type === "JSXAttribute" &&
            attr.name.type === "JSXIdentifier" &&
            attr.name.name === "className",
        );
        if (!classNameAttr) return;
        const cls = findTruncatingClass(classNameAttr);
        if (!cls) return;
        if (coveredByTooltipTrigger(node)) return;
        context.report({
          node: classNameAttr,
          messageId: "needsTooltip",
          data: { tag: jsxElementName(node) ?? "unknown", cls },
        });
      },
    };
  },
};

export default truncateRequiresTooltip;
