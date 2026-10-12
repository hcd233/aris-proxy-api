import { FRAGMENT_PRELUDE, type AmbientScene } from "./common";

/* 纸与墨 · Atelier (spec §5.3): fibrous paper lit by a raking desk lamp that
 * lazily follows the pointer, ink drops diffusing where the user acts.
 * Lighting is clamped to ±3% so text on bare paper keeps its contrast. */

const ATELIER_BODY = `
const vec3 PAPER = vec3(0.9804, 0.9725, 0.9608);  // --background #faf8f5
const vec3 INK = vec3(0.8510, 0.4667, 0.3412);    // --primary #d97757
const vec3 SEPIA = vec3(0.4275, 0.3333, 0.2549);
const float LIGHT_RANGE = 0.03;

float paperHeight(vec2 p) {
  float fibers = fbm(p * vec2(0.9, 5.0));          // stretched along x
  float grain = vnoise(p * 9.0);
  return fibers * 0.75 + grain * 0.25;
}

// One ink drop: diffuses for 1.4s, dries to a faint stain, gone by 7.4s.
float inkDrop(vec2 s, vec4 pulse, float idx, float aspect) {
  float age = uTime - pulse.z;
  if (pulse.w < 0.5 || age < 0.0 || age > 7.4) return 0.0;
  vec2 c = vec2(pulse.x * aspect, pulse.y);
  float size = pulse.w > 1.5 ? 0.06 : 0.09;        // route drops are smaller
  float r = size * (1.0 - exp(-age * 2.2));
  float d = length(s - c) + (fbm((s - c) * 14.0 + idx * 7.3) - 0.5) * 0.025;
  float wet = 1.0 - smoothstep(r * 0.6, r, d);
  float rim = smoothstep(r * 0.55, r * 0.95, d) * (1.0 - smoothstep(r * 0.95, r * 1.05, d));
  float peak = 0.06 * smoothstep(0.0, 0.2, age)
             * mix(1.0, 0.25, smoothstep(1.4, 2.2, age))
             * (1.0 - smoothstep(2.2, 7.4, age));
  return clamp(wet * 0.7 + rim * 0.6, 0.0, 1.0) * peak;
}

void main() {
  vec2 fc = gl_FragCoord.xy;
  vec2 uv = fc / uResolution;
  float aspect = uResolution.x / uResolution.y;
  vec2 s = vec2(uv.x * aspect, uv.y);              // screen-height units
  vec2 p = s * 6.0;

  // Height → normal via screen-space derivatives (one fbm per pixel).
  float h = paperHeight(p);
  vec2 g = vec2(dFdx(h), dFdy(h)) * (uResolution.y / 6.0);
  vec3 n = normalize(vec3(-g * 0.08, 1.0));

  float sway = sin(uDrift * 0.21) * 0.035;          // ~±2° idle breathing
  vec3 L = normalize(vec3(uPointer * 0.8 + vec2(-0.4 + sway, 0.5), 0.6));
  float shade = (dot(n, L) - L.z) * 0.15;
  vec2 lampUV = uPointer * 0.5 + 0.5;
  float pool = (1.0 - smoothstep(0.0, 1.2, distance(uv, lampUV))) * 0.015;
  float light = clamp((shade + pool) * uIntro, -LIGHT_RANGE, LIGHT_RANGE);
  vec3 col = PAPER * (1.0 + light);

  float vig = smoothstep(0.6, 1.4, length((uv - 0.5) * 2.0)) * 0.06;
  col *= 1.0 - vig * vec3(0.85, 0.95, 1.0);         // warm: blue darkens most

  float ink = 0.0;
  for (int i = 0; i < 4; i++) ink = max(ink, inkDrop(s, uPulse[i], float(i), aspect));
  ink *= mix(0.5, 1.0, presenceN());
  col = mix(col, mix(INK, SEPIA, 0.5), ink);

  outColor = vec4(col + ditherNoise(fc), 1.0);
}
`;

export const atelierScene: AmbientScene = {
  id: "atelier",
  fragment: FRAGMENT_PRELUDE + ATELIER_BODY,
  introMs: 800,
  pointerTauMs: 1200,
};
