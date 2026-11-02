/**
 * 逐字入场动画（SplitText）用的分词：把文本切成「词」与「空白」两类 token，词内不允许换行。
 *
 * 为什么不能直接逐字 inline-block：每个 inline-block 两侧都是换行机会，
 * 中文会把「接入」拆成「接 / 入」，英文会把 model 断成 mod / el。
 * 这里按 Intl.Segmenter 的词边界分组，再让组件给每个词加 nowrap。
 */
export interface SplitToken {
  /** 空白 token 是唯一的换行机会，渲染成普通文本节点 */
  space: boolean;
  /** 按字素簇切好的字符（emoji / 组合字符安全） */
  chars: string[];
}

interface Segment {
  segment: string;
  isWordLike: boolean;
}

const SPACE = /^\s+$/u;
/** 开括号 / 开引号：必须贴着后面的词，否则会孤零零留在行尾 */
const OPENING = /^[\p{Ps}\p{Pi}]+$/u;

export function graphemes(text: string): string[] {
  if (typeof Intl !== "undefined" && "Segmenter" in Intl) {
    return Array.from(
      new Intl.Segmenter(undefined, { granularity: "grapheme" }).segment(text),
      (s) => s.segment,
    );
  }
  return Array.from(text);
}

function wordSegments(text: string): Segment[] {
  if (typeof Intl !== "undefined" && "Segmenter" in Intl) {
    return Array.from(
      new Intl.Segmenter(undefined, { granularity: "word" }).segment(text),
      (s) => ({ segment: s.segment, isWordLike: Boolean(s.isWordLike) }),
    );
  }
  // 无 Segmenter 的老环境：退化为按空白切，词内仍整体不换行
  return text
    .split(/(\s+)/u)
    .filter(Boolean)
    .map((segment) => ({ segment, isWordLike: !SPACE.test(segment) }));
}

/** 文案里用它显式标出允许断行的位置；出现时 CJK 连续文字只在这里断（见 splitTokens）。 */
export const BREAK_HINT = "\u200b";

export function splitTokens(text: string): SplitToken[] {
  if (!text.includes(BREAK_HINT)) return segmentTokens(text);
  return text.split(BREAK_HINT).flatMap((phrase, i) => {
    const hint: SplitToken[] = i > 0 ? [{ space: true, chars: [BREAK_HINT] }] : [];
    return [...hint, ...mergeAdjacentWords(segmentTokens(phrase))];
  });
}

/** 没有空白隔开的相邻词（CJK 连续文字）合成一个不可断的整体。 */
function mergeAdjacentWords(tokens: SplitToken[]): SplitToken[] {
  const merged: SplitToken[] = [];
  for (const token of tokens) {
    const prev = merged.at(-1);
    if (prev && !prev.space && !token.space) prev.chars.push(...token.chars);
    else merged.push({ space: token.space, chars: [...token.chars] });
  }
  return merged;
}

function segmentTokens(text: string): SplitToken[] {
  const tokens: SplitToken[] = [];
  let pendingOpen: string[] = [];

  const flushPending = () => {
    if (pendingOpen.length === 0) return;
    tokens.push({ space: false, chars: pendingOpen });
    pendingOpen = [];
  };

  for (const { segment, isWordLike } of wordSegments(text)) {
    const chars = graphemes(segment);
    if (SPACE.test(segment)) {
      flushPending();
      tokens.push({ space: true, chars });
    } else if (isWordLike) {
      tokens.push({ space: false, chars: [...pendingOpen, ...chars] });
      pendingOpen = [];
    } else if (OPENING.test(segment)) {
      pendingOpen.push(...chars);
    } else {
      // 其余标点 / 符号：贴在前一个词后面，避免行首出现「，」「。」
      const prev = tokens.at(-1);
      if (prev && !prev.space) prev.chars.push(...chars);
      else tokens.push({ space: false, chars });
    }
  }
  flushPending();
  return tokens;
}
