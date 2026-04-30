import {
  adminGetPostCategories,
  adminCreatePostCategory,
  adminUpdatePostCategory,
  adminDeletePostCategory,
  type PostCategory
} from '$lib/api/admin';
import { fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ cookies }) => {
  const token = cookies.get('admin_token') ?? '';
  const categories = await adminGetPostCategories(token).catch(() => [] as PostCategory[]);
  return { categories };
};

export const actions: Actions = {
  create: async ({ request, cookies }) => {
    const token = cookies.get('admin_token') ?? '';
    const data = await request.formData();
    try {
      // New categories go to the end of the list — drag-and-drop later.
      const existing = await adminGetPostCategories(token).catch(() => [] as PostCategory[]);
      await adminCreatePostCategory(token, {
        slug: (data.get('slug') as string).trim(),
        name: (data.get('name') as string).trim(),
        sort_order: existing.length + 1
      });
      return { success: true };
    } catch {
      return fail(500, { error: 'Failed to create category' });
    }
  },

  update: async ({ request, cookies }) => {
    const token = cookies.get('admin_token') ?? '';
    const data = await request.formData();
    const id = data.get('id') as string;
    try {
      // Preserve existing sort_order — order is managed via drag-and-drop.
      const existing = await adminGetPostCategories(token).catch(() => [] as PostCategory[]);
      const current = existing.find((c) => c.id === id);
      await adminUpdatePostCategory(token, id, {
        slug: (data.get('slug') as string).trim(),
        name: (data.get('name') as string).trim(),
        sort_order: current?.sort_order ?? 0
      });
      return { success: true };
    } catch {
      return fail(500, { error: 'Failed to update category' });
    }
  },

  delete: async ({ request, cookies }) => {
    const token = cookies.get('admin_token') ?? '';
    const data = await request.formData();
    const id = data.get('id') as string;
    try {
      await adminDeletePostCategory(token, id);
      return { success: true };
    } catch {
      return fail(500, { error: 'Failed to delete category' });
    }
  }
};
