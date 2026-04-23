import { error, fail, redirect } from '@sveltejs/kit';
import { getMyAddresses, updateMyAddress, deleteMyAddress } from '$lib/api';
import type { Actions, PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ parent, params }) => {
  const { token } = await parent();
  const addresses = token ? await getMyAddresses(token).catch(() => []) : [];
  const address = addresses.find((a) => a.id === params.id);
  if (!address) throw error(404, 'Address not found');
  return { address };
};

export const actions: Actions = {
  update: async ({ request, cookies, params }) => {
    const token = cookies.get('customer_token') ?? '';
    const form = await request.formData();

    const data = {
      first_name:  form.get('first_name')?.toString() ?? '',
      last_name:   form.get('last_name')?.toString() ?? '',
      phone:       form.get('phone')?.toString() || undefined,
      line1:       form.get('line1')?.toString() ?? '',
      line2:       form.get('line2')?.toString() || undefined,
      city:        form.get('city')?.toString() ?? '',
      state:       form.get('state')?.toString() || undefined,
      postal_code: form.get('postal_code')?.toString() ?? '',
      country:     form.get('country')?.toString() ?? 'HK',
      is_default:  form.get('is_default') === 'on'
    };

    if (!data.first_name || !data.last_name || !data.line1 || !data.city || !data.postal_code) {
      return fail(400, { error: 'Please fill in all required fields', values: data });
    }

    try {
      await updateMyAddress(token, params.id, data);
    } catch {
      return fail(500, { error: 'Failed to update address.', values: data });
    }

    throw redirect(303, '/account/addresses');
  },

  delete: async ({ cookies, params }) => {
    const token = cookies.get('customer_token') ?? '';
    await deleteMyAddress(token, params.id);
    throw redirect(303, '/account/addresses');
  }
};
