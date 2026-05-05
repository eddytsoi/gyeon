import { adminGetSettings, adminBulkUpdateSettings, type Setting } from '$lib/api/admin';
import { fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

const TAX_KEYS = ['tax_enabled', 'tax_rate', 'tax_label', 'tax_inclusive'] as const;

export const load: PageServerLoad = async ({ cookies }) => {
  const token = cookies.get('admin_token') ?? '';
  const all = await adminGetSettings(token).catch(() => [] as Setting[]);
  const settings = all.filter((s) => (TAX_KEYS as readonly string[]).includes(s.key));
  return { settings };
};

export const actions: Actions = {
  save: async ({ request, cookies }) => {
    const token = cookies.get('admin_token') ?? '';
    const data = await request.formData();

    const enabled = data.get('tax_enabled') === 'true';
    const inclusive = data.get('tax_inclusive') === 'true';
    const label = String(data.get('tax_label') ?? '').trim() || 'Sales Tax';

    // The form sends rate as a percentage (5 = 5%); we store the decimal (0.05).
    // Old UI stored decimal directly, but this page presents a friendlier %.
    const ratePctRaw = String(data.get('tax_rate_pct') ?? '0').trim();
    const ratePct = Number(ratePctRaw);
    if (!Number.isFinite(ratePct) || ratePct < 0 || ratePct > 100) {
      return fail(400, { error: 'Rate must be between 0 and 100' });
    }
    const rate = (ratePct / 100).toFixed(6).replace(/\.?0+$/, '') || '0';

    const updates: Record<string, string> = {
      tax_enabled: enabled ? 'true' : 'false',
      tax_inclusive: inclusive ? 'true' : 'false',
      tax_rate: rate,
      tax_label: label
    };

    try {
      await adminBulkUpdateSettings(token, updates);
    } catch {
      return fail(500, { error: 'Failed to save tax settings' });
    }
    return { success: true };
  }
};
