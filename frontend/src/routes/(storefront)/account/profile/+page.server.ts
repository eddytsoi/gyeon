import { fail } from '@sveltejs/kit';
import { updateMyProfile, getMyLoyaltyBalance } from '$lib/api';
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
