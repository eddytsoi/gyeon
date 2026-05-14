import { fail, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';
import { adminGetSettings, adminBulkUpdateSettings, adminGetMedia, adminGetCategories, adminGetPages } from '$lib/api/admin';

export const load: PageServerLoad = async ({ parent }) => {
  const { token } = await parent();
  if (!token) throw redirect(303, '/admin/login');

  const [settings, mediaFiles, categories, allPages] = await Promise.all([
    adminGetSettings(token).catch(() => []),
    adminGetMedia(token).catch(() => []),
    adminGetCategories(token).catch(() => []),
    adminGetPages(token).catch(() => [])
  ]);
  const pages = allPages.filter((p) => p.is_published);
  return { settings, mediaFiles, categories, pages };
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
