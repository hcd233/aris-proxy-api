/** 思考内容字数展示：千以下原样，千以上换算 k（保留 1 位小数、去掉末尾 .0） */
export function formatCharCount(n: number): string {
  if (n < 1000) return String(n);
  return `${(n / 1000).toFixed(1).replace(/\.0$/, "")}k`;
}
