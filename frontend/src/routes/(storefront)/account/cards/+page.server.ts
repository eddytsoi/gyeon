import { getMySavedCards } from '$lib/api';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ parent }) => {
  const { token } = await parent();
  const cards = token ? await getMySavedCards(token).catch(() => []) : [];
  return { cards };
};
