import { fail } from '@sveltejs/kit';
import { updateMyProfile } from '$lib/api';
import type { Actions, PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ parent }) => {
  await parent(); // ensures auth check runs and customer is available
  return {};
};

export const actions: Actions = {
  default: async ({ request, cookies, locals }) => {
    const token = cookies.get('customer_token') ?? '';
    const form = await request.formData();
    const firstName = form.get('first_name')?.toString() ?? '';
    const lastName = form.get('last_name')?.toString() ?? '';
    const phone = form.get('phone')?.toString() || undefined;

    if (!firstName || !lastName) return fail(400, { error: 'First and last name are required' });

    try {
      const customer = await updateMyProfile(token, { first_name: firstName, last_name: lastName, phone });
      return { success: true, customer };
    } catch {
      return fail(500, { error: 'Failed to update profile. Please try again.' });
    }
  }
};
