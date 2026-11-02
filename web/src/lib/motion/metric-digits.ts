/* Digit plan for MetricNumber (spec §7.3): which glyph cells change between
 * two values, right-aligned like an odometer. A first render (`from` null)
 * rolls every digit up from 0. Values may be pre-formatted text ("1.2K",
 * "99.50%", "$3.40"): only digits roll, other glyphs stay put. */

export interface DigitCell {
  from: string;
  to: string;
  changed: boolean;
}

export interface DigitPlan {
  /** 1 increase, -1 decrease, 0 unchanged — the odometer roll direction.
   * Formatted text has no numeric order, so any change counts as 1. */
  direction: -1 | 0 | 1;
  cells: DigitCell[];
}

const DIGIT = /\d/;

function planDirection(from: number | string | null, to: number | string): DigitPlan["direction"] {
  if (from === null) return 1;
  if (typeof from === "number" && typeof to === "number") {
    return Math.sign(to - from) as DigitPlan["direction"];
  }
  return String(from) === String(to) ? 0 : 1;
}

export function planDigits(from: number | string | null, to: number | string): DigitPlan {
  const toStr = String(to);
  const fromStr = from === null ? toStr.replace(/\d/g, "0") : String(from);
  const cells: DigitCell[] = [];
  for (let i = 0; i < toStr.length; i++) {
    const t = toStr[i];
    if (!DIGIT.test(t)) {
      cells.push({ from: t, to: t, changed: false });
      continue;
    }
    const fi = fromStr.length - (toStr.length - i);
    const raw = fi >= 0 ? fromStr[fi] : "0";
    const f = DIGIT.test(raw) ? raw : "0";
    cells.push({ from: f, to: t, changed: f !== t });
  }
  return { direction: planDirection(from, to), cells };
}
