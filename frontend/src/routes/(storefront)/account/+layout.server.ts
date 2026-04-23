import { redirect } from '@sveltejs/kit';
import { getMyProfile } from '$lib/api';
import type { LayoutServerLoad } from './$types';

const PUBLIC_PATHS = ['/account/login', '/account/register'];

export const load: LayoutServerLoad = async ({ cookies, url }) => {
  const token = cookies.get('customer_token') ?? null;
  const isPublic = PUBLIC_PATHS.includes(url.pathname);

  if (!token && !isPublic) throw redirect(303, '/account/login');
  if (token && isPublic) throw redirect(303, '/account');

  let customer = null;
  if (token) {
    customer = await getMyProfile(token).catch(() => {
      // Token invalid/expired — clear it and redirect to login
      cookies.delete('customer_token', { path: '/' });
      throw redirect(303, '/account/login');
    });
  }

  return { token, customer };
};
