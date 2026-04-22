import { adminGetOrder, adminUpdateOrderStatus } from '$lib/api/admin';
import { fail, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ parent, params }) => {
  const { token } = await parent();
  if (!token) throw redirect(303, '/admin/login');
  const order = await adminGetOrder(token, params.id);
  return { order };
};

export const actions: Actions = {
  updateStatus: async ({ request, cookies, params }) => {
    const token = cookies.get('admin_token');
    if (!token) return fail(401, { error: 'Unauthorized' });

    const form = await request.formData();
    const status = form.get('status')?.toString() ?? '';
    const note = form.get('note')?.toString() || undefined;

    try {
      await adminUpdateOrderStatus(token, params.id, status, note);
    } catch (e: unknown) {
      return fail(400, { error: e instanceof Error ? e.message : 'Failed to update status' });
    }
    return { success: true };
  }
};
