/**
 * Markdown 文本预处理（渲染前的一次性清洗）。
 *
 * 数据来源是模型输出，而模型输出会被 `max_tokens` 从任意位置切断：最常见的残骸是
 * 落单的 `**`（示例：`核心目标都是：**尽量拦掉无效请求……并最终`）。react-markdown
 * 遇到不成对的强调符号会直接把它当字面量输出，于是正文里冒出 `**尽量拦掉…`，
 * 而用户看到的应该是「少了一段粗体」，不是「看得见标记」。
 */

/** 围栏代码块：清洗时必须整体跳过，否则会把代码里的 `**` 当强调处理 */
const FENCED_CODE_RE = /(```[\s\S]*?```|~~~[\s\S]*?~~~)/;

/** 去掉最后一个没有配对的 `**`；配对数量为偶数时原样返回 */
function dropTrailingUnpairedBold(part: string): string {
  const marks = [...part.matchAll(/\*\*/g)];
  if (marks.length % 2 === 0) return part;
  const last = marks[marks.length - 1];
  return part.slice(0, last.index) + part.slice(last.index + 2);
}

/**
 * 平衡被截断的粗体标记：只处理围栏代码块之外的 `**`，围栏块原样保留。
 * 刻意不补配对（在末尾补 `**` 会把后半段整段加粗），而是丢弃落单的那个。
 */
export function balanceEmphasis(text: string): string {
  return text
    .split(FENCED_CODE_RE)
    .map((part, i) => (i % 2 === 1 ? part : dropTrailingUnpairedBold(part)))
    .join("");
}
