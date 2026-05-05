import { adminListEmailTemplates, type EmailTemplateListItem } from '$lib/api/admin';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ cookies }) => {
  const token = cookies.get('admin_token') ?? '';
  const items = await adminListEmailTemplates(token).catch(
    () => [] as EmailTemplateListItem[]
  );
  return { items };
};
