import type { RequestHandler } from './$types';
import { getPublicSettings } from '$lib/api';
import { siteOrigin } from '$lib/seo';

export const GET: RequestHandler = async ({ setHeaders }) => {
  const settings = await getPublicSettings().catch(() => []);
  const origin = siteOrigin(settings);
  const maintenance = settings?.find((s) => s.key === 'maintenance_mode')?.value === 'true';

  const lines = [
    'User-agent: *',
    maintenance ? 'Disallow: /' : 'Disallow: /admin',
    maintenance ? '' : 'Disallow: /account',
    maintenance ? '' : 'Disallow: /cart',
    maintenance ? '' : 'Disallow: /checkout',
    '',
    `Sitemap: ${origin}/sitemap.xml`
  ].filter((l) => l !== undefined);

  setHeaders({
    'Content-Type': 'text/plain; charset=utf-8',
    'Cache-Control': 'public, max-age=3600'
  });
  return new Response(lines.join('\n') + '\n');
};
