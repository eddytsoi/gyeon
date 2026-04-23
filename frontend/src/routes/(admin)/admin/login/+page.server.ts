import { fail, redirect } from '@sveltejs/kit';
import { adminLogin } from '$lib/api/admin';
import type { Actions } from './$types';

export const actions: Actions = {
  default: async ({ request, cookies }) => {
    const form = await request.formData();
    const email = form.get('email')?.toString() ?? '';
    const password = form.get('password')?.toString() ?? '';

    try {
      const token = await adminLogin(email, password);
      cookies.set('admin_token', token, {
        path: '/',
        httpOnly: true,
        sameSite: 'lax',
        maxAge: 60 * 60 * 24
      });
    } catch {
      return fail(401, { error: 'Invalid email or password' });
    }

    throw redirect(303, '/admin/dashboard');
  }
};
