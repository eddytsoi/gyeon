<script lang="ts">
  import type { Product, ProductImage, Variant } from '$lib/types';
  import { isVideo, isStreamingVideo } from '$lib/media';
  import * as m from '$lib/paraglide/messages';
  import WishlistButton from '$lib/components/shop/WishlistButton.svelte';

  let { product, image, variant }: {
    product: Product;
    image?: ProductImage;
    variant?: Variant;
  } = $props();

  const hasDiscount = $derived(
    variant?.compare_at_price != null && variant.compare_at_price > variant.price
  );

  const discountPct = $derived(
    hasDiscount && variant
      ? Math.round((1 - variant.price / variant.compare_at_price!) * 100)
      : 0
  );
</script>

<a href="/products/{product.slug}"
   class="group relative flex flex-col rounded-2xl overflow-hidden bg-white border border-gray-100
          hover:shadow-md transition-shadow duration-200">

  <!-- Wishlist heart (overlay) -->
  <div class="absolute top-2 right-2 z-10">
    <WishlistButton productID={product.id} variant="icon" />
  </div>

  <!-- Image -->
  <div class="aspect-square bg-gray-50 overflow-hidden">
    {#if image}
      {#if isVideo(image) && !isStreamingVideo(image) && !image.thumbnail_url}
        <video src={image.url} muted playsinline preload="metadata"
               class="w-full h-full object-cover group-hover:scale-105 transition-transform duration-300">
        </video>
      {:else if isStreamingVideo(image) && !image.thumbnail_url}
        <div class="w-full h-full flex items-center justify-center bg-gray-100">
          <svg class="w-12 h-12 text-gray-300" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
            <path stroke-linecap="round" stroke-linejoin="round" d="M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9-3.75 5.25 3.75-5.25 3.75v-7.5Z" />
          </svg>
        </div>
      {:else}
        <img src={image.thumbnail_url ?? image.url} alt={image.alt_text ?? product.name}
             class="w-full h-full object-cover group-hover:scale-105 transition-transform duration-300" />
      {/if}
    {:else}
      <div class="w-full h-full flex items-center justify-center text-gray-300">
        <svg xmlns="http://www.w3.org/2000/svg" class="h-16 w-16" fill="none"
             viewBox="0 0 24 24" stroke="currentColor" stroke-width="1">
          <path stroke-linecap="round" stroke-linejoin="round"
            d="m2.25 15.75 5.159-5.159a2.25 2.25 0 0 1 3.182 0l5.159 5.159m-1.5-1.5
               1.409-1.409a2.25 2.25 0 0 1 3.182 0l2.909 2.909m-18 3.75h16.5a1.5 1.5
               0 0 0 1.5-1.5V6a1.5 1.5 0 0 0-1.5-1.5H3.75A1.5 1.5 0 0 0 2.25 6v12a1.5
               1.5 0 0 0 1.5 1.5Zm10.5-11.25h.008v.008h-.008V8.25Zm.375 0a.375.375 0 1
               1-.75 0 .375.375 0 0 1 .75 0Z" />
        </svg>
      </div>
    {/if}
  </div>

  <!-- Info -->
  <div class="p-4 flex flex-col gap-1 flex-1">
    <h3 class="text-sm font-medium text-gray-900 line-clamp-2 group-hover:text-gray-600
               transition-colors">
      {product.name}
    </h3>

    {#if variant}
      <div class="mt-auto flex items-center gap-2 pt-2">
        <span class="font-semibold text-gray-900">
          HK${variant.price.toFixed(2)}
        </span>
        {#if hasDiscount}
          <span class="text-sm text-gray-400 line-through">
            HK${variant.compare_at_price!.toFixed(2)}
          </span>
          <span class="text-xs font-medium text-red-500">{m.product_card_discount_pct({ pct: discountPct })}</span>
        {/if}
      </div>
      {#if variant.stock_qty === 0}
        <span class="text-xs text-gray-400">{m.product_card_out_of_stock()}</span>
      {/if}
    {:else}
      <span class="mt-auto text-sm text-gray-400 pt-2">{m.product_card_no_variants()}</span>
    {/if}
  </div>
</a>
