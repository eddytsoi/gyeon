import { redirect } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';
import { adminGetCustomers } from '$lib/api/admin';

export const load: PageServerLoad = async ({ parent, url }) => {
  const { token } = await parent();
  if (!token) throw redirect(303, '/admin/login');

  const q = url.searchParams.get('q') ?? '';
  const customers = await adminGetCustomers(token, 50, 0, q).catch(() => []);
  return { customers, q };
};
