import { redirect } from '@sveltejs/kit';
import type { RequestHandler } from './$types';

export const POST: RequestHandler = async ({ cookies }) => {
  cookies.delete('admin_token', { path: '/' });
  throw redirect(303, '/admin/login');
};
