import {
  adminGetCampaign,
  adminCreateCampaign,
  adminUpdateCampaign,
  adminGetCategories,
  adminGetProducts,
  type Campaign,
  type CampaignInput
} from '$lib/api/admin';
import { fail, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ params, cookies }) => {
  const token = cookies.get('admin_token') ?? '';
  const [categories, products] = await Promise.all([
    adminGetCategories(token).catch(() => []),
    adminGetProducts(token, 200, 0, '').catch(() => [])
  ]);
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
    const targetIDValue = data.get('target_id') ? String(data.get('target_id')).trim() : '';
    const targetID = targetType === 'all' ? null : (targetIDValue || null);

    const body: CampaignInput = {
      name: String(data.get('name') ?? '').trim(),
      description: String(data.get('description') ?? '').trim() || undefined,
      discount_type: data.get('discount_type') as 'percentage' | 'fixed',
      discount_value: Number(data.get('discount_value') ?? 0),
      target_type: targetType,
      target_id: targetID,
      min_order_amount: parseOptionalNumber(data.get('min_order_amount')),
      starts_at: parseOptionalDate(data.get('starts_at')),
      ends_at: parseOptionalDate(data.get('ends_at')),
      is_active: data.get('is_active') === 'true'
    };

    if (!body.name) return fail(400, { error: 'Name is required' });
    if (body.discount_value <= 0) return fail(400, { error: 'Discount value must be positive' });
    if (body.target_type !== 'all' && !body.target_id) {
      return fail(400, { error: 'Target is required when scope is not "all"' });
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
