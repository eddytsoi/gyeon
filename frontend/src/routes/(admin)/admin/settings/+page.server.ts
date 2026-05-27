import { fail, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';
import { adminGetSettings, adminBulkUpdateSettings, adminGetMedia, adminGetPages } from '$lib/api/admin';

export const load: PageServerLoad = async ({ parent }) => {
  const { token } = await parent();
  if (!token) throw redirect(303, '/admin/login');

  // categories was used by the now-removed Hidden Categories picker
  // (migration 103 moved it to /admin/products/category-roles). Dropped
  // from the parallel load to keep the settings page snappy.
  const [settings, mediaFiles, pagesRes] = await Promise.all([
    adminGetSettings(token).catch(() => []),
    adminGetMedia(token).catch(() => []),
    adminGetPages(token, 100, 0).catch(() => ({ items: [], total: 0 }))
  ]);
  const pages = pagesRes.items.filter((p) => p.is_published);
  return { settings, mediaFiles, pages };
};

export const actions: Actions = {
  save: async ({ request, cookies }) => {
    const token = cookies.get('admin_token');
    if (!token) return fail(401, { error: 'Unauthorized' });

    const form = await request.formData();
    const updates: Record<string, string> = {};
    for (const [key, value] of form.entries()) {
      if (typeof value === 'string') updates[key] = value;
    }

    try {
      await adminBulkUpdateSettings(token, updates);
    } catch {
      return fail(400, { error: 'Failed to save settings' });
    }
    return { success: true };
  }
};
