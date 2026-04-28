import { fail, redirect } from '@sveltejs/kit';
import { loginCustomer, requestPasswordReset } from '$lib/api';
import type { Actions } from './$types';

export const actions: Actions = {
  login: async ({ request, cookies }) => {
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
  },

  forgotPassword: async ({ request }) => {
    const form = await request.formData();
    const email = form.get('email')?.toString().trim() ?? '';

    if (!email) return fail(400, { forgot: { error: '請輸入電郵地址。' } });

    const res = await requestPasswordReset(email);
    if (!res.ok && res.status !== 204) {
      return fail(502, { forgot: { error: '寄送失敗，請稍後再試。' } });
    }
    return { forgot: { sent: true, email } };
  }
};
