import { getMyProfile, getMyAddresses, getPaymentConfig } from '$lib/api';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ cookies }) => {
  const token = cookies.get('customer_token') ?? null;
  const paymentConfig = await getPaymentConfig().catch(() => ({ publishable_key: '', mode: 'test' as const }));

  if (!token) {
    return { token: null, customer: null, addresses: [], paymentConfig };
  }

  try {
    const [customer, addresses] = await Promise.all([
      getMyProfile(token),
      getMyAddresses(token)
    ]);
    return { token, customer, addresses, paymentConfig };
  } catch {
    return { token: null, customer: null, addresses: [], paymentConfig };
  }
};
