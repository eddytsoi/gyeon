import { getStats } from '$lib/api/admin';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ parent }) => {
  const { token } = await parent();
  const stats = token ? await getStats(token).catch(() => null) : null;
  return { stats };
};
