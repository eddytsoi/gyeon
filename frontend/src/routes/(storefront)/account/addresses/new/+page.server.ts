import { fail, redirect } from '@sveltejs/kit';
import { createMyAddress, getPublicSettings } from '$lib/api';
import type { Actions, PageServerLoad } from './$types';

function parseShippingCountries(raw: string | undefined): string[] {
  if (!raw) return ['HK'];
  try {
    const parsed = JSON.parse(raw);
    if (Array.isArray(parsed)) {
      const codes = parsed.filter((v): v is string => typeof v === 'string');
      return codes.length > 0 ? codes : ['HK'];
    }
  } catch {
    /* fall through */
  }
  return ['HK'];
}

export const load: PageServerLoad = async ({ parent }) => {
  await parent();
  const settings = await getPublicSettings().catch(() => []);
  const shippingCountries = parseShippingCountries(
    settings.find((s) => s.key === 'shipping_countries')?.value
  );
  return { shippingCountries };
};

export const actions: Actions = {
  default: async ({ request, cookies }) => {
    const token = cookies.get('customer_token') ?? '';
    const form = await request.formData();

    const data = {
      first_name:  form.get('first_name')?.toString() ?? '',
      last_name:   form.get('last_name')?.toString() ?? '',
      phone:       form.get('phone')?.toString() || undefined,
      line1:       form.get('line1')?.toString() ?? '',
      city:        form.get('city')?.toString() ?? '',
      state:       form.get('state')?.toString() || undefined,
      postal_code: form.get('postal_code')?.toString() ?? '',
      country:     form.get('country')?.toString() ?? 'HK',
      is_default:  form.get('is_default') === 'on'
    };

    if (!data.first_name || !data.line1 || !data.country) {
      return fail(400, { error: 'Please fill in all required fields', values: data });
    }

    try {
      await createMyAddress(token, data);
    } catch {
      return fail(500, { error: 'Failed to save address. Please try again.', values: data });
    }

    throw redirect(303, '/account/addresses');
  }
};
