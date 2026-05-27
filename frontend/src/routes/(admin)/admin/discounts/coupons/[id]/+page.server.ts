import {
  adminGetCoupon,
  adminCreateCoupon,
  adminUpdateCoupon,
  type Coupon,
  type CouponInput
} from '$lib/api/admin';
import type { CustomerRole } from '$lib/types';
import { fail, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

function parseAllowedRoles(values: FormDataEntryValue[]): CustomerRole[] {
  const valid: CustomerRole[] = [];
  const seen = new Set<string>();
  for (const v of values) {
    const s = String(v).trim();
    if ((s === 'customer' || s === 'installer') && !seen.has(s)) {
      seen.add(s);
      valid.push(s as CustomerRole);
    }
  }
  return valid;
}

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

    const allowedRoles = parseAllowedRoles(data.getAll('allowed_roles'));
    const allowGuests = data.get('allow_guests') === 'true';

    const body: CouponInput = {
      code: String(data.get('code') ?? '').trim().toUpperCase(),
      description: String(data.get('description') ?? '').trim() || undefined,
      discount_type: data.get('discount_type') as 'percentage' | 'fixed',
      discount_value: Number(data.get('discount_value') ?? 0),
      min_order_amount: parseOptionalNumber(data.get('min_order_amount')),
      max_order_amount: parseOptionalNumber(data.get('max_order_amount')),
      max_uses: parseOptionalInt(data.get('max_uses')),
      allowed_roles: allowedRoles,
      allow_guests: allowGuests,
      starts_at: parseOptionalDate(data.get('starts_at')),
      ends_at: parseOptionalDate(data.get('ends_at')),
      is_active: data.get('is_active') === 'true'
    };

    if (!body.code) return fail(400, { error: 'Code is required' });
    if (body.discount_value <= 0) return fail(400, { error: 'Discount value must be positive' });
    if (body.allowed_roles.length === 0 && !body.allow_guests) {
      return fail(400, { error: 'Select at least one eligible account type' });
    }
    if (
      body.min_order_amount != null &&
      body.max_order_amount != null &&
      body.max_order_amount < body.min_order_amount
    ) {
      return fail(400, { error: 'Maximum order amount must be greater than or equal to minimum' });
    }

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
