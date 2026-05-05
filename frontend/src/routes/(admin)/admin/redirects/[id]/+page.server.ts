import {
  adminGetRedirect,
  adminCreateRedirect,
  adminUpdateRedirect,
  type Redirect,
  type RedirectInput
} from '$lib/api/admin';
import { fail, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ params, cookies }) => {
  const token = cookies.get('admin_token') ?? '';
  if (params.id === 'new') {
    return { redirect: null as Redirect | null };
  }
  const r = await adminGetRedirect(token, params.id).catch(() => null);
  return { redirect: r };
};

export const actions: Actions = {
  save: async ({ request, params, cookies }) => {
    const token = cookies.get('admin_token') ?? '';
    const data = await request.formData();

    const fromPath = String(data.get('from_path') ?? '').trim();
    const toPath = String(data.get('to_path') ?? '').trim();
    const codeRaw = Number(data.get('code') ?? 301);
    const code = (codeRaw === 302 ? 302 : 301) as 301 | 302;
    const isActive = data.get('is_active') === 'true';
    const noteRaw = String(data.get('note') ?? '').trim();

    if (!fromPath || !fromPath.startsWith('/')) {
      return fail(400, { error: 'From path must start with /' });
    }
    if (!toPath) return fail(400, { error: 'To path is required' });
    if (fromPath === toPath) return fail(400, { error: 'From and To must differ' });

    const body: RedirectInput = {
      from_path: fromPath,
      to_path: toPath,
      code,
      is_active: isActive,
      note: noteRaw || null
    };

    try {
      if (params.id === 'new') {
        await adminCreateRedirect(token, body);
      } else {
        await adminUpdateRedirect(token, params.id, body);
      }
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Failed to save redirect';
      return fail(500, { error: msg });
    }
    redirect(303, '/admin/redirects');
  }
};
