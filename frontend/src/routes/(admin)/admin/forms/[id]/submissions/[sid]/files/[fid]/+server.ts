// Proxy for admin downloads of contact-form attachments. admin_token is
// httpOnly, so the browser can't add its own Bearer header for a backend
// call — this handler reads the cookie server-side and streams the file
// through to the client with its original filename + content-type intact.
import { error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';

const BACKEND = process.env.API_BASE ?? 'http://localhost:8080/api/v1';

export const GET: RequestHandler = async ({ params, cookies, fetch }) => {
  const token = cookies.get('admin_token') ?? '';
  if (!token) throw error(401, 'not signed in');

  const upstream = await fetch(
    `${BACKEND}/admin/forms/submissions/${params.sid}/files/${params.fid}`,
    { headers: { Authorization: `Bearer ${token}` } }
  );
  if (upstream.status === 404) throw error(404, 'file not found');
  if (!upstream.ok) throw error(upstream.status, 'failed to fetch file');

  // Stream the body through, preserving disposition + type + length so the
  // browser's <a download> attribute works correctly.
  const headers = new Headers();
  for (const k of ['content-type', 'content-length', 'content-disposition']) {
    const v = upstream.headers.get(k);
    if (v) headers.set(k, v);
  }
  return new Response(upstream.body, { status: 200, headers });
};
