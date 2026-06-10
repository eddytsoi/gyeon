import type { RequestHandler } from './$types';
import {
  getPublicSettings,
  getProducts,
  getCategories,
  getBlogPosts,
  getBlogCategories,
  getPublishedCmsPages
} from '$lib/api';
import { siteOrigin } from '$lib/seo';

interface Url {
  loc: string;
  lastmod?: string;
  changefreq?: 'daily' | 'weekly' | 'monthly';
  priority?: string;
}

// Backend public list endpoints clamp limit to 100; page through until a
// short page signals the end. MAX_URLS is a per-content-type safety cap.
const PAGE = 100;
const MAX_URLS = 5000;

async function fetchAll<T>(fn: (limit: number, offset: number) => Promise<T[]>): Promise<T[]> {
  const out: T[] = [];
  for (let offset = 0; out.length < MAX_URLS; offset += PAGE) {
    const batch = await fn(PAGE, offset).catch(() => []);
    out.push(...batch);
    if (batch.length < PAGE) break;
  }
  return out;
}

// Slugs owned by static SvelteKit routes — never emit them as CMS `/{slug}`
// pages (would duplicate / shadow the real route).
const RESERVED_SLUGS = new Set([
  'products',
  'blog',
  'cart',
  'checkout',
  'account',
  'pages',
  'wishlist'
]);

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
  const homepageId = settings.find((s) => s.key === 'homepage_page_id')?.value || '';

  const [products, categories, posts, blogCats, pages] = await Promise.all([
    fetchAll((l, o) => getProducts(l, o)),
    getCategories().catch(() => []),
    blogEnabled ? fetchAll((l, o) => getBlogPosts(l, o)) : Promise.resolve([]),
    blogEnabled ? getBlogCategories().catch(() => []) : Promise.resolve([]),
    getPublishedCmsPages().catch(() => [])
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

  for (const c of blogCats) {
    urls.push({
      loc: `${origin}/blog/category/${c.slug}`,
      changefreq: 'weekly',
      priority: '0.5'
    });
  }

  // All published CMS pages render at root `/{slug}`. Skip the homepage page
  // (already listed as `/`) and any slug owned by a static route.
  for (const p of pages) {
    if (p.id === homepageId) continue;
    if (RESERVED_SLUGS.has(p.slug)) continue;
    urls.push({
      loc: `${origin}/${p.slug}`,
      lastmod: p.updated_at?.slice(0, 10),
      changefreq: 'monthly',
      priority: '0.4'
    });
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
