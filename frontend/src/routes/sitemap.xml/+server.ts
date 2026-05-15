import type { RequestHandler } from './$types';
import { getPublicSettings, getProducts, getCategories, getBlogPosts, getCmsPageBySlug } from '$lib/api';
import { siteOrigin } from '$lib/seo';

interface Url {
  loc: string;
  lastmod?: string;
  changefreq?: 'daily' | 'weekly' | 'monthly';
  priority?: string;
}

function escapeXml(s: string): string {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&apos;');
}

export const GET: RequestHandler = async ({ setHeaders }) => {
  const settings = await getPublicSettings().catch(() => []);
  const origin = siteOrigin(settings);
  const blogEnabled = settings.find((s) => s.key === 'blog_enabled')?.value !== 'false';

  const [products, categories, posts] = await Promise.all([
    getProducts(500, 0).catch(() => []),
    getCategories().catch(() => []),
    blogEnabled ? getBlogPosts(500, 0).catch(() => []) : Promise.resolve([])
  ]);

  const urls: Url[] = [
    { loc: `${origin}/`, changefreq: 'weekly', priority: '1.0' },
    { loc: `${origin}/products`, changefreq: 'daily', priority: '0.9' }
  ];
  if (blogEnabled) {
    urls.push({ loc: `${origin}/blog`, changefreq: 'weekly', priority: '0.7' });
  }

  for (const c of categories) {
    if (c.is_active) {
      urls.push({
        loc: `${origin}/products/category/${c.slug}`,
        changefreq: 'weekly',
        priority: '0.7'
      });
    }
  }

  for (const p of products) {
    if (p.status === 'active') {
      urls.push({
        loc: `${origin}/products/${p.slug}`,
        lastmod: p.updated_at?.slice(0, 10),
        changefreq: 'weekly',
        priority: '0.8'
      });
    }
  }

  for (const post of posts) {
    if (post.is_published) {
      urls.push({
        loc: `${origin}/blog/${post.slug}`,
        lastmod: post.updated_at?.slice(0, 10),
        changefreq: 'monthly',
        priority: '0.6'
      });
    }
  }

  // Best-effort: include 'about' / 'contact' if those CMS pages exist.
  for (const slug of ['about', 'contact', 'privacy', 'terms']) {
    const p = await getCmsPageBySlug(slug).catch(() => null);
    if (p && p.is_published) {
      urls.push({
        loc: `${origin}/${slug}`,
        lastmod: p.updated_at?.slice(0, 10),
        changefreq: 'monthly',
        priority: '0.4'
      });
    }
  }

  const body =
    '<?xml version="1.0" encoding="UTF-8"?>\n' +
    '<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">\n' +
    urls
      .map((u) => {
        const lines = [`  <url>`, `    <loc>${escapeXml(u.loc)}</loc>`];
        if (u.lastmod) lines.push(`    <lastmod>${u.lastmod}</lastmod>`);
        if (u.changefreq) lines.push(`    <changefreq>${u.changefreq}</changefreq>`);
        if (u.priority) lines.push(`    <priority>${u.priority}</priority>`);
        lines.push(`  </url>`);
        return lines.join('\n');
      })
      .join('\n') +
    '\n</urlset>\n';

  setHeaders({
    'Content-Type': 'application/xml; charset=utf-8',
    'Cache-Control': 'public, max-age=3600'
  });
  return new Response(body);
};
