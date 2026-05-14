// Helpers for the [banner] shortcode and the <Banner> Svelte component.
// Lives outside the .svelte files so the validation logic can be reasoned
// about (and unit-tested later) without a Svelte runtime — same split as
// section.ts.

export type BannerBleed = 'full' | 'container';
export type BannerAspect = number | 'auto';
export type BannerHeight = number | 'auto';

export function resolveBleed(v: unknown): BannerBleed {
  return v === 'full' || v === 'container' ? v : 'full';
}

// Optional responsive override applied at the Tailwind `lg` breakpoint
// (≥ 1024px). Returns undefined when unset/empty/invalid so the consumer
// can fall back to plain `bleed`.
export function resolveBleedLg(v: unknown): BannerBleed | undefined {
  if (v === undefined || v === null || v === '') return undefined;
  return v === 'full' || v === 'container' ? v : undefined;
}

// Positive finite number → number; 'auto' / undefined / empty / invalid → 'auto'.
export function resolveAspectRatio(v: unknown): BannerAspect {
  if (v === undefined || v === null || v === '' || v === 'auto') return 'auto';
  const n = Number(v);
  return Number.isFinite(n) && n > 0 ? n : 'auto';
}

// Number in [1, 2000] (px) → number; 'auto' / undefined / empty / invalid → 'auto'.
export function resolveHeight(v: unknown): BannerHeight {
  if (v === undefined || v === null || v === '' || v === 'auto') return 'auto';
  const n = Number(v);
  return Number.isFinite(n) && n >= 1 && n <= 2000 ? n : 'auto';
}
