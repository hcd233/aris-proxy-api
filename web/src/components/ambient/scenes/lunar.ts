import { FRAGMENT_PRELUDE, type AmbientScene } from "./common";

/* 月光与引力 · Lunar Orbit (spec §6.3): a huge cropped moon in the
 * bottom-right corner (analytic sphere, crater normals, drifting sun),
 * three parallax star layers with a gravitational lens at the pointer,
 * faint nebula. Route pulses warp the stars, primary pulses ripple them. */

const LUNAR_BODY = `
const vec3 MOONLIGHT = vec3(1.0, 0.9333, 0.8510);  // #FFEED9
const vec3 ICE = vec3(0.6588, 0.7412, 0.8471);     // #A8BDD8
const float MOON_RADIUS = 0.9;                     // screen heights
const vec2 MOON_OFFSET = vec2(-0.3, 0.05);         // from the right edge / bottom
const float MOON_MAX = 0.22;                       // brightness cap

vec2 craterField(vec2 p) {
  vec2 i = floor(p);
  vec2 f = fract(p);
  vec2 acc = vec2(0.0);
  for (int y = -1; y <= 1; y++) {
    for (int x = -1; x <= 1; x++) {
      vec2 g = vec2(float(x), float(y));
      vec2 o = hash22(i + g);
      float rad = 0.25 + 0.3 * hash12(i + g + 17.0);
      vec2 v = g + o - f;                          // pixel → crater center
      float len = length(v);
      float t = len / rad;
      if (t < 1.2) {
        vec2 dir = v / max(len, 1e-4);
        float bowl = smoothstep(0.0, 0.85, t) * (1.0 - smoothstep(0.85, 1.0, t));
        float rimOut = smoothstep(0.85, 1.0, t) * (1.0 - smoothstep(1.0, 1.2, t));
        acc += dir * (bowl * 0.8 - rimOut * 0.6);
      }
    }
  }
  return acc;
}

vec3 moon(vec2 s, float aspect, out float cover) {
  float rise = 1.0 - uIntro;                       // below the viewport before the intro
  float lift = min(uScroll * 0.08, 0.15);
  vec2 c = vec2(aspect + MOON_OFFSET.x, MOON_OFFSET.y - rise + lift);
  vec2 d = (s - c) / MOON_RADIUS;
  float r2 = dot(d, d);
  float px = 1.0 / (uResolution.y * MOON_RADIUS);
  cover = 1.0 - smoothstep(1.0 - 3.0 * px, 1.0, r2);
  if (cover <= 0.0) return vec3(0.0);

  vec3 nrm = vec3(d, sqrt(max(1.0 - r2, 0.0)));
  vec2 tc = nrm.xy / (1.0 + nrm.z) * 2.5;
  float maria = smoothstep(0.45, 0.75, fbm(tc * 1.3 + 3.1));
  vec2 cr = craterField(tc * 6.0) * 0.35 + craterField(tc * 14.0) * 0.15;
  vec3 nn = normalize(nrm + vec3(cr, 0.0));

  float a = 0.9 + sin(uDrift * 6.2832 / 90.0) * 0.5 + uPointer.x * 0.15;
  vec3 sun = normalize(vec3(cos(a), sin(a) * 0.6 + 0.3, 0.55));
  float albedo = mix(0.92, 0.55, maria) * (0.9 + 0.2 * vnoise(tc * 30.0));
  float term = smoothstep(-0.05, 0.25, dot(nrm, sun));
  float diff = max(dot(nn, sun), 0.0);
  float earthshine = 0.025 * (1.0 - term);
  float rim = pow(1.0 - nrm.z, 3.0) * 0.25 * term;
  float lum = min(albedo * diff * term + earthshine + rim, 1.0);

  float pres = presenceN();
  float breathe = 1.0 + 0.04 * sin(uDrift * 0.5) * pres;
  float dim = mix(0.7, 1.0, pres) * (1.0 - 0.4 * uFocus) * breathe;
  vec3 tint = mix(vec3(0.86, 0.86, 0.88), MOONLIGHT, term);
  return tint * lum * MOON_MAX * dim;
}

// Gravitational lens: sample toward the pointer so stars appear pushed out.
vec2 lens(vec2 s, float aspect) {
  vec2 ptr = vec2((uPointer.x * 0.5 + 0.5) * aspect, uPointer.y * 0.5 + 0.5);
  vec2 dv = s - ptr;
  float d = length(dv);
  float R = 0.15;
  float k = 0.012 * R / (d + R) * (1.0 - smoothstep(R, 2.0 * R, d)) * presenceN();
  return s - dv / max(d, 1e-4) * k;
}

// Primary pulses: an expanding ring that displaces stars and glows faintly.
vec2 ripple(vec2 s, float aspect, out float glow) {
  glow = 0.0;
  vec2 off = vec2(0.0);
  for (int i = 0; i < 4; i++) {
    vec4 pu = uPulse[i];
    float age = uTime - pu.z;
    if (pu.w < 0.5 || pu.w > 1.5 || age < 0.0 || age > 0.6) continue;
    vec2 dv = s - vec2(pu.x * aspect, pu.y);
    float d = length(dv);
    float x = (d - age * 0.5) / 0.02;
    float band = exp(-x * x) * (1.0 - age / 0.6);
    off += dv / max(d, 1e-4) * band * 0.006;
    glow += band * 0.03;
  }
  return off;
}

// Route pulses: stars stretch radially for 450ms, then relax.
float warpAmount() {
  float w = 0.0;
  for (int i = 0; i < 4; i++) {
    vec4 pu = uPulse[i];
    float age = uTime - pu.z;
    if (pu.w > 1.5 && age >= 0.0 && age <= 0.45) w = max(w, sin(3.14159 * age / 0.45));
  }
  return w;
}

float starLayer(vec2 s, float scale, float depth, float seed, float chance, vec2 center, float warp) {
  vec2 par = uPointer * depth * 0.012 + vec2(0.0, uScroll * depth * 0.05);
  vec2 p = (s + par) * scale;
  vec2 i = floor(p);
  vec2 f = fract(p);
  float h = hash12(i + seed);
  if (h > chance) return 0.0;
  vec2 pos = hash22(i + seed * 1.7) * 0.8 + 0.1;
  vec2 dv = (f - pos) * (uResolution.y / scale);   // buffer px
  vec2 radial = normalize(s - center + 1e-5);
  float dr = dot(dv, radial);
  float dt = dot(dv, vec2(-radial.y, radial.x));
  float d2 = dr * dr / (1.0 + warp * 40.0) + dt * dt;
  float rad = (0.6 + 1.2 * hash12(i + seed + 3.3)) * uResolution.y / 1200.0 + 0.4;
  float tw = 0.55 + 0.45 * sin(uDrift * (0.6 + h * 4.0) + h * 40.0);
  return exp(-d2 / (rad * rad)) * tw;
}

vec3 nebula(vec2 s) {
  float a = smoothstep(0.45, 0.85, fbm(s * 1.6 + vec2(uDrift * 0.004, uDrift * 0.002)));
  float b = smoothstep(0.55, 0.9, fbm(s * 2.3 + 7.0 - uDrift * 0.003));
  return ICE * a * 0.04 + MOONLIGHT * b * 0.03;
}

void main() {
  vec2 fc = gl_FragCoord.xy;
  vec2 uv = fc / uResolution;
  float aspect = uResolution.x / uResolution.y;
  vec2 s = vec2(uv.x * aspect, uv.y);
  float pres = presenceN();

  float rippleGlow;
  vec2 ss = lens(s, aspect) - ripple(s, aspect, rippleGlow);
  float warp = warpAmount();
  vec2 center = vec2(aspect * 0.5, 0.5);
  float stars = starLayer(ss, 6.0, 1.0, 11.0, 0.25, center, warp)
              + starLayer(ss, 10.0, 0.5, 23.0, 0.30, center, warp) * 0.7
              + starLayer(ss, 16.0, 0.25, 37.0, 0.35, center, warp) * 0.45;

  vec3 col = nebula(s) + vec3(stars * (1.0 + warp * 0.6) * 0.85 + rippleGlow);
  col *= smoothstep(0.0, 0.3, uIntro) * (1.0 - 0.3 * uFocus) * mix(0.75, 1.0, pres);

  float cover;
  vec3 m = moon(s, aspect, cover);
  col = mix(col, m, cover);                        // the moon occludes the stars

  float lit = step(0.002, max(col.r, max(col.g, col.b)));
  outColor = vec4(col + ditherNoise(fc) * lit, 1.0);
}
`;

export const lunarScene: AmbientScene = {
  id: "lunar",
  fragment: FRAGMENT_PRELUDE + LUNAR_BODY,
  introMs: 1200,
  pointerTauMs: 150,
};
