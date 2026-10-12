/* Shared GLSL for Ambient Stage scenes (spec §4.3). Every scene fragment is
 * FRAGMENT_PRELUDE + its own body; the stage sets the uniforms below each
 * rendered frame. Keep this module free of `@/` imports (see gl.ts). */

export type SceneId = "atelier" | "lunar";

export interface AmbientScene {
  id: SceneId;
  /** Full fragment source: FRAGMENT_PRELUDE + scene body. */
  fragment: string;
  /** Intro duration (atelier lamp-on / lunar moonrise), ms. */
  introMs: number;
  /** Pointer smoothing time constant, ms (atelier: lazy lamp; lunar: snappy lens). */
  pointerTauMs: number;
}

export const PULSE_SLOTS = 4;

/** uPulse[i].w codes; 0 marks an empty slot. */
export const PULSE_KIND_CODE = { primary: 1, route: 2 } as const;

/** Fullscreen triangle from gl_VertexID — no vertex buffers. */
export const VERTEX_SOURCE = `#version 300 es
void main() {
  vec2 p = vec2(float((gl_VertexID << 1) & 2), float(gl_VertexID & 2));
  gl_Position = vec4(p * 2.0 - 1.0, 0.0, 1.0);
}
`;

export const FRAGMENT_PRELUDE = `#version 300 es
precision highp float;

uniform vec2 uResolution;           // drawing-buffer px
uniform float uTime;                // s since stage start
uniform float uDrift;               // s, presence/focus-scaled drift clock
uniform vec2 uPointer;              // smoothed pointer, -1..1, y up
uniform float uScroll;              // scrollTop / viewport height
uniform float uPresence;            // 0.35 (busy) .. 1 (idle)
uniform float uFocus;               // 0..1, dialog focus
uniform float uIntro;               // 0..1, eased intro progress
uniform vec4 uPulse[${PULSE_SLOTS}]; // xy uv (y up), z start time (s), w kind (1 primary, 2 route)

out vec4 outColor;

float hash12(vec2 p) {
  vec3 p3 = fract(vec3(p.xyx) * 0.1031);
  p3 += dot(p3, p3.yzx + 33.33);
  return fract((p3.x + p3.y) * p3.z);
}

vec2 hash22(vec2 p) {
  vec3 p3 = fract(vec3(p.xyx) * vec3(0.1031, 0.1030, 0.0973));
  p3 += dot(p3, p3.yzx + 33.33);
  return fract((p3.xx + p3.yz) * p3.zy);
}

float vnoise(vec2 p) {
  vec2 i = floor(p);
  vec2 f = fract(p);
  vec2 u = f * f * (3.0 - 2.0 * f);
  return mix(mix(hash12(i), hash12(i + vec2(1.0, 0.0)), u.x),
             mix(hash12(i + vec2(0.0, 1.0)), hash12(i + vec2(1.0, 1.0)), u.x), u.y);
}

float fbm(vec2 p) {
  float v = 0.0;
  float a = 0.5;
  mat2 r = mat2(0.8, 0.6, -0.6, 0.8);
  for (int i = 0; i < 5; i++) {
    v += a * vnoise(p);
    p = r * p * 2.02;
    a *= 0.5;
  }
  return v;
}

// 0.35 mirrors PRESENCE_BUSY in lib/motion/attention.ts.
float presenceN() {
  return clamp((uPresence - 0.35) / 0.65, 0.0, 1.0);
}

float ditherNoise(vec2 fc) {
  return (hash12(fc) - 0.5) / 255.0;
}
`;
