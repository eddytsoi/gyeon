import { fail, redirect } from '@sveltejs/kit';
import { loginCustomer, requestPasswordReset, ApiError } from '$lib/api';
import * as m from '$lib/paraglide/messages';
import type { Actions } from './$types';

export const actions: Actions = {
  login: async ({ request, cookies }) => {
    const form = await request.formData();
    const email = form.get('email')?.toString() ?? '';
    const password = form.get('password')?.toString() ?? '';

    if (!email || !password) return fail(400, { error: m.account_login_error_required(), email });

    try {
      const { token, expires_in } = await loginCustomer(email, password);
      cookies.set('customer_token', token, {
        path: '/',
        httpOnly: true,
        sameSite: 'lax',
        maxAge: expires_in ?? 60 * 60 * 24 * 30
      });
    } catch (e) {
      // `legacy` marks a real account that has no password yet (WooCommerce
      // import) so the page can show the "old customer — reset password"
      // guidance. Unknown emails and wrong passwords leave it unset.
      const legacy = e instanceof ApiError && e.code === 'password_not_set';
      return fail(401, { error: m.account_login_error_invalid(), email, legacy });
    }

    throw redirect(303, '/account/orders');
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
