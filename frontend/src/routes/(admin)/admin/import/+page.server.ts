import { redirect } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ parent, cookies }) => {
  const { token: parentToken } = await parent();
  if (!parentToken) throw redirect(303, '/admin/login');

  // Pass token to the page so client-side fetch can include the Bearer header.
  const token = cookies.get('admin_token') ?? '';
  return { token };
};
