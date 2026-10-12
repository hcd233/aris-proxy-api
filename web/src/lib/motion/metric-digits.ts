/* Digit plan for MetricNumber (spec §7.3): which glyph cells change between
 * two values, right-aligned like an odometer. A first render (`from` null)
 * rolls every digit up from 0. */

export interface DigitCell {
  from: string;
  to: string;
  changed: boolean;
}

export interface DigitPlan {
  /** 1 increase, -1 decrease, 0 unchanged — the odometer roll direction. */
  direction: -1 | 0 | 1;
  cells: DigitCell[];
}

const DIGIT = /\d/;

export function planDigits(from: number | null, to: number): DigitPlan {
  const toStr = String(to);
  const fromStr = from === null ? toStr.replace(/\d/g, "0") : String(from);
  const direction = (from === null ? 1 : Math.sign(to - from)) as DigitPlan["direction"];
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
  return { direction, cells };
}
