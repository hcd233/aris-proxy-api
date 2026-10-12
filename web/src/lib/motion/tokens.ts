import type { SpringParams } from "./spring";

/* JS-side motion tokens (spec §5.1 / §6.1). CSS-side durations and easings
 * live in src/styles/motion-*.css; --ease-orbit there is generated from
 * LUNAR_SPRING (spring.test.ts keeps them in sync). */

/** Atelier: critically damped (ζ≈0.99), never overshoots. */
export const ATELIER_SPRING: SpringParams = { stiffness: 260, damping: 32, mass: 1 };

/** Lunar: underdamped (ζ≈0.69), ~5% overshoot. */
export const LUNAR_SPRING: SpringParams = { stiffness: 170, damping: 18, mass: 1 };

/** Lunar magnetic buttons: ζ≈0.58, ~10% overshoot. */
export const MAGNETIC_SPRING: SpringParams = { stiffness: 300, damping: 20, mass: 1 };
