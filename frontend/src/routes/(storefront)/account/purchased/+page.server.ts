import { getMyPurchasedProducts } from '$lib/api';
import type { PageServerLoad } from './$types';

// The provenance list (which product, from which order/bundle) comes from the
// authenticated aggregation endpoint. Current price / stock / image are
// hydrated client-side in +page.svelte via the public product APIs, mirroring
// the Wishlist page — so buy-again always reflects live data, not snapshots.
export const load: PageServerLoad = async ({ parent }) => {
  const { token } = await parent();
  const purchased = token
    ? await getMyPurchasedProducts(token).catch(() => [])
    : [];
  return { purchased };
};
