import { error } from '@sveltejs/kit';
import { getOrderByID } from '$lib/api';
import { resolveCustomerOrderId } from '$lib/storefront/resolveOrderId';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ parent, params, cookies }) => {
  await parent();
  const token = cookies.get('customer_token') ?? '';
  const id = await resolveCustomerOrderId(token, params.id);
  const order = await getOrderByID(id).catch(() => null);
  if (!order) throw error(404, 'Order not found');
  return { order };
};
