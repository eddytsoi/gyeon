import { getMyProfile, getNavMenu, getPublicSettings } from '$lib/api';
import type { NavMenu } from '$lib/types';
import type { LayoutServerLoad } from './$types';

export const load: LayoutServerLoad = async ({ cookies }) => {
  const [headerNav, footerNav, settings] = await Promise.all([
    getNavMenu('header').catch(() => null as NavMenu | null),
    getNavMenu('footer').catch(() => null as NavMenu | null),
    getPublicSettings().catch(() => [])
  ]);
  const mcpEnabled = settings.find((s) => s.key === 'mcp_enabled')?.value === 'true';

  const token = cookies.get('customer_token') ?? null;
  const customer = token ? await getMyProfile(token).catch(() => null) : null;

  return { headerNav, footerNav, mcpEnabled, customer };
};
