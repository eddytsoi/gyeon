import { error } from '@sveltejs/kit';
import { getOrderByID } from '$lib/api';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ parent, params }) => {
  await parent();
  const order = await getOrderByID(params.id).catch(() => null);
  if (!order) throw error(404, 'Order not found');
  return { order };
};
