import {
  adminListRedirects,
  adminDeleteRedirect,
  type Redirect
} from '$lib/api/admin';
import { fail, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ cookies }) => {
  const token = cookies.get('admin_token') ?? '';
  const items = await adminListRedirects(token).catch(() => [] as Redirect[]);
  return { items };
};

export const actions: Actions = {
  delete: async ({ request, cookies }) => {
    const token = cookies.get('admin_token') ?? '';
    const id = (await request.formData()).get('id') as string;
    try {
      await adminDeleteRedirect(token, id);
    } catch {
      return fail(500, { error: 'Failed to delete redirect' });
    }
    redirect(303, '/admin/redirects');
  }
};
