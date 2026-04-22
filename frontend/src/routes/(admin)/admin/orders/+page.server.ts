import { adminGetOrders } from '$lib/api/admin';
import { redirect } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ parent }) => {
  const { token } = await parent();
  if (!token) throw redirect(303, '/admin/login');
  const orders = await adminGetOrders(token).catch(() => []).then(r => r ?? []);
  return { orders };
};
