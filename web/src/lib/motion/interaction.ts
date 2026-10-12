/* Pure geometry for InteractionLayer (spec §5.2 / §6.2). Viewport CSS px in,
 * CSS-ready numbers out. */

export interface Box {
  left: number;
  top: number;
  width: number;
  height: number;
}

export const MAGNET_RANGE_PX = 60;
export const MAGNET_MAX_PX = 4;
const MAGNET_PULL = 0.12;
export const TILT_MAX_DEG = 4;

const clamp = (v: number, lo: number, hi: number) => Math.min(hi, Math.max(lo, v));

/** Magnetic offset (px) for a button: pulled toward the pointer, clamped to
 * MAGNET_MAX_PX, fading linearly to 0 at MAGNET_RANGE_PX from its edge. */
export function magnetTarget(r: Box, px: number, py: number): { x: number; y: number } {
  const outX = Math.max(r.left - px, 0, px - (r.left + r.width));
  const outY = Math.max(r.top - py, 0, py - (r.top + r.height));
  const dist = Math.hypot(outX, outY);
  if (dist >= MAGNET_RANGE_PX) return { x: 0, y: 0 };
  const falloff = 1 - dist / MAGNET_RANGE_PX;
  const dx = px - (r.left + r.width / 2);
  const dy = py - (r.top + r.height / 2);
  return {
    x: clamp(dx * MAGNET_PULL, -MAGNET_MAX_PX, MAGNET_MAX_PX) * falloff,
    y: clamp(dy * MAGNET_PULL, -MAGNET_MAX_PX, MAGNET_MAX_PX) * falloff,
  };
}

/** Card tilt (deg): the edge under the pointer is pressed away. CSS
 * rotateX(+) sends the top edge back, rotateY(+) sends the right edge back. */
export function tiltFor(r: Box, px: number, py: number): { rx: number; ry: number } {
  const nx = clamp((px - r.left) / r.width, 0, 1);
  const ny = clamp((py - r.top) / r.height, 0, 1);
  return {
    rx: (0.5 - ny) * 2 * TILT_MAX_DEG,
    ry: (nx - 0.5) * 2 * TILT_MAX_DEG,
  };
}
