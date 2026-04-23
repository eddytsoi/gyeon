import { fail, redirect } from '@sveltejs/kit';
import { registerCustomer } from '$lib/api';
import type { Actions } from './$types';

export const actions: Actions = {
  default: async ({ request, cookies }) => {
    const form = await request.formData();
    const email = form.get('email')?.toString() ?? '';
    const password = form.get('password')?.toString() ?? '';
    const firstName = form.get('first_name')?.toString() ?? '';
    const lastName = form.get('last_name')?.toString() ?? '';
    const phone = form.get('phone')?.toString() || undefined;

    if (!email || !password || !firstName || !lastName) {
      return fail(400, { error: 'All required fields must be filled in' });
    }
    if (password.length < 8) {
      return fail(400, { error: 'Password must be at least 8 characters' });
    }

    try {
      const { token } = await registerCustomer(email, password, firstName, lastName, phone);
      cookies.set('customer_token', token, {
        path: '/',
        httpOnly: true,
        sameSite: 'lax',
        maxAge: 60 * 60 * 24 * 30
      });
    } catch (e) {
      const msg = e instanceof Error ? e.message : '';
      if (msg.includes('409')) return fail(409, { error: 'An account with this email already exists' });
      return fail(500, { error: 'Registration failed. Please try again.' });
    }

    throw redirect(303, '/account');
  }
};
