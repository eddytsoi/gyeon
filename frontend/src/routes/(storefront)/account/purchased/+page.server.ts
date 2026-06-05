import { getMyPurchasedProducts, getCategories } from '$lib/api';
import type { PageServerLoad } from './$types';

// The provenance list (which product, from which order/bundle) comes from the
// authenticated aggregation endpoint. Current price / stock / image are
// hydrated client-side in +page.svelte via the public product APIs, mirroring
// the Wishlist page — so buy-again always reflects live data, not snapshots.
//
// `categories` powers the storefront category dropdown filter; fetched
// role-aware via the customer token (same call the products page uses).
export const load: PageServerLoad = async ({ parent }) => {
  const { token } = await parent();
  const [purchased, categories] = await Promise.all([
    token ? getMyPurchasedProducts(token).catch(() => []) : Promise.resolve([]),
    getCategories(token).catch(() => []).then((r) => r ?? [])
  ]);
  return { purchased, categories };
};
