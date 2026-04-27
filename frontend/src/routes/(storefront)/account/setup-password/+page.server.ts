import type { PageServerLoad, Actions } from './$types';
import { fail, redirect } from '@sveltejs/kit';
import { setupPassword } from '$lib/api';

export const load: PageServerLoad = async ({ url }) => {
  const token = url.searchParams.get('token') ?? '';
  return { token };
};

export const actions: Actions = {
  default: async ({ request }) => {
    const form = await request.formData();
    const token = String(form.get('token') ?? '');
    const password = String(form.get('password') ?? '');
    const confirm = String(form.get('confirm') ?? '');

    if (!token) return fail(400, { error: '此連結缺少必要的識別碼。' });
    if (password.length < 8) return fail(400, { error: '密碼長度至少 8 個字元。' });
    if (password !== confirm) return fail(400, { error: '兩次輸入的密碼不一致。' });

    const res = await setupPassword(token, password);
    if (res.status === 410) return fail(410, { error: '此連結已過期或已被使用。' });
    if (!res.ok) return fail(400, { error: '無法完成設定，請稍後再試。' });

    throw redirect(303, '/account/login?welcome=1');
  }
};
