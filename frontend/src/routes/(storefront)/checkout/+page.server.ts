import { getMyProfile, getMyAddresses } from '$lib/api';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ cookies }) => {
  const token = cookies.get('customer_token') ?? null;
  if (!token) return { token: null, customer: null, addresses: [] };

  try {
    const [customer, addresses] = await Promise.all([
      getMyProfile(token),
      getMyAddresses(token)
    ]);
    return { token, customer, addresses };
  } catch {
    return { token: null, customer: null, addresses: [] };
  }
};
