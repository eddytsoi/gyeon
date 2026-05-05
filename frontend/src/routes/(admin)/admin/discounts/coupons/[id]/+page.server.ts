import {
  adminGetCoupon,
  adminCreateCoupon,
  adminUpdateCoupon,
  type Coupon,
  type CouponInput
} from '$lib/api/admin';
import { fail, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ params, cookies }) => {
  const token = cookies.get('admin_token') ?? '';
  if (params.id === 'new') {
    return { coupon: null as Coupon | null };
  }
  const coupon = await adminGetCoupon(token, params.id).catch(() => null);
  return { coupon };
};

function parseOptionalNumber(value: FormDataEntryValue | null): number | null {
  if (value == null) return null;
  const s = String(value).trim();
  if (!s) return null;
  const n = Number(s);
  return Number.isFinite(n) ? n : null;
}

function parseOptionalInt(value: FormDataEntryValue | null): number | null {
  if (value == null) return null;
  const s = String(value).trim();
  if (!s) return null;
  const n = parseInt(s, 10);
  return Number.isFinite(n) ? n : null;
}

function parseOptionalDate(value: FormDataEntryValue | null): string | null {
  if (value == null) return null;
  const s = String(value).trim();
  if (!s) return null;
  return new Date(s).toISOString();
}

export const actions: Actions = {
  save: async ({ request, params, cookies }) => {
    const token = cookies.get('admin_token') ?? '';
    const data = await request.formData();

    const body: CouponInput = {
      code: String(data.get('code') ?? '').trim().toUpperCase(),
      description: String(data.get('description') ?? '').trim() || undefined,
      discount_type: data.get('discount_type') as 'percentage' | 'fixed',
      discount_value: Number(data.get('discount_value') ?? 0),
      min_order_amount: parseOptionalNumber(data.get('min_order_amount')),
      max_uses: parseOptionalInt(data.get('max_uses')),
      starts_at: parseOptionalDate(data.get('starts_at')),
      ends_at: parseOptionalDate(data.get('ends_at')),
      is_active: data.get('is_active') === 'true'
    };

    if (!body.code) return fail(400, { error: 'Code is required' });
    if (body.discount_value <= 0) return fail(400, { error: 'Discount value must be positive' });

    try {
      if (params.id === 'new') {
        await adminCreateCoupon(token, body);
      } else {
        await adminUpdateCoupon(token, params.id, body);
      }
    } catch {
      return fail(500, { error: 'Failed to save coupon' });
    }
    redirect(303, '/admin/discounts');
  }
};
