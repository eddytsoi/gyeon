import { redirect } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';
import { adminGetCustomer, adminGetOrders } from '$lib/api/admin';

export const load: PageServerLoad = async ({ parent, params }) => {
  const { token } = await parent();
  if (!token) throw redirect(303, '/admin/login');

  const [customer, allOrders] = await Promise.all([
    adminGetCustomer(token, params.id).catch(() => null),
    adminGetOrders(token, 200, 0).catch(() => [])
  ]);

  const orders = allOrders.filter(o => o.customer_id === params.id);

  return { customer, orders };
};
