import { adminGetSettings, adminBulkUpdateSettings } from '$lib/api/admin';
import { fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

const KEYS = ['recaptcha_enabled', 'recaptcha_site_key', 'recaptcha_secret_key', 'recaptcha_min_score'];

export const load: PageServerLoad = async ({ cookies }) => {
  const token = cookies.get('admin_token') ?? '';
  const all = await adminGetSettings(token).catch(() => []);
  const map: Record<string, string> = {};
  for (const s of all) {
    if (KEYS.includes(s.key)) map[s.key] = s.value;
  }
  return {
    values: {
      recaptcha_enabled: map.recaptcha_enabled ?? 'false',
      recaptcha_site_key: map.recaptcha_site_key ?? '',
      recaptcha_secret_key: map.recaptcha_secret_key ?? '',
      recaptcha_min_score: map.recaptcha_min_score ?? '0.5'
    }
  };
};

export const actions: Actions = {
  save: async ({ request, cookies }) => {
    const token = cookies.get('admin_token') ?? '';
    const data = await request.formData();
    const updates: Record<string, string> = {
      recaptcha_enabled: data.get('recaptcha_enabled') === 'true' ? 'true' : 'false',
      recaptcha_site_key: ((data.get('recaptcha_site_key') as string) || '').trim(),
      recaptcha_secret_key: ((data.get('recaptcha_secret_key') as string) || '').trim(),
      recaptcha_min_score: ((data.get('recaptcha_min_score') as string) || '0.5').trim()
    };
    try {
      await adminBulkUpdateSettings(token, updates);
    } catch {
      return fail(500, { error: 'Save failed' });
    }
    return { ok: true };
  }
};
