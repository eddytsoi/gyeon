import {
  adminGetCategories,
  adminCreateCategory,
  adminUpdateCategory,
  adminDeleteCategory,
} from '$lib/api/admin';
import type { Category } from '$lib/types';
import { fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ cookies }) => {
  const token = cookies.get('admin_token') ?? '';
  const categories = await adminGetCategories(token).catch(() => [] as Category[]);
  return { categories };
};

export const actions: Actions = {
  create: async ({ request, cookies }) => {
    const token = cookies.get('admin_token') ?? '';
    const data = await request.formData();
    try {
      await adminCreateCategory(token, {
        slug: (data.get('slug') as string).trim(),
        name: (data.get('name') as string).trim(),
        sort_order: parseInt(data.get('sort_order') as string) || 0,
        is_active: true,
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
      await adminUpdateCategory(token, id, {
        slug: (data.get('slug') as string).trim(),
        name: (data.get('name') as string).trim(),
        sort_order: parseInt(data.get('sort_order') as string) || 0,
        is_active: true,
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
      await adminDeleteCategory(token, id);
      return { success: true };
    } catch {
      return fail(500, { error: 'Failed to delete category' });
    }
  }
};
