import { adminGetOrder, adminGetShipment, adminUpdateOrderStatus, adminCreateShipment, adminRequestShipanyPickup, adminUpdateOrderShippingAddress, adminGetSettings, adminListShipanyCouriers, adminListOrderNotices, adminCreateOrderNotice, adminMarkOrderNoticesRead, adminIssueRefund } from '$lib/api/admin';
import { fail, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';
import { resolveAdminId } from '$lib/admin/resolveId';

const resolve = (token: string, id: string) =>
  resolveAdminId(token, 'ORD', id, '/admin/orders');

export const load: PageServerLoad = async ({ parent, params }) => {
  const { token } = await parent();
  if (!token) throw redirect(303, '/admin/login');
  const id = await resolve(token, params.id);
  const [order, shipment, settings, couriers, notices] = await Promise.all([
    adminGetOrder(token, id),
    adminGetShipment(token, id).catch(() => null),
    adminGetSettings(token).catch(() => []),
    adminListShipanyCouriers(token).catch(() => []),
    adminListOrderNotices(token, id).catch(() => [])
  ]);
  // Fire-and-forget: viewing the page clears the customer-message unread badge.
  adminMarkOrderNoticesRead(token, id).catch(() => {});
  const defaultCarrier = settings.find((s) => s.key === 'shipany_default_courier')?.value ?? '';
  const defaultService = settings.find((s) => s.key === 'shipany_default_service')?.value ?? '';
  return { order, shipment, defaultCarrier, defaultService, couriers, notices };
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
  },

  createShipment: async ({ request, cookies, params }) => {
    const token = cookies.get('admin_token');
    if (!token) return fail(401, { error: 'Unauthorized' });
    const id = await resolve(token, params.id);

    const form = await request.formData();
    const carrier = form.get('carrier')?.toString() || '';
    const service = form.get('service')?.toString() || '';
    const override = carrier && service ? { carrier, service } : undefined;

    try {
      await adminCreateShipment(token, id, override);
    } catch (e: unknown) {
      return fail(400, { error: e instanceof Error ? e.message : 'Failed to create shipment' });
    }
    return { success: true };
  },

  refund: async ({ request, cookies, params }) => {
    const token = cookies.get('admin_token');
    if (!token) return fail(401, { error: 'Unauthorized' });
    const id = await resolve(token, params.id);

    const form = await request.formData();
    const amount = parseFloat(form.get('amount')?.toString() ?? '0');
    const reason = form.get('reason')?.toString() ?? '';
    const amountCents = Math.round(amount * 100);

    // Per-line restock selection: the modal serialises {order_item_id, quantity}
    // pairs (qty > 0) into a hidden `restock` JSON field. Always send an array
    // (possibly empty = restock nothing) so the backend uses the explicit path
    // rather than legacy auto-restock. A malformed value falls back to "none".
    let restockItems: { order_item_id: string; quantity: number }[] = [];
    try {
      const raw = form.get('restock')?.toString();
      if (raw) {
        const parsed = JSON.parse(raw);
        if (Array.isArray(parsed)) {
          restockItems = parsed
            .map((it) => ({ order_item_id: String(it.order_item_id), quantity: Number(it.quantity) }))
            .filter((it) => it.order_item_id && Number.isFinite(it.quantity) && it.quantity > 0);
        }
      }
    } catch {
      restockItems = [];
    }

    try {
      await adminIssueRefund(token, id, amountCents, reason, restockItems);
    } catch (e: unknown) {
      return fail(400, { error: e instanceof Error ? e.message : 'Failed to issue refund' });
    }
    return { success: true };
  },

  requestPickup: async ({ cookies, params }) => {
    const token = cookies.get('admin_token');
    if (!token) return fail(401, { error: 'Unauthorized' });
    const id = await resolve(token, params.id);

    try {
      await adminRequestShipanyPickup(token, id);
    } catch (e: unknown) {
      return fail(400, { error: e instanceof Error ? e.message : 'Failed to request pickup' });
    }
    return { success: true };
  },

  updateShippingAddress: async ({ request, cookies, params }) => {
    const token = cookies.get('admin_token');
    if (!token) return fail(401, { error: 'Unauthorized' });
    const id = await resolve(token, params.id);

    const form = await request.formData();
    const str = (k: string) => form.get(k)?.toString().trim() ?? '';
    const address = {
      first_name: str('first_name'),
      last_name: str('last_name'),
      phone: str('phone') || undefined,
      line1: str('line1'),
      line2: str('line2') || undefined,
      city: str('city'),
      state: str('state') || undefined,
      postal_code: str('postal_code'),
      country: str('country') || 'HK'
    };
    if (!address.line1) return fail(400, { error: '請填寫地址' });

    try {
      await adminUpdateOrderShippingAddress(token, id, address);
    } catch (e: unknown) {
      return fail(400, { error: e instanceof Error ? e.message : 'Failed to update shipping address' });
    }
    return { success: true };
  },

  addInternalNote: async ({ request, cookies, params }) => {
    const token = cookies.get('admin_token');
    if (!token) return fail(401, { error: 'Unauthorized' });
    const id = await resolve(token, params.id);

    const form = await request.formData();
    const body = form.get('body')?.toString().trim() || '';
    if (!body) return fail(400, { error: 'Note body is required' });

    try {
      await adminCreateOrderNotice(token, id, 'system', body);
    } catch (e: unknown) {
      return fail(400, { error: e instanceof Error ? e.message : 'Failed to add note' });
    }
    return { success: true };
  },

  sendAdminMessage: async ({ request, cookies, params }) => {
    const token = cookies.get('admin_token');
    if (!token) return fail(401, { error: 'Unauthorized' });
    const id = await resolve(token, params.id);

    const form = await request.formData();
    const body = form.get('body')?.toString().trim() || '';
    if (!body) return fail(400, { error: 'Message body is required' });

    try {
      await adminCreateOrderNotice(token, id, 'admin', body);
    } catch (e: unknown) {
      return fail(400, { error: e instanceof Error ? e.message : 'Failed to send message' });
    }
    return { success: true };
  }
};
