import {
  adminGetEmailTemplate,
  adminUpsertEmailTemplate,
  adminResetEmailTemplate,
  type EmailTemplateDetail
} from '$lib/api/admin';
import { fail, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ params, cookies }) => {
  const token = cookies.get('admin_token') ?? '';
  const detail = await adminGetEmailTemplate(token, params.key).catch(
    () => null as EmailTemplateDetail | null
  );
  if (!detail) throw redirect(303, '/admin/email-templates');
  return { detail, token };
};

export const actions: Actions = {
  save: async ({ request, params, cookies }) => {
    const token = cookies.get('admin_token') ?? '';
    const data = await request.formData();
    const subject = String(data.get('subject') ?? '').trim();
    const html = String(data.get('html') ?? '');
    const text = String(data.get('text') ?? '');
    const isEnabled = data.get('is_enabled') === 'true';
    if (!subject) return fail(400, { error: 'Subject is required' });
    if (!html) return fail(400, { error: 'HTML body is required' });
    try {
      await adminUpsertEmailTemplate(token, params.key, { subject, html, text, is_enabled: isEnabled });
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Failed to save';
      return fail(500, { error: msg });
    }
    return { success: true };
  },
  reset: async ({ params, cookies }) => {
    const token = cookies.get('admin_token') ?? '';
    try {
      await adminResetEmailTemplate(token, params.key);
    } catch {
      return fail(500, { error: 'Failed to reset' });
    }
    redirect(303, `/admin/email-templates/${params.key}`);
  }
};
