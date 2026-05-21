import { adminGetStockMutation } from '$lib/api/admin';
import { error, redirect } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ parent, params }) => {
  const { token } = await parent();
  if (!token) throw redirect(303, '/admin/login');
  try {
    const mutation = await adminGetStockMutation(token, params.id);
    return { mutation };
  } catch (e) {
    const msg = e instanceof Error ? e.message : 'failed to load mutation';
    throw error(404, msg);
  }
};
