import { adminGetMedia, type MediaFile } from '$lib/api/admin';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ cookies }) => {
  const token = cookies.get('admin_token') ?? '';
  const media = token
    ? await adminGetMedia(token).catch(() => [] as MediaFile[])
    : ([] as MediaFile[]);
  return { media, token };
};
