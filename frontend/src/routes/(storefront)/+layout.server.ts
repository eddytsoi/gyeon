import { getMyProfile, getNavMenu, getPublicSettings } from '$lib/api';
import type { NavItem, NavMenu, SocialMediaEntry } from '$lib/types';
import type { LayoutServerLoad } from './$types';

function parseSocials(raw: string | undefined): SocialMediaEntry[] {
  if (!raw) return [];
  try {
    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [];
    return parsed
      .filter(
        (e): e is SocialMediaEntry =>
          !!e && typeof e === 'object' && typeof e.icon === 'string' && typeof e.url === 'string'
      )
      .map((e) => ({
        icon: e.icon,
        url: e.url,
        label: typeof e.label === 'string' ? e.label : undefined,
        customSvgPath: typeof e.customSvgPath === 'string' ? e.customSvgPath : undefined
      }));
  } catch {
    return [];
  }
}

export const load: LayoutServerLoad = async ({ cookies }) => {
  // Read first so both nav fetches can be filtered by the customer's
  // role. Anonymous visitors pass token=null and the backend treats
  // them as "customer".
  const token = cookies.get('customer_token') ?? null;
  const [headerNav, footerNav, settings] = await Promise.all([
    getNavMenu('header', token).catch(() => null as NavMenu | null),
    getNavMenu('footer', token).catch(() => null as NavMenu | null),
    getPublicSettings().catch(() => [])
  ]);
  const mcpEnabled = settings.find((s) => s.key === 'mcp_enabled')?.value === 'true';
  // Default to true so uninitialised installs (no setting row yet) keep the blog visible.
  const blogEnabled = settings.find((s) => s.key === 'blog_enabled')?.value !== 'false';
  // Default ON: an uninitialised install (no row, or fetch failure) keeps PWA active.
  const pwaEnabled = settings.find((s) => s.key === 'pwa_enabled')?.value !== 'false';
  const socials = parseSocials(settings.find((s) => s.key === 'social_media')?.value);

  const stripBlogLinks = (menu: NavMenu | null): NavMenu | null => {
    if (!menu || blogEnabled) return menu;
    const filter = (items: NavItem[]): NavItem[] =>
      items
        .filter((i) => !i.url?.startsWith('/blog'))
        .map((i) => ({ ...i, children: i.children ? filter(i.children) : i.children }));
    return { ...menu, items: filter(menu.items) };
  };

  const customer = token ? await getMyProfile(token).catch(() => null) : null;

  return {
    headerNav: stripBlogLinks(headerNav),
    footerNav: stripBlogLinks(footerNav),
    mcpEnabled,
    blogEnabled,
    pwaEnabled,
    customer,
    publicSettings: settings,
    socials
  };
};
