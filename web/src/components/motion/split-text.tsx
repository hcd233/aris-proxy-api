import { useMemo, type CSSProperties } from "react";
import { BREAK_HINT, splitTokens } from "@/lib/split-words";

type Item = { kind: "text"; text: string } | { kind: "word"; glyphs: { ch: string; i: number }[] };

/** 把 token 映射成渲染项，并一次性算好每个字符的 stagger 序号。 */
function toItems(text: string): Item[] {
  let next = 0;
  return splitTokens(text).map((token): Item => {
    if (token.space) {
      // 提示符不占位也不占 stagger 序号；真空白占一个序号，保持原有节奏
      if (token.chars[0] !== BREAK_HINT) next += token.chars.length;
      return { kind: "text", text: token.chars.join("") };
    }
    return { kind: "word", glyphs: token.chars.map((ch) => ({ ch, i: next++ })) };
  });
}

/**
 * Per-character entrance (login titles, spec §7.2). Splits by grapheme
 * (CJK / emoji safe); each glyph gets its index as `--i`, and the active
 * Motion World's `.split-char` rule (styles/motion-signature.css) staggers
 * them. Screen readers get the intact string; the glyph spans are hidden.
 *
 * Glyphs are grouped into `.split-word` boxes (inline-block, nowrap) so a line
 * can only break between words, never inside one. Spaces and U+200B hints
 * (see lib/split-words.ts) stay plain text nodes: they are the break points.
 */
export function SplitText({ text, className }: { text: string; className?: string }) {
  const items = useMemo(() => toItems(text), [text]);
  return (
    <span className={className}>
      <span className="sr-only">{text.replaceAll(BREAK_HINT, "")}</span>
      <span aria-hidden="true">
        {items.map((item, idx) =>
          item.kind === "text" ? (
            item.text
          ) : (
            <span key={idx} className="split-word">
              {item.glyphs.map(({ ch, i }) => (
                <span key={i} className="split-char" style={{ "--i": i } as CSSProperties}>
                  {ch}
                </span>
              ))}
            </span>
          ),
        )}
      </span>
    </span>
  );
}
