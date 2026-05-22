// Helpers for the [video] shortcode and its renderer component. Pure-TS so
// the validation logic can be reasoned about (and unit-tested later) without
// a Svelte runtime — same split as banner.ts / section.ts.

export type VideoBleed = 'full' | 'container';
export type VideoAspect = number | 'auto';
export type VideoHeight = number | 'auto';

export function resolveBleed(v: unknown): VideoBleed {
  return v === 'full' || v === 'container' ? v : 'full';
}

// Optional responsive override applied at the Tailwind `lg` breakpoint
// (≥ 1024px). Returns undefined when unset/empty/invalid so the consumer
// can fall back to plain `bleed`.
export function resolveBleedLg(v: unknown): VideoBleed | undefined {
  if (v === undefined || v === null || v === '') return undefined;
  return v === 'full' || v === 'container' ? v : undefined;
}

// Positive finite number → number; 'auto' / undefined / empty / invalid → 'auto'.
export function resolveAspectRatio(v: unknown): VideoAspect {
  if (v === undefined || v === null || v === '' || v === 'auto') return 'auto';
  const n = Number(v);
  return Number.isFinite(n) && n > 0 ? n : 'auto';
}

// Responsive override variant: returns undefined when unset/empty so the
// consumer can inherit from `aspect-ratio`. Honors an explicit 'auto'.
export function resolveAspectRatioBreakpoint(v: unknown): VideoAspect | undefined {
  if (v === undefined || v === null || v === '') return undefined;
  if (v === 'auto') return 'auto';
  const n = Number(v);
  return Number.isFinite(n) && n > 0 ? n : undefined;
}

// Number in [1, 2000] (px) → number; 'auto' / undefined / empty / invalid → 'auto'.
export function resolveHeight(v: unknown): VideoHeight {
  if (v === undefined || v === null || v === '' || v === 'auto') return 'auto';
  const n = Number(v);
  return Number.isFinite(n) && n >= 1 && n <= 2000 ? n : 'auto';
}

// 'false' (case-insensitive) → false; anything else (including missing) → true.
export function resolveAutoplay(v: unknown): boolean {
  if (typeof v !== 'string') return true;
  return v.trim().toLowerCase() !== 'false';
}
