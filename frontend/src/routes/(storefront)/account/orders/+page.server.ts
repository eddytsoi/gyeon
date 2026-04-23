import { getMyOrders } from '$lib/api';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ parent, url }) => {
  const { token } = await parent();
  const offset = Number(url.searchParams.get('offset') ?? 0);
  const orders = token ? await getMyOrders(token, 20, offset).catch(() => []) : [];
  return { orders, offset };
};
