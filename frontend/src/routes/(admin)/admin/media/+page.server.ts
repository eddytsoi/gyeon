import { adminGetMedia, adminGetSettings, type MediaFile } from '$lib/api/admin';
import { extractMediaUploadLimits } from '$lib/media';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ cookies }) => {
  const token = cookies.get('admin_token') ?? '';
  const [media, settings] = token
    ? await Promise.all([
        adminGetMedia(token).catch(() => [] as MediaFile[]),
        adminGetSettings(token).catch(() => [])
      ])
    : [[] as MediaFile[], []];
  const uploadLimits = extractMediaUploadLimits(settings);
  return { media, token, uploadLimits };
};
