import { error, fail, redirect } from '@sveltejs/kit';
import { getMyOrderByID, getMyOrderNotices, createMyOrderNotice, markMyOrderNoticesRead } from '$lib/api';
import { resolveCustomerOrderId } from '$lib/storefront/resolveOrderId';
import type { Actions, PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ parent, params, cookies }) => {
  await parent();
  const token = cookies.get('customer_token') ?? '';
  if (!token) throw redirect(303, '/account/login');
  const id = await resolveCustomerOrderId(token, params.id);
  const [order, notices] = await Promise.all([
    getMyOrderByID(token, id).catch(() => null),
    getMyOrderNotices(token, id).catch(() => [])
  ]);
  if (!order) throw error(404, 'Order not found');
  // Fire-and-forget: clear the unread badge on view.
  markMyOrderNoticesRead(token, id).catch(() => {});
  return { order, notices };
};

export const actions: Actions = {
  sendMessage: async ({ request, cookies, params }) => {
    const token = cookies.get('customer_token');
    if (!token) throw redirect(303, '/account/login');
    const id = await resolveCustomerOrderId(token, params.id);

    const form = await request.formData();
    const body = form.get('body')?.toString().trim() || '';
    if (!body) return fail(400, { error: 'Message body is required' });

    try {
      await createMyOrderNotice(token, id, body);
    } catch (e: unknown) {
      return fail(400, { error: e instanceof Error ? e.message : 'Failed to send message' });
    }
    return { success: true };
  }
};
