import { fail, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';
import { adminGetUsers, adminCreateUser, adminUpdateUser, adminDeleteUser } from '$lib/api/admin';

const PAGE_SIZE = 50;

export const load: PageServerLoad = async ({ parent, url }) => {
  const { token } = await parent();
  if (!token) throw redirect(303, '/admin/login');

  const q = url.searchParams.get('q') ?? '';
  const pageNum = Math.max(1, parseInt(url.searchParams.get('page') ?? '1', 10) || 1);
  const offset = (pageNum - 1) * PAGE_SIZE;

  const res = await adminGetUsers(token, PAGE_SIZE, offset, q).catch(() => ({ items: [], total: 0 }));
  return {
    users: res.items,
    total: res.total,
    page: pageNum,
    pageSize: PAGE_SIZE,
    q
  };
};

export const actions: Actions = {
  create: async ({ request, cookies }) => {
    const token = cookies.get('admin_token');
    if (!token) return fail(401, { error: 'Unauthorized' });

    const form = await request.formData();
    try {
      await adminCreateUser(token, {
        email: form.get('email')?.toString() ?? '',
        password: form.get('password')?.toString() ?? '',
        name: form.get('name')?.toString() ?? '',
        role: form.get('role')?.toString() ?? 'editor'
      });
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : '';
      if (msg.includes('409')) return fail(409, { error: 'Email already registered' });
      return fail(400, { error: 'Failed to create user' });
    }
    return { success: true };
  },

  update: async ({ request, cookies }) => {
    const token = cookies.get('admin_token');
    if (!token) return fail(401, { error: 'Unauthorized' });

    const form = await request.formData();
    const id = form.get('id')?.toString() ?? '';
    try {
      await adminUpdateUser(token, id, {
        name: form.get('name')?.toString() ?? '',
        role: form.get('role')?.toString() ?? 'editor',
        is_active: form.get('is_active') === 'true'
      });
    } catch {
      return fail(400, { error: 'Failed to update user' });
    }
    return { success: true };
  },

  delete: async ({ request, cookies }) => {
    const token = cookies.get('admin_token');
    if (!token) return fail(401, { error: 'Unauthorized' });

    const form = await request.formData();
    const id = form.get('id')?.toString() ?? '';
    try {
      await adminDeleteUser(token, id);
    } catch {
      return fail(400, { error: 'Failed to delete user' });
    }
    return { success: true };
  }
};
