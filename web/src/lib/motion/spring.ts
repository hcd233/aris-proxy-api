/* Damped spring helpers: a stable per-frame integrator for pointer-driven
 * motion (magnetic buttons) and a sampler that turns a spring's step
 * response into a CSS linear() easing (--ease-orbit). */

export interface SpringParams {
  stiffness: number;
  damping: number;
  mass: number;
}

export interface SpringState {
  x: number;
  v: number;
}

const MAX_SUBSTEP_S = 0.004;
const REST_EPSILON = 0.01;

export function dampingRatio(p: SpringParams): number {
  return p.damping / (2 * Math.sqrt(p.stiffness * p.mass));
}

/** Semi-implicit Euler with ≤4ms substeps, so long frames stay stable. */
export function springStep(
  s: SpringState,
  target: number,
  dtSec: number,
  p: SpringParams,
): SpringState {
  const steps = Math.max(1, Math.ceil(dtSec / MAX_SUBSTEP_S));
  const h = dtSec / steps;
  let { x, v } = s;
  for (let i = 0; i < steps; i++) {
    const a = (-p.stiffness * (x - target) - p.damping * v) / p.mass;
    v += a * h;
    x += v * h;
  }
  return { x, v };
}

export function springAtRest(s: SpringState, target: number, epsilon = REST_EPSILON): boolean {
  return Math.abs(s.x - target) < epsilon && Math.abs(s.v) < epsilon;
}

/** Step response 0→1 sampled at `points` evenly spaced instants across the
 * settle time; first sample is 0, last is exactly 1, values rounded to 3dp. */
export function sampleSpringEasing(p: SpringParams, points: number): number[] {
  const dt = 0.001;
  const trace: number[] = [];
  let s: SpringState = { x: 0, v: 0 };
  trace.push(0);
  while (trace.length < 10_000) {
    s = springStep(s, 1, dt, p);
    trace.push(s.x);
    if (springAtRest(s, 1, 0.001)) break;
  }
  const last = trace.length - 1;
  const out: number[] = [];
  for (let i = 0; i < points; i++) {
    const idx = Math.round((i / (points - 1)) * last);
    out.push(Math.round(trace[idx] * 1000) / 1000);
  }
  out[0] = 0;
  out[points - 1] = 1;
  return out;
}

export function toCssLinear(samples: number[]): string {
  return `linear(${samples.join(", ")})`;
}
