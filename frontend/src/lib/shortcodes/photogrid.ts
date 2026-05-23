// Helpers for the [photo-grid] shortcode and its renderer component.
// Pure-TS so the validation logic can be reasoned about (and unit-tested
// later) without a Svelte runtime — same split as banner.ts / video.ts.

export type PhotoGridBleed = 'full' | 'container';

const COL_MIN = 1;
const COL_MAX = 12;

// CSS length tokens we accept for `gap`: `0`, `8px`, `1.5rem`, `0.5em`, `2%`.
// Restricting to a single token keeps the value safe to embed in a `style=""`
// CSS custom property without escaping.
const GAP_RE = /^(?:0|\d+(?:\.\d+)?(?:px|rem|em|%))$/;

export function resolveBleed(v: unknown): PhotoGridBleed {
  return v === 'full' || v === 'container' ? v : 'full';
}

// Optional responsive override applied at the Tailwind `lg` breakpoint
// (≥ 1024px). Returns undefined when unset/empty/invalid so the consumer
// can fall back to plain `bleed`.
export function resolveBleedLg(v: unknown): PhotoGridBleed | undefined {
  if (v === undefined || v === null || v === '') return undefined;
  return v === 'full' || v === 'container' ? v : undefined;
}

// Base `col`: integer in [1, 12]; falls back to `fallback` (default 2) for
// missing/invalid values so the grid always renders something sensible.
export function resolveCol(v: unknown, fallback = 2): number {
  if (v === undefined || v === null || v === '') return fallback;
  const n = Number(v);
  if (!Number.isInteger(n) || n < COL_MIN || n > COL_MAX) return fallback;
  return n;
}

// Breakpoint override: undefined when unset/empty/invalid so the consumer
// can inherit from the base `col`.
export function resolveColBreakpoint(v: unknown): number | undefined {
  if (v === undefined || v === null || v === '') return undefined;
  const n = Number(v);
  if (!Number.isInteger(n) || n < COL_MIN || n > COL_MAX) return undefined;
  return n;
}

export function resolveGap(v: unknown, fallback = '8px'): string {
  if (typeof v !== 'string' || v === '') return fallback;
  const t = v.trim();
  return GAP_RE.test(t) ? t : fallback;
}

export function resolveGapBreakpoint(v: unknown): string | undefined {
  if (typeof v !== 'string' || v === '') return undefined;
  const t = v.trim();
  return GAP_RE.test(t) ? t : undefined;
}

// Split "a.jpg, b.jpg, c.jpg" → ["a.jpg","b.jpg","c.jpg"]. Trims tokens and
// drops empties so trailing commas / extra whitespace are forgiving — same
// behavior as [products ids="..."].
export function parseSourceList(v: unknown): string[] {
  if (typeof v !== 'string') return [];
  return v.split(',').map((s) => s.trim()).filter(Boolean);
}

// Resolve a single `source` token to a /uploads/... URL. Tokens that already
// look like a path or URL pass through verbatim (so authors can paste output
// from the admin Media picker). Bare strings are treated as
// media_files.original_name and looked up in the server-resolved refs map;
// returns '' when there's no match so the caller can omit the slot.
export function resolveSourceToken(
  token: string,
  mediaByName: Record<string, string>
): string {
  if (!token) return '';
  if (token.startsWith('/') || /^https?:\/\//i.test(token)) return token;
  return mediaByName[token] ?? '';
}
