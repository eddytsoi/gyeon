import { getMyOrders } from '$lib/api';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ parent }) => {
  const { token } = await parent();
  const orders = token ? await getMyOrders(token, 5, 0).catch(() => []) : [];
  return { orders };
};
