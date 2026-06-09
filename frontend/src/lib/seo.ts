import type { PublicSetting } from '$lib/api';

/** Returns the public storefront origin for canonical / OG / sitemap URLs. */
export function siteOrigin(settings: PublicSetting[] | null | undefined, fallback = 'http://localhost:5173'): string {
  const v = settings?.find((s) => s.key === 'public_base_url')?.value?.trim();
  return (v || fallback).replace(/\/+$/, '');
}

/** Returns the configured site name (admin 網站名稱) for title/brand use. */
export function siteName(settings: PublicSetting[] | null | undefined, fallback = 'GYEON'): string {
  const v = settings?.find((s) => s.key === 'site_name')?.value?.trim();
  return v || fallback;
}

/**
 * Returns the site-wide default SEO description (admin 網站描述). Used as the
 * fallback meta description on pages that set no description of their own.
 */
export function siteDescription(settings: PublicSetting[] | null | undefined, fallback = ''): string {
  const v = settings?.find((s) => s.key === 'site_description')?.value?.trim();
  return v || fallback;
}

/** Strip basic markdown / HTML and trim to a meta-description-length string. */
export function snippet(input: string | null | undefined, maxLen = 160): string {
  if (!input) return '';
  const plain = input
    .replace(/<[^>]+>/g, ' ')
    .replace(/!?\[([^\]]*)\]\([^)]*\)/g, '$1')
    .replace(/\[\/?[a-z0-9-]+[^\]]*\]/gi, ' ')
    .replace(/[#*_`>]/g, ' ')
    .replace(/\s+/g, ' ')
    .trim();
  if (plain.length <= maxLen) return plain;
  return plain.slice(0, maxLen - 1).replace(/\s+\S*$/, '') + '…';
}
