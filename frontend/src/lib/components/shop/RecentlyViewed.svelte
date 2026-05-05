<script lang="ts">
  import { onMount } from 'svelte';
  import { recentlyViewedStore } from '$lib/stores/recentlyViewed.svelte';
  import { getProductByID, getProductImages, getProductVariants } from '$lib/api';
  import type { Product, ProductImage, Variant } from '$lib/types';
  import ProductCard from '$lib/components/shop/ProductCard.svelte';
  import * as m from '$lib/paraglide/messages';

  interface Props {
    /** Hide cards for this id (the product the user is currently viewing). */
    excludeID?: string;
  }
  let { excludeID }: Props = $props();

  type Card = { product: Product; image?: ProductImage; variant?: Variant };
  let cards = $state<Card[]>([]);

  onMount(async () => {
    recentlyViewedStore.init();
    const ids = excludeID ? recentlyViewedStore.others(excludeID) : recentlyViewedStore.ids;
    if (ids.length === 0) return;
    const results = await Promise.all(
      ids.map(async (pid) => {
        try {
          const [product, images, variants] = await Promise.all([
            getProductByID(pid),
            getProductImages(pid).catch(() => [] as ProductImage[]),
            getProductVariants(pid).catch(() => [] as Variant[])
          ]);
          if (product.status !== 'active') return null;
          return {
            product,
            image: images.find((i) => i.is_primary) ?? images[0],
            variant: variants.slice().sort((a, b) => a.price - b.price)[0]
          } satisfies Card;
        } catch {
          return null;
        }
      })
    );
    cards = results.filter((x): x is Card => x !== null);
  });
</script>

{#if cards.length > 0}
  <section class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-12">
    <h2 class="text-xl font-bold text-gray-900 mb-6">{m.recently_viewed_heading()}</h2>
    <div class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-4 md:gap-6">
      {#each cards as item (item.product.id)}
        <ProductCard product={item.product} image={item.image} variant={item.variant} />
      {/each}
    </div>
  </section>
{/if}
