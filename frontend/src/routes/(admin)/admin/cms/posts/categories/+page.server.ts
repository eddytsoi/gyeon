import {
  adminGetPostCategories,
  adminCreatePostCategory,
  adminUpdatePostCategory,
  adminDeletePostCategory,
  adminGetMedia,
  type PostCategory,
  type MediaFile
} from '$lib/api/admin';
import { fail } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ cookies }) => {
  const token = cookies.get('admin_token') ?? '';
  const [categories, mediaFiles] = await Promise.all([
    adminGetPostCategories(token).catch(() => [] as PostCategory[]),
    adminGetMedia(token).catch(() => [] as MediaFile[])
  ]);
  return { categories, mediaFiles };
};

function bannerFields(data: FormData) {
  const desktop = (data.get('desktop_banner_url') as string | null)?.trim() || undefined;
  const mobile = (data.get('mobile_banner_url') as string | null)?.trim() || undefined;
  return { desktop_banner_url: desktop, mobile_banner_url: mobile };
}

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
        ...bannerFields(data),
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
        ...bannerFields(data),
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
