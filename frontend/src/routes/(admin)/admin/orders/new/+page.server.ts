import { adminCreateOrder } from '$lib/api/admin';
import { fail, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ parent }) => {
  const { token } = await parent();
  if (!token) throw redirect(303, '/admin/login');
  return {};
};

export const actions: Actions = {
  default: async ({ request, cookies }) => {
    const token = cookies.get('admin_token');
    if (!token) throw redirect(303, '/admin/login');

    const fd = await request.formData();
    const raw = fd.get('body');
    if (typeof raw !== 'string' || raw === '') {
      return fail(400, { error: 'invalid request body' });
    }

    let body: Record<string, unknown>;
    try {
      body = JSON.parse(raw);
    } catch {
      return fail(400, { error: 'invalid JSON' });
    }

    let createdId: string;
    try {
      const order = await adminCreateOrder(token, body);
      createdId = order.id;
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'failed to create order';
      return fail(400, { error: msg });
    }

    // Redirect to the new order's detail page so the admin sees the
    // canonical totals + can mark it paid / add an internal note next.
    throw redirect(303, `/admin/orders/${createdId}`);
  }
};
