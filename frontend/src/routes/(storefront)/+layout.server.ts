import { getNavMenu } from '$lib/api';
import type { NavMenu } from '$lib/types';
import type { LayoutServerLoad } from './$types';

export const load: LayoutServerLoad = async () => {
  const headerNav = await getNavMenu('header').catch(() => null as NavMenu | null);
  return { headerNav };
};
