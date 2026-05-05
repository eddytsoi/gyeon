import { adminGetSettings, adminBulkUpdateSettings, type Setting } from '$lib/api/admin';
import { fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

const KEYS = [
  'shipany_enabled',
  'shipany_user_id',
  'shipany_api_key',
  'shipany_region',
  'shipany_webhook_secret',
  'shipping_countries'
] as const;

export const load: PageServerLoad = async ({ cookies }) => {
  const token = cookies.get('admin_token') ?? '';
  const all = await adminGetSettings(token).catch(() => [] as Setting[]);
  const settings = all.filter((s) => (KEYS as readonly string[]).includes(s.key));
  return { settings, token };
};

export const actions: Actions = {
  save: async ({ request, cookies }) => {
    const token = cookies.get('admin_token') ?? '';
    const data = await request.formData();

    const enabled = data.get('shipany_enabled') === 'true';
    const userID = String(data.get('shipany_user_id') ?? '').trim();
    const apiKey = String(data.get('shipany_api_key') ?? '').trim();
    const region = String(data.get('shipany_region') ?? '').trim();
    const webhookSecret = String(data.get('shipany_webhook_secret') ?? '').trim();

    // shipping_countries is sent as a JSON array string
    const countriesRaw = String(data.get('shipping_countries') ?? '[]');
    let countries: string[] = [];
    try {
      const parsed = JSON.parse(countriesRaw);
      if (Array.isArray(parsed)) countries = parsed.filter((v) => typeof v === 'string');
    } catch {
      return fail(400, { error: 'Invalid shipping_countries payload' });
    }

    const updates: Record<string, string> = {
      shipany_enabled: enabled ? 'true' : 'false',
      shipany_user_id: userID,
      shipany_api_key: apiKey,
      shipany_region: region,
      shipany_webhook_secret: webhookSecret,
      shipping_countries: JSON.stringify(countries)
    };

    try {
      await adminBulkUpdateSettings(token, updates);
    } catch {
      return fail(500, { error: 'Failed to save shipping settings' });
    }
    return { success: true };
  }
};
