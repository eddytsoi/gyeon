// Helpers for the [section] shortcode and the <Section> Svelte component.
// Lives outside the .svelte files so the regex + whitelist logic can be unit
// tested without a Svelte runtime.

export type SectionBg = 'paper' | 'cream' | 'white' | 'ink-900' | 'navy-900';
export type SectionLayout = 'default' | 'split' | 'split-reverse' | 'hero';
export type SectionPadding = 'sm' | 'md' | 'lg';
export type SectionWidth = 'default' | 'narrow' | 'full';
export type SectionAlign = 'left' | 'center';
export type SectionBleed = 'full' | 'container';

// Static maps so Tailwind JIT sees full class names. Unknown values fall
// back to defaults via the resolvers below.
export const SECTION_BG: Record<SectionBg, string> = {
  paper: 'bg-paper',
  cream: 'bg-cream',
  white: 'bg-white',
  'ink-900': 'bg-ink-900',
  'navy-900': 'bg-navy-900'
};

export const SECTION_PADDING: Record<SectionPadding, string> = {
  sm: 'py-8 md:py-12',
  md: 'py-12 md:py-20 lg:py-24',
  lg: 'py-16 md:py-24'
};

export const SECTION_WIDTH: Record<SectionWidth, string> = {
  default: 'max-w-7xl mx-auto px-4 sm:px-6 lg:px-8',
  narrow: 'max-w-4xl mx-auto px-4 sm:px-6 lg:px-8',
  full: ''
};

export function resolveBg(v: unknown): SectionBg {
  return v === 'paper' || v === 'cream' || v === 'white' || v === 'ink-900' || v === 'navy-900'
    ? v
    : 'paper';
}
export function resolveLayout(v: unknown): SectionLayout {
  return v === 'default' || v === 'split' || v === 'split-reverse' || v === 'hero' ? v : 'default';
}
export function resolvePadding(v: unknown): SectionPadding {
  return v === 'sm' || v === 'md' || v === 'lg' ? v : 'md';
}
export function resolveWidth(v: unknown): SectionWidth {
  return v === 'default' || v === 'narrow' || v === 'full' ? v : 'default';
}
export function resolveAlign(v: unknown): SectionAlign {
  return v === 'left' || v === 'center' ? v : 'left';
}
export function resolveBleed(v: unknown): SectionBleed {
  return v === 'full' || v === 'container' ? v : 'full';
}

// Optional responsive override applied at the Tailwind `lg` breakpoint
// (≥ 1024px). Returns undefined when unset/empty/invalid so the consumer
// can fall back to plain `bleed`.
export function resolveBleedLg(v: unknown): SectionBleed | undefined {
  if (v === undefined || v === null || v === '') return undefined;
  return v === 'full' || v === 'container' ? v : undefined;
}

// Split body on a markdown horizontal rule (`---` on its own line). Returns
// [first, second]. If no HR is found the second half is empty — the wrapping
// layout decides what to do with that.
export function splitBodyOnHr(body: string): [string, string] {
  const re = /(^|\n)[ \t]*-{3,}[ \t]*(?=\n|$)/;
  const m = re.exec(body);
  if (!m || m.index === undefined) return [body, ''];
  const start = m.index + m[1].length;
  const end = m.index + m[0].length;
  return [body.slice(0, start).replace(/\n+$/, ''), body.slice(end).replace(/^\n+/, '')];
}
