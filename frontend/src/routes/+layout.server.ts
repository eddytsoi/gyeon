import { getPublicSettings } from '$lib/api';
import type { LayoutServerLoad } from './$types';

// Favicon is a site-wide concern, so the root layout is the single source of
// truth for the <link rel="icon"> tag. Emitting it here (rather than a static
// link in app.html) guarantees exactly one icon link reaches the head — the
// custom favicon when set, otherwise the bundled /icon.svg fallback. A static
// SVG icon in app.html would otherwise win over a raster PNG favicon in Chrome
// regardless of order, shadowing the admin-configured favicon.
export const load: LayoutServerLoad = async () => {
  const settings = await getPublicSettings().catch(() => []);
  return { faviconUrl: settings.find((s) => s.key === 'favicon_url')?.value ?? '' };
};
