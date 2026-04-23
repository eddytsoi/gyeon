import { redirect } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';
import { adminGetCustomers } from '$lib/api/admin';

export const load: PageServerLoad = async ({ parent }) => {
  const { token } = await parent();
  if (!token) throw redirect(303, '/admin/login');

  const customers = await adminGetCustomers(token).catch(() => []);
  return { customers };
};
