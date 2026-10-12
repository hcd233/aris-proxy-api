import { describe, expect, it } from "vitest";
import {
  BUSY_FALL_MS,
  IDLE_DELAY_MS,
  IDLE_RISE_MS,
  PRESENCE_BUSY,
  createAttention,
  markActive,
  stepAttention,
  type AttentionState,
} from "./attention";

const FRAME_MS = 16;

function advance(s: AttentionState, from: number, to: number): AttentionState {
  let state = s;
  for (let t = from + FRAME_MS; t <= to; t += FRAME_MS) state = stepAttention(state, t, FRAME_MS);
  return state;
}

describe("attention", () => {
  it("starts fully present and stays present while idle", () => {
    expect(advance(createAttention(), 0, 20_000).presence).toBe(1);
  });

  it("falls linearly during the busy fall", () => {
    const s = advance(markActive(createAttention(), 0), 0, 144);
    expect(s.presence).toBeCloseTo(1 - (1 - PRESENCE_BUSY) * (144 / BUSY_FALL_MS), 5);
  });

  it("reaches the busy floor within BUSY_FALL_MS of activity", () => {
    const s = advance(markActive(createAttention(), 0), 0, BUSY_FALL_MS + FRAME_MS);
    expect(s.presence).toBe(PRESENCE_BUSY);
  });

  it("holds the floor until IDLE_DELAY_MS has passed", () => {
    const s = advance(markActive(createAttention(), 0), 0, IDLE_DELAY_MS - FRAME_MS);
    expect(s.presence).toBe(PRESENCE_BUSY);
  });

  it("rises back to 1 over IDLE_RISE_MS after the idle delay", () => {
    const s = advance(markActive(createAttention(), 0), 0, IDLE_DELAY_MS + IDLE_RISE_MS + FRAME_MS);
    expect(s.presence).toBe(1);
  });

  it("new activity during the rise pushes it back down", () => {
    let s = advance(markActive(createAttention(), 0), 0, IDLE_DELAY_MS + 1000);
    expect(s.presence).toBeGreaterThan(PRESENCE_BUSY);
    const at = IDLE_DELAY_MS + 1000;
    s = advance(markActive(s, at), at, at + BUSY_FALL_MS + FRAME_MS);
    expect(s.presence).toBe(PRESENCE_BUSY);
  });
});
