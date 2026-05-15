import { fail, redirect } from '@sveltejs/kit';
import type { PageServerLoad, Actions } from './$types';
import { adminGetCustomer, adminGetOrders, adminSendResetPasswordEmail } from '$lib/api/admin';

export const load: PageServerLoad = async ({ parent, params }) => {
  const { token } = await parent();
  if (!token) throw redirect(303, '/admin/login');

  const [customer, ordersRes] = await Promise.all([
    adminGetCustomer(token, params.id).catch(() => null),
    adminGetOrders(token, 200, 0).catch(() => ({ items: [], total: 0 }))
  ]);

  const orders = ordersRes.items.filter(o => o.customer_id === params.id);

  return { customer, orders };
};

export const actions: Actions = {
  sendResetPassword: async ({ params, cookies }) => {
    const token = cookies.get('admin_token');
    if (!token) throw redirect(303, '/admin/login');
    try {
      await adminSendResetPasswordEmail(token, params.id);
      return { resetSent: true };
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Send failed';
      return fail(502, { resetError: message });
    }
  }
};
