<script lang="ts">
  /*
   * WooCommerce cross-sells — complementary products promoted in the cart
   * based on its contents. The cart is client-side (no server loader), so we
   * fetch from the browser keyed by the cart's variant IDs and render the
   * shared UpsellGrid (the "grid of product cards, each with a quick-add
   * button" surface) so cross-sells match the PDP up-sells style. Products
   * already in the cart are excluded server-side. Mounted just above
   * RecentlyViewed in both cart layouts.
   */
  import type { Product } from '$lib/types';
  import { cartStore } from '$lib/stores/cart.svelte';
  import { getCartCrossSells } from '$lib/api';
  import UpsellGrid from '$lib/components/shop/UpsellGrid.svelte';
  import * as m from '$lib/paraglide/messages';

  let {
    kicker,
    heading
  }: {
    kicker?: string;
    heading?: string;
  } = $props();

  // A primitive key over the deduped, sorted variant-id set so the fetch only
  // re-runs when the *set* of cart products changes — quantity-only edits
  // recompute the same string and don't trigger a refetch.
  const variantKey = $derived(
    [...new Set((cartStore.cart?.items ?? []).map((i) => i.variant_id))].sort().join(',')
  );

  let items = $state<Product[]>([]);

  $effect(() => {
    const key = variantKey;
    if (!key) {
      items = [];
      return;
    }
    let cancelled = false;
    getCartCrossSells(key.split(','), 4)
      .then((r) => {
        if (!cancelled) items = r;
      })
      .catch(() => {
        if (!cancelled) items = [];
      });
    // Guard against a slow earlier response overwriting a newer cart's result.
    return () => {
      cancelled = true;
    };
  });
</script>

{#if items.length > 0}
  <UpsellGrid
    {items}
    kicker={kicker ?? m.cart_cross_sells_kicker()}
    heading={heading ?? m.cart_cross_sells_heading()}
  />
{/if}
