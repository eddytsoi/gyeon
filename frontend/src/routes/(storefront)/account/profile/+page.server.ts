import { fail } from '@sveltejs/kit';
import { updateMyProfile, changeMyPassword, getMyLoyaltyBalance, ApiError } from '$lib/api';
import * as m from '$lib/paraglide/messages';
import type { Actions, PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ parent, cookies }) => {
  await parent(); // ensures auth check runs and customer is available
  const token = cookies.get('customer_token') ?? '';
  // Best-effort: if loyalty is disabled the endpoint still returns 0.
  const loyalty = token
    ? await getMyLoyaltyBalance(token).catch(() => ({ points: 0 }))
    : { points: 0 };
  return { loyalty };
};

export const actions: Actions = {
  profile: async ({ request, cookies }) => {
    const token = cookies.get('customer_token') ?? '';
    const form = await request.formData();
    const firstName = form.get('first_name')?.toString() ?? '';
    const lastName = form.get('last_name')?.toString() ?? '';
    const phone = form.get('phone')?.toString() || undefined;

    if (!firstName || !lastName) return fail(400, { profileError: 'First and last name are required' });

    try {
      const customer = await updateMyProfile(token, { first_name: firstName, last_name: lastName, phone });
      return { profileSuccess: true, customer };
    } catch {
      return fail(500, { profileError: 'Failed to update profile. Please try again.' });
    }
  },

  password: async ({ request, cookies }) => {
    const token = cookies.get('customer_token') ?? '';
    const form = await request.formData();
    const current = form.get('current_password')?.toString() ?? '';
    const next = form.get('new_password')?.toString() ?? '';
    const confirm = form.get('confirm')?.toString() ?? '';

    if (!current || !next) return fail(400, { passwordError: m.account_password_error_generic() });
    if (next.length < 8) return fail(400, { passwordError: m.account_password_error_short() });
    if (next !== confirm) return fail(400, { passwordError: m.account_password_error_mismatch() });

    try {
      const { token: fresh, expires_in } = await changeMyPassword(token, {
        current_password: current,
        new_password: next
      });
      // Other devices are signed out by the token_version bump; keep THIS
      // session alive by swapping in the fresh token (same opts as login).
      cookies.set('customer_token', fresh, {
        path: '/',
        httpOnly: true,
        sameSite: 'lax',
        maxAge: expires_in ?? 60 * 60 * 24 * 30
      });
      return { passwordSuccess: true };
    } catch (e) {
      if (e instanceof ApiError && e.status === 401)
        return fail(401, { passwordError: m.account_password_error_current() });
      if (e instanceof ApiError && e.status === 422)
        return fail(422, { passwordError: m.account_password_error_short() });
      return fail(500, { passwordError: m.account_password_error_generic() });
    }
  }
};
