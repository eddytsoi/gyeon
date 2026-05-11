import { adminGetMediaPage, adminGetSettings, type MediaFile } from '$lib/api/admin';
import { extractMediaUploadLimits } from '$lib/media';
import type { PageServerLoad } from './$types';

const INITIAL_LIMIT = 20;

export const load: PageServerLoad = async ({ cookies }) => {
  const token = cookies.get('admin_token') ?? '';
  const [page, settings] = token
    ? await Promise.all([
        adminGetMediaPage(token, { limit: INITIAL_LIMIT, offset: 0, type: 'all' }).catch(
          () => ({ items: [] as MediaFile[], total: 0 })
        ),
        adminGetSettings(token).catch(() => [])
      ])
    : [{ items: [] as MediaFile[], total: 0 }, []];
  const uploadLimits = extractMediaUploadLimits(settings);
  return {
    media: page.items,
    total: page.total,
    initialLimit: INITIAL_LIMIT,
    token,
    uploadLimits
  };
};
