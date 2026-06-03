import { fail, redirect } from '@sveltejs/kit';
import type { PageServerLoad, Actions } from './$types';
import {
  adminGetCustomer,
  adminGetOrders,
  adminSendResetPasswordEmail,
  adminUpdateCustomerRole
} from '$lib/api/admin';
import type { CustomerRole } from '$lib/types';

export const load: PageServerLoad = async ({ parent, params }) => {
  const { token } = await parent();
  if (!token) throw redirect(303, '/admin/login');

  const [customer, ordersRes] = await Promise.all([
    adminGetCustomer(token, params.id).catch(() => null),
    adminGetOrders(token, { limit: 200, offset: 0 }).catch(() => ({ items: [], total: 0 }))
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
  },
  updateRole: async ({ params, cookies, request }) => {
    const token = cookies.get('admin_token');
    if (!token) throw redirect(303, '/admin/login');
    const form = await request.formData();
    const role = String(form.get('role') ?? '') as CustomerRole;
    if (role !== 'customer' && role !== 'installer' && role !== 'installer_v2') {
      return fail(400, { roleError: 'invalid role' });
    }
    try {
      const updated = await adminUpdateCustomerRole(token, params.id, role);
      return { roleSaved: true, role: updated.role };
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Update failed';
      return fail(502, { roleError: message });
    }
  }
};
