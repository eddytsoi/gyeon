import { getNavMenu, getPublicSettings } from '$lib/api';
import type { NavMenu } from '$lib/types';
import type { LayoutServerLoad } from './$types';

export const load: LayoutServerLoad = async () => {
  const headerNav = await getNavMenu('header').catch(() => null as NavMenu | null);
  const settings = await getPublicSettings().catch(() => []);
  const mcpEnabled = settings.find((s) => s.key === 'mcp_enabled')?.value === 'true';
  return { headerNav, mcpEnabled };
};
