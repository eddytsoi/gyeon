import {
  adminGetForm,
  adminListFormSubmissions,
  adminDeleteFormSubmission,
  type AdminForm,
  type FormSubmissionRow
} from '$lib/api/admin';
import { error, fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ params, cookies, url }) => {
  const token = cookies.get('admin_token') ?? '';
  const limit = Number(url.searchParams.get('limit')) || 50;
  const offset = Number(url.searchParams.get('offset')) || 0;

  const [form, page] = await Promise.all([
    adminGetForm(token, params.id).catch(() => null as AdminForm | null),
    adminListFormSubmissions(token, params.id, limit, offset).catch(
      () => ({ items: [] as FormSubmissionRow[], total: 0 })
    )
  ]);
  if (!form) throw error(404, 'Form not found');
  return { form, submissions: page, limit, offset };
};

export const actions: Actions = {
  delete: async ({ request, cookies }) => {
    const token = cookies.get('admin_token') ?? '';
    const data = await request.formData();
    const sid = data.get('sid') as string;
    try {
      await adminDeleteFormSubmission(token, sid);
    } catch {
      return fail(500, { error: 'Failed to delete submission' });
    }
    return { ok: true };
  }
};
