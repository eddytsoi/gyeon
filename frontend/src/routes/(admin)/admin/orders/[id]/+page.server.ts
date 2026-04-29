import { adminGetOrder, adminUpdateOrderStatus } from '$lib/api/admin';
import { fail, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';
import { resolveAdminId } from '$lib/admin/resolveId';

const resolve = (token: string, id: string) =>
  resolveAdminId(token, 'ORD', id, '/admin/orders');

export const load: PageServerLoad = async ({ parent, params }) => {
  const { token } = await parent();
  if (!token) throw redirect(303, '/admin/login');
  const id = await resolve(token, params.id);
  const order = await adminGetOrder(token, id);
  return { order };
};

export const actions: Actions = {
  updateStatus: async ({ request, cookies, params }) => {
    const token = cookies.get('admin_token');
    if (!token) return fail(401, { error: 'Unauthorized' });
    const id = await resolve(token, params.id);

    const form = await request.formData();
    const status = form.get('status')?.toString() ?? '';
    const note = form.get('note')?.toString() || undefined;

    try {
      await adminUpdateOrderStatus(token, id, status, note);
    } catch (e: unknown) {
      return fail(400, { error: e instanceof Error ? e.message : 'Failed to update status' });
    }
    return { success: true };
  }
};
