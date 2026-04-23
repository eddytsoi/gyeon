import { error } from '@sveltejs/kit';
import { getOrderByID } from '$lib/api';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ url }) => {
  const orderID = url.searchParams.get('order');
  if (!orderID) throw error(404, 'Order not found');

  try {
    const order = await getOrderByID(orderID);
    return { order };
  } catch {
    throw error(404, 'Order not found');
  }
};
