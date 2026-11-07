/**
 * Y 轴宽度不要写死。
 *
 * 背景：recharts 的 Y 轴刻度是「右对齐到轴内侧」的，轴宽写死时，超过
 * `width - tickSize - tickMargin` 的刻度文本会越出 SVG 左边缘被裁掉。实测
 * `width={40}`（tickMargin 默认 8）下 5 字符刻度 `160ms`（12px 等宽 = 36px）
 * 从 `1` 的旗子开始被切，线上读作 `l60ms`。刻度宽度随数据（`ms`/`s` 单位切换）
 * 与语言变化，任何常数都不安全——`width={44}` 只对 5 字符恰好压线，加一位小数
 * 又会溢出。
 *
 * 规范（docs/agents/web-frontend.md「图表契约」）：`<YAxis width="auto" />`，
 * 由 recharts 用 `getCalculatedYAxisWidth` 按实际渲染的刻度测量轴宽。
 *
 * 检测：仅 `<YAxis>` 上的 `width` 属性；数字字面量（`width={40}`、`width="44"`
 * 等）报错。`width="auto"`、动态变量、不写 width 一律放行（宁漏报不误报）——
 * 动态值可能来自画布测量，静态不可知。
 */

function isNumericLiteral(node) {
  if (!node) return false;
  if (node.type === "Literal") {
    if (typeof node.value === "number") return true;
    if (typeof node.value === "string" && /^\d+$/.test(node.value.trim())) return true;
    return false;
  }
  if (node.type === "UnaryExpression" && (node.operator === "-" || node.operator === "+")) {
    return isNumericLiteral(node.argument);
  }
  return false;
}

const noHardcodedYAxisWidth = {
  meta: {
    type: "problem",
    docs: {
      description:
        'Disallow hard-coded YAxis widths; use width="auto" so tick labels are measured instead of clipped.',
      recommended: true,
    },
    schema: [],
    messages: {
      hardcodedWidth:
        '<YAxis> must not hard-code `width` ({{value}}). Use `width="auto"` so recharts measures the rendered tick labels; a fixed width clips long labels (e.g. `160ms` renders as `l60ms`).',
    },
  },
  create(context) {
    return {
      JSXOpeningElement(node) {
        if (node.name.type !== "JSXIdentifier" || node.name.name !== "YAxis") return;
        const widthAttr = node.attributes.find(
          (attr) =>
            attr.type === "JSXAttribute" &&
            attr.name.type === "JSXIdentifier" &&
            attr.name.name === "width",
        );
        if (!widthAttr) return;
        const expression =
          widthAttr.value?.type === "JSXExpressionContainer"
            ? widthAttr.value.expression
            : widthAttr.value;
        if (!isNumericLiteral(expression)) return;
        context.report({
          node: widthAttr,
          messageId: "hardcodedWidth",
          data: { value: String(expression.value) },
        });
      },
    };
  },
};

export default noHardcodedYAxisWidth;
