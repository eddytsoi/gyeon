import { redirect } from '@sveltejs/kit';
import { getMyProfile, getPublicSettings } from '$lib/api';
import type { LayoutServerLoad } from './$types';

const PUBLIC_PATHS = ['/account/login', '/account/register', '/account/setup-password', '/account/reset-password'];

export const load: LayoutServerLoad = async ({ cookies, url }) => {
  const token = cookies.get('customer_token') ?? null;
  const isPublic = PUBLIC_PATHS.includes(url.pathname);

  if (!token && !isPublic) throw redirect(303, '/account/login');
  if (token && isPublic) throw redirect(303, '/account/orders');

  const settings = await getPublicSettings().catch(() => []);
  const googleOAuthEnabled = settings.find((s) => s.key === 'google_oauth_enabled')?.value === 'true';
  const appleOAuthEnabled = settings.find((s) => s.key === 'apple_oauth_enabled')?.value === 'true';
  const accountPageLayout = settings.find((s) => s.key === 'account_page_layout')?.value || 'classic';

  let customer = null;
  if (token) {
    customer = await getMyProfile(token).catch(() => {
      // Token invalid/expired — clear it and redirect to login
      cookies.delete('customer_token', { path: '/' });
      throw redirect(303, '/account/login');
    });
  }

  return { token, customer, googleOAuthEnabled, appleOAuthEnabled, accountPageLayout };
};
