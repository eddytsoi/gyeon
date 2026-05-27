import {
  adminGetCampaign,
  adminCreateCampaign,
  adminUpdateCampaign,
  adminGetCategories,
  adminGetProducts,
  type Campaign,
  type CampaignInput
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

function parseTargetIDs(values: FormDataEntryValue[]): string[] {
  const out: string[] = [];
  const seen = new Set<string>();
  for (const v of values) {
    const s = String(v).trim();
    if (s && !seen.has(s)) {
      seen.add(s);
      out.push(s);
    }
  }
  return out;
}

export const load: PageServerLoad = async ({ params, cookies }) => {
  const token = cookies.get('admin_token') ?? '';
  const [categories, productsRes] = await Promise.all([
    adminGetCategories(token).catch(() => []),
    adminGetProducts(token, 200, 0, '').catch(() => ({ items: [], total: 0 }))
  ]);
  const products = productsRes.items;
  if (params.id === 'new') {
    return { campaign: null as Campaign | null, categories, products };
  }
  const campaign = await adminGetCampaign(token, params.id).catch(() => null);
  return { campaign, categories, products };
};

function parseOptionalNumber(value: FormDataEntryValue | null): number | null {
  if (value == null) return null;
  const s = String(value).trim();
  if (!s) return null;
  const n = Number(s);
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

    const targetType = data.get('target_type') as 'all' | 'category' | 'product';
    const targetIDs = targetType === 'all' ? [] : parseTargetIDs(data.getAll('target_ids'));

    const allowedRoles = parseAllowedRoles(data.getAll('allowed_roles'));
    const allowGuests = data.get('allow_guests') === 'true';

    const body: CampaignInput = {
      name: String(data.get('name') ?? '').trim(),
      description: String(data.get('description') ?? '').trim() || undefined,
      discount_type: data.get('discount_type') as 'percentage' | 'fixed',
      discount_value: Number(data.get('discount_value') ?? 0),
      target_type: targetType,
      target_ids: targetIDs,
      min_order_amount: parseOptionalNumber(data.get('min_order_amount')),
      max_order_amount: parseOptionalNumber(data.get('max_order_amount')),
      allowed_roles: allowedRoles,
      allow_guests: allowGuests,
      starts_at: parseOptionalDate(data.get('starts_at')),
      ends_at: parseOptionalDate(data.get('ends_at')),
      is_active: data.get('is_active') === 'true'
    };

    if (!body.name) return fail(400, { error: 'Name is required' });
    if (body.discount_value <= 0) return fail(400, { error: 'Discount value must be positive' });
    if (body.target_type !== 'all' && body.target_ids.length === 0) {
      return fail(400, { error: 'Select at least one target when scope is not "all"' });
    }
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
        await adminCreateCampaign(token, body);
      } else {
        await adminUpdateCampaign(token, params.id, body);
      }
    } catch {
      return fail(500, { error: 'Failed to save campaign' });
    }
    redirect(303, '/admin/discounts');
  }
};
