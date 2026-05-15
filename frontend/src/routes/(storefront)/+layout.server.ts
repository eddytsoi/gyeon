import { getMyProfile, getNavMenu, getPublicSettings } from '$lib/api';
import type { NavItem, NavMenu } from '$lib/types';
import type { LayoutServerLoad } from './$types';

export const load: LayoutServerLoad = async ({ cookies }) => {
  const [headerNav, footerNav, settings] = await Promise.all([
    getNavMenu('header').catch(() => null as NavMenu | null),
    getNavMenu('footer').catch(() => null as NavMenu | null),
    getPublicSettings().catch(() => [])
  ]);
  const mcpEnabled = settings.find((s) => s.key === 'mcp_enabled')?.value === 'true';
  // Default to true so uninitialised installs (no setting row yet) keep the blog visible.
  const blogEnabled = settings.find((s) => s.key === 'blog_enabled')?.value !== 'false';

  const stripBlogLinks = (menu: NavMenu | null): NavMenu | null => {
    if (!menu || blogEnabled) return menu;
    const filter = (items: NavItem[]): NavItem[] =>
      items
        .filter((i) => !i.url?.startsWith('/blog'))
        .map((i) => ({ ...i, children: i.children ? filter(i.children) : i.children }));
    return { ...menu, items: filter(menu.items) };
  };

  const token = cookies.get('customer_token') ?? null;
  const customer = token ? await getMyProfile(token).catch(() => null) : null;

  return {
    headerNav: stripBlogLinks(headerNav),
    footerNav: stripBlogLinks(footerNav),
    mcpEnabled,
    blogEnabled,
    customer,
    publicSettings: settings
  };
};
