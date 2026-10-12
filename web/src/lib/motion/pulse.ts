/* Ambient Pulse bus (spec §4.3): UI events request one-shot effects from the
 * Ambient Stage without a React context. Coordinates are viewport CSS px;
 * omitted coordinates mean "viewport center". Each scene maps kinds to its
 * own effect (atelier: ink drops; lunar: gravity ripple / warp). */

export type AmbientPulseKind = "primary" | "route" | "focus-on" | "focus-off" | "intro";

export interface AmbientPulse {
  kind: AmbientPulseKind;
  x?: number;
  y?: number;
}

type Listener = (pulse: AmbientPulse) => void;

const listeners = new Set<Listener>();

export function emitAmbientPulse(pulse: AmbientPulse): void {
  for (const listener of listeners) listener(pulse);
}

export function onAmbientPulse(listener: Listener): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}
