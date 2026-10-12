/* Attention signal → Ambient Presence (spec §4.3). Active input pushes the
 * ambient layer back within BUSY_FALL_MS; after IDLE_DELAY_MS of calm it
 * returns over IDLE_RISE_MS. pointermove is deliberately NOT activity: it
 * drives the lamp / parallax and must not dim the scene it animates.
 * PRESENCE_BUSY is mirrored in scenes/common.ts presenceN(). */

export const PRESENCE_BUSY = 0.35;
export const BUSY_FALL_MS = 300;
export const IDLE_DELAY_MS = 8000;
export const IDLE_RISE_MS = 2000;

export interface AttentionState {
  presence: number;
  lastActiveAt: number;
}

export function createAttention(): AttentionState {
  return { presence: 1, lastActiveAt: Number.NEGATIVE_INFINITY };
}

export function markActive(s: AttentionState, now: number): AttentionState {
  return { ...s, lastActiveAt: now };
}

export function stepAttention(s: AttentionState, now: number, dtMs: number): AttentionState {
  const span = 1 - PRESENCE_BUSY;
  const busy = now - s.lastActiveAt < IDLE_DELAY_MS;
  const presence = busy
    ? Math.max(PRESENCE_BUSY, s.presence - (span * dtMs) / BUSY_FALL_MS)
    : Math.min(1, s.presence + (span * dtMs) / IDLE_RISE_MS);
  return presence === s.presence ? s : { ...s, presence };
}

const ACTIVITY_EVENTS = ["keydown", "input", "wheel", "scroll", "pointerdown"] as const;

/** Capture-phase listeners so scrolls inside nested containers (dashboard
 * <main>) count too. Returns an unbind function. */
export function bindAttention(onActive: () => void): () => void {
  const opts: AddEventListenerOptions = { capture: true, passive: true };
  for (const type of ACTIVITY_EVENTS) window.addEventListener(type, onActive, opts);
  return () => {
    for (const type of ACTIVITY_EVENTS) window.removeEventListener(type, onActive, opts);
  };
}
