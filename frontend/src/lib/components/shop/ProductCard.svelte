<script lang="ts">
  import type { Product, ProductImage, Variant } from '$lib/types';
  import { isVideo, isStreamingVideo } from '$lib/media';
  import * as m from '$lib/paraglide/messages';
  import WishlistButton from '$lib/components/shop/WishlistButton.svelte';
  import ResponsiveImage from '$lib/components/ResponsiveImage.svelte';

  // Grid: 2-up on mobile, 3-up on md, 4-up on xl (see (storefront)/+page.svelte
  // and /products/+page.svelte). Sizes attribute mirrors that breakdown so the
  // browser asks for the smallest variant that still covers DPR × column width.
  const CARD_WIDTHS = [320, 480, 640, 960];
  const CARD_SIZES = '(min-width: 1024px) 25vw, (min-width: 640px) 33vw, 50vw';

  let { product, image, variant, loading = 'lazy', fetchpriority = 'auto' }: {
    product: Product;
    image?: ProductImage;
    variant?: Variant;
    loading?: 'lazy' | 'eager';
    fetchpriority?: 'high' | 'auto';
  } = $props();

  const hasDiscount = $derived(
    variant?.compare_at_price != null && variant.compare_at_price > variant.price
  );

  const discountPct = $derived(
    hasDiscount && variant
      ? Math.round((1 - variant.price / variant.compare_at_price!) * 100)
      : 0
  );

  const soldOut = $derived(variant != null && variant.stock_qty === 0);
</script>

<div class="group relative flex flex-col">
  <a href="/products/{product.slug}" class="flex flex-col">
    <!-- Square media -->
    <div class="relative aspect-square bg-paper overflow-hidden rounded-lg">
      {#if image}
        {#if isVideo(image) && !isStreamingVideo(image) && !image.thumbnail_url}
          <video src={image.url} muted playsinline preload="metadata"
                 class="w-full h-full object-cover transition-transform duration-500 ease-gy group-hover:scale-[1.04]">
          </video>
        {:else if isStreamingVideo(image) && !image.thumbnail_url}
          <div class="w-full h-full flex items-center justify-center bg-paper">
            <svg class="w-12 h-12 text-ink-300" aria-hidden="true" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
              <path stroke-linecap="round" stroke-linejoin="round" d="M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9-3.75 5.25 3.75-5.25 3.75v-7.5Z" />
            </svg>
          </div>
        {:else}
          <ResponsiveImage src={image.url} alt={image.alt_text ?? product.name}
                           widths={CARD_WIDTHS} sizes={CARD_SIZES}
                           {loading} {fetchpriority}
                           class="w-full h-full object-cover transition-transform duration-500 ease-gy group-hover:scale-[1.04]" />
        {/if}
      {:else}
        <div class="w-full h-full flex items-center justify-center text-ink-300">
          <svg xmlns="http://www.w3.org/2000/svg" class="h-16 w-16" aria-hidden="true" fill="none"
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

      <!-- Discount chip — square, navy, top-right -->
      {#if hasDiscount}
        <span class="absolute top-3 right-3 inline-flex items-center justify-center
                     w-12 h-12 bg-navy-500 text-white font-display font-bold text-sm
                     rounded-sm tabular-nums">
          {m.product_card_discount_pct({ pct: discountPct })}
        </span>
      {/if}

      <!-- Sold-out pill — bottom-left -->
      {#if soldOut}
        <span class="absolute bottom-3 left-3 px-2.5 py-1 bg-white/95 text-ink-900
                     text-[11px] uppercase tracking-[0.15em] font-semibold rounded-sm">
          {m.product_card_out_of_stock()}
        </span>
      {/if}
    </div>

    <!-- Text -->
    <div class="pt-4 flex flex-col gap-1">
      {#if product.subtitle}
        <p class="font-display text-[0.7438rem] md:text-[0.85rem] font-normal text-ink-900 line-clamp-1 tracking-wide uppercase">
          {product.subtitle}
        </p>
      {/if}
      <h3 class="font-display text-lg md:text-xl font-medium text-ink-500 line-clamp-2 group-hover:text-navy-500 transition-colors">
        {product.name}
      </h3>

      {#if variant}
        <div class="mt-1 flex items-baseline gap-2">
          <span class="font-display text-base md:text-lg font-bold tabular-nums text-ink-900">
            HK${variant.price.toFixed(2)}
          </span>
          {#if hasDiscount}
            <span class="text-sm font-body line-through tabular-nums text-ink-500">
              HK${variant.compare_at_price!.toFixed(2)}
            </span>
          {/if}
        </div>
      {:else}
        <span class="mt-1 text-sm text-ink-500">{m.product_card_no_variants()}</span>
      {/if}
    </div>
  </a>

  <!-- Wishlist heart — top-left, ghost, hover-revealed (always visible on touch) -->
  <div class="absolute top-3 left-3 z-10 opacity-0 group-hover:opacity-100 focus-within:opacity-100 transition-opacity duration-200"
       data-hover-only>
    <WishlistButton productID={product.id} variant="icon" />
  </div>
</div>
