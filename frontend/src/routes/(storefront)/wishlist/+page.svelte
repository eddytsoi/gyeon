<script lang="ts">
  import { onMount } from 'svelte';
  import { wishlistStore } from '$lib/stores/wishlist.svelte';
  import { getProductByID, getProductImages, getProductVariants } from '$lib/api';
  import type { Product, ProductImage, Variant } from '$lib/types';
  import ProductCard from '$lib/components/shop/ProductCard.svelte';
  import Seo from '$lib/components/Seo.svelte';
  import * as m from '$lib/paraglide/messages';
  import { page } from '$app/state';

  type Card = { product: Product; image?: ProductImage; variant?: Variant };
  let cards = $state<Card[]>([]);
  let loading = $state(true);

  // For guest users we only have product IDs in localStorage; need to hydrate
  // from the public API. For authenticated users the server already returns
  // joined slug/name/image, but we still need the cheapest variant.
  async function hydrate(productIDs: string[]) {
    if (productIDs.length === 0) {
      cards = [];
      loading = false;
      return;
    }
    const results = await Promise.all(
      productIDs.map(async (pid) => {
        try {
          const [product, images, variants] = await Promise.all([
            getProductByID(pid),
            getProductImages(pid).catch(() => [] as ProductImage[]),
            getProductVariants(pid).catch(() => [] as Variant[])
          ]);
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
    loading = false;
  }

  onMount(async () => {
    await wishlistStore.init(!!page.data.customer);
    await hydrate(wishlistStore.ids);
  });

  $effect(() => {
    // Re-hydrate when ids change (e.g. user removes an item from this page).
    hydrate(wishlistStore.ids);
  });
</script>

<Seo title={m.wishlist_title()} description={m.wishlist_heading()} />

<div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-10">
  <h1 class="text-3xl font-bold text-gray-900 mb-2">{m.wishlist_heading()}</h1>
  {#if cards.length > 0}
    <p class="text-sm text-gray-500 mb-6">
      {cards.length === 1 ? m.wishlist_count_one({ count: cards.length }) : m.wishlist_count_many({ count: cards.length })}
    </p>
  {/if}

  {#if !page.data.customer && cards.length > 0}
    <p class="text-sm text-gray-500 mb-6 bg-gray-50 px-4 py-3 rounded-xl">
      {m.wishlist_login_prompt()}
      <a href="/account/login" class="ml-1 underline underline-offset-2 text-gray-900">{m.wishlist_login_link()}</a>
    </p>
  {/if}

  {#if loading}
    <div class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-4 md:gap-6">
      {#each Array(4) as _}
        <div class="aspect-[3/4] bg-gray-100 rounded-2xl animate-pulse"></div>
      {/each}
    </div>
  {:else if cards.length === 0}
    <div class="text-center py-24 text-gray-400">
      <p class="text-base mb-4">{m.wishlist_empty()}</p>
      <a href="/products" class="text-sm text-gray-900 underline underline-offset-2">{m.wishlist_empty_cta()}</a>
    </div>
  {:else}
    <div class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-4 md:gap-6">
      {#each cards as item (item.product.id)}
        <ProductCard product={item.product} image={item.image} variant={item.variant} />
      {/each}
    </div>
  {/if}
</div>
