import { getMyAddresses } from '$lib/api';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ parent }) => {
  const { token } = await parent();
  const addresses = token ? await getMyAddresses(token).catch(() => []) : [];
  return { addresses };
};
