import { adminGetMediaFile, adminUpdateMedia, adminDeleteMedia } from '$lib/api/admin';
import { error, fail, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ params, cookies }) => {
  const token = cookies.get('admin_token') ?? '';
  const file = token
    ? await adminGetMediaFile(token, params.id).catch(() => null)
    : null;
  if (!file) throw error(404, 'Media not found');
  return { file };
};

export const actions: Actions = {
  save: async ({ request, params, cookies }) => {
    const token = cookies.get('admin_token') ?? '';
    const data = await request.formData();
    const body: {
      original_name?: string;
      url?: string;
      video_autoplay?: boolean;
      video_fit?: 'contain' | 'cover';
    } = {};

    const name = (data.get('original_name') as string)?.trim();
    if (name) body.original_name = name;

    const url = (data.get('url') as string)?.trim();
    if (url) body.url = url;

    if (data.has('video_autoplay_present')) {
      body.video_autoplay = data.get('video_autoplay') === 'true';
    }

    if (data.has('video_fit_present')) {
      const fit = data.get('video_fit');
      if (fit === 'cover' || fit === 'contain') body.video_fit = fit;
    }

    try {
      await adminUpdateMedia(token, params.id, body);
    } catch {
      return fail(500, { error: 'Failed to save changes' });
    }
    return { success: true };
  },

  delete: async ({ params, cookies }) => {
    const token = cookies.get('admin_token') ?? '';
    try {
      await adminDeleteMedia(token, params.id);
    } catch {
      return fail(500, { error: 'Failed to delete media' });
    }
    redirect(303, '/admin/media');
  }
};
