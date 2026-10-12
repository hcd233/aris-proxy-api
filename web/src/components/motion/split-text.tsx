import { useMemo, type CSSProperties } from "react";

function graphemes(text: string): string[] {
  if (typeof Intl !== "undefined" && "Segmenter" in Intl) {
    const seg = new Intl.Segmenter(undefined, { granularity: "grapheme" });
    return Array.from(seg.segment(text), (s) => s.segment);
  }
  return Array.from(text);
}

/**
 * Per-character entrance (login titles, spec §7.2). Splits by grapheme
 * (CJK / emoji safe); each glyph gets its index as `--i`, and the active
 * Motion World's `.split-char` rule (styles/motion-signature.css) staggers
 * them. Screen readers get the intact string; the glyph spans are hidden.
 */
export function SplitText({ text, className }: { text: string; className?: string }) {
  const chars = useMemo(() => graphemes(text), [text]);
  return (
    <span className={className}>
      <span className="sr-only">{text}</span>
      <span aria-hidden="true">
        {chars.map((ch, i) => (
          <span key={i} className="split-char" style={{ "--i": i } as CSSProperties}>
            {ch}
          </span>
        ))}
      </span>
    </span>
  );
}
