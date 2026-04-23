import { fail, redirect } from '@sveltejs/kit';
import { loginCustomer } from '$lib/api';
import type { Actions } from './$types';

export const actions: Actions = {
  default: async ({ request, cookies }) => {
    const form = await request.formData();
    const email = form.get('email')?.toString() ?? '';
    const password = form.get('password')?.toString() ?? '';

    if (!email || !password) return fail(400, { error: 'Email and password are required' });

    try {
      const { token } = await loginCustomer(email, password);
      cookies.set('customer_token', token, {
        path: '/',
        httpOnly: true,
        sameSite: 'lax',
        maxAge: 60 * 60 * 24 * 30
      });
    } catch {
      return fail(401, { error: 'Invalid email or password' });
    }

    throw redirect(303, '/account');
  }
};
