/**
 * 自定义 lint 规则共用的 className 静态分析工具。
 *
 * 只认字符串字面量片段（直接字符串、模板字符串 quasis、cn()/三元/逻辑/数组/
 * 对象键里的字面量）；变量、成员表达式等动态值静态不可见，一律放行（宁漏报不误报）。
 */

export function collectStringLiterals(node, out) {
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
    case "ObjectExpression":
      // cn({ truncate: cond }) 对象键形式：类名在键上（含简写 { truncate }）；
      // 计算键/展开元素是动态值不可见，只收集字面量键与值里的字面量
      for (const prop of node.properties) {
        if (prop.type !== "Property") continue;
        if (!prop.computed && prop.key.type === "Identifier") out.push(prop.key.name);
        else collectStringLiterals(prop.key, out);
        collectStringLiterals(prop.value, out);
      }
      return;
    default:
      return; // Identifier / MemberExpression / SpreadElement 等动态值：不可见
  }
}

/** JSX 元素名：`div` / `Input` 原样返回，`Foo.Bar` 返回 `Foo.Bar`，其余返回 null。 */
export function jsxElementName(opening) {
  const { name } = opening;
  if (name.type === "JSXIdentifier") return name.name;
  if (name.type === "JSXMemberExpression") {
    const parts = [];
    for (let n = name; n; n = n.type === "JSXMemberExpression" ? n.object : null) {
      parts.unshift(n.type === "JSXMemberExpression" ? n.property.name : n.name);
    }
    return parts.join(".");
  }
  return null;
}

/** 元素上所有 `className` / `xxxClassName` 属性。 */
export function classAttributes(opening) {
  return opening.attributes.filter(
    (attr) =>
      attr.type === "JSXAttribute" &&
      attr.name.type === "JSXIdentifier" &&
      /^className$|ClassName$/.test(attr.name.name),
  );
}

/** 读取 JSX 属性的字符串字面量值（`type="checkbox"` → "checkbox"），动态值返回 null。 */
export function literalAttribute(opening, attrName) {
  const attr = opening.attributes.find(
    (a) => a.type === "JSXAttribute" && a.name.type === "JSXIdentifier" && a.name.name === attrName,
  );
  if (!attr?.value) return null;
  if (attr.value.type === "Literal" && typeof attr.value.value === "string")
    return attr.value.value;
  return null;
}
