import { adminListForms, adminDeleteForm, type AdminForm } from '$lib/api/admin';
import { fail, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ cookies }) => {
  const token = cookies.get('admin_token') ?? '';
  const forms = await adminListForms(token).catch(() => [] as AdminForm[]);
  return { forms };
};

export const actions: Actions = {
  delete: async ({ request, cookies }) => {
    const token = cookies.get('admin_token') ?? '';
    const data = await request.formData();
    const id = data.get('id') as string;
    try {
      await adminDeleteForm(token, id);
    } catch {
      return fail(500, { error: 'Failed to delete form' });
    }
    redirect(303, '/admin/forms');
  }
};
