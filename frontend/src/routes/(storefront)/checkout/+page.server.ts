import { getMyProfile, getMyAddresses, getPaymentConfig, getPublicSettings } from '$lib/api';
import type { PageServerLoad } from './$types';

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

export const load: PageServerLoad = async ({ cookies }) => {
  const token = cookies.get('customer_token') ?? null;
  const [paymentConfig, settings] = await Promise.all([
    getPaymentConfig().catch(() => ({ publishable_key: '', mode: 'test' as const })),
    getPublicSettings().catch(() => [])
  ]);
  const shippingCountries = parseShippingCountries(
    settings.find((s) => s.key === 'shipping_countries')?.value
  );

  if (!token) {
    return { token: null, customer: null, addresses: [], paymentConfig, shippingCountries };
  }

  try {
    const [customer, addresses] = await Promise.all([
      getMyProfile(token),
      getMyAddresses(token)
    ]);
    return { token, customer, addresses, paymentConfig, shippingCountries };
  } catch {
    return { token: null, customer: null, addresses: [], paymentConfig, shippingCountries };
  }
};
