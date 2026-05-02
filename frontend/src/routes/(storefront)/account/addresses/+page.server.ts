import { fail, redirect } from '@sveltejs/kit';
import { getMyAddresses, deleteMyAddress } from '$lib/api';
import type { Actions, PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ parent }) => {
  const { token } = await parent();
  const addresses = token ? await getMyAddresses(token).catch(() => []) : [];
  return { addresses };
};

export const actions: Actions = {
  delete: async ({ request, cookies }) => {
    const token = cookies.get('customer_token') ?? '';
    const form = await request.formData();
    const id = form.get('id')?.toString();
    if (!id) return fail(400, { error: 'Missing address id' });
    await deleteMyAddress(token, id);
    throw redirect(303, '/account/addresses');
  }
};
