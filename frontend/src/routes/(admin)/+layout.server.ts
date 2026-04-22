import { redirect } from '@sveltejs/kit';
import type { LayoutServerLoad } from './$types';

export const load: LayoutServerLoad = async ({ cookies, url }) => {
  const token = cookies.get('admin_token');
  const isLoginPage = url.pathname === '/admin/login';

  if (!token && !isLoginPage) throw redirect(303, '/admin/login');
  if (token && isLoginPage) throw redirect(303, '/admin/dashboard');

  return { token: token ?? null };
};
