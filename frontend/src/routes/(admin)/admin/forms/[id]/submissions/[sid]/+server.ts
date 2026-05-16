// Proxy for the admin submission-detail endpoint. The admin_token cookie is
// httpOnly so the modal's lazy fetch can't attach it as a Bearer header
// itself; this handler reads the cookie server-side, re-attaches it, and
// forwards the JSON body through.
import { error } from '@sveltejs/kit';
import { adminGetFormSubmission } from '$lib/api/admin';
import type { RequestHandler } from './$types';

export const GET: RequestHandler = async ({ params, cookies }) => {
  const token = cookies.get('admin_token') ?? '';
  if (!token) throw error(401, 'not signed in');
  const sub = await adminGetFormSubmission(token, params.sid).catch(() => null);
  if (!sub) throw error(404, 'submission not found');
  return new Response(JSON.stringify(sub), {
    headers: { 'Content-Type': 'application/json' }
  });
};
