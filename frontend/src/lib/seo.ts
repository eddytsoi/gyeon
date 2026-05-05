import type { PublicSetting } from '$lib/api';

/** Returns the public storefront origin for canonical / OG / sitemap URLs. */
export function siteOrigin(settings: PublicSetting[] | null | undefined, fallback = 'http://localhost:5173'): string {
  const v = settings?.find((s) => s.key === 'public_base_url')?.value?.trim();
  return (v || fallback).replace(/\/+$/, '');
}

/** Strip basic markdown / HTML and trim to a meta-description-length string. */
export function snippet(input: string | null | undefined, maxLen = 160): string {
  if (!input) return '';
  const plain = input
    .replace(/<[^>]+>/g, ' ')
    .replace(/!?\[([^\]]*)\]\([^)]*\)/g, '$1')
    .replace(/[#*_`>]/g, ' ')
    .replace(/\s+/g, ' ')
    .trim();
  if (plain.length <= maxLen) return plain;
  return plain.slice(0, maxLen - 1).replace(/\s+\S*$/, '') + '…';
}
