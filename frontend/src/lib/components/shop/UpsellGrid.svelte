<script lang="ts">
  /*
   * WooCommerce up-sells — the "buy this instead" alternatives shown on the
   * PDP. Unlike the FBT / "complete the set" BundleComposer, up-sells are
   * alternatives: each is a plain linked ProductCard (click through to the
   * product), with no checkboxes / running total / "add all" — adding an
   * alternative alongside the viewed product would be semantically wrong.
   * The kicker + heading shell mirrors BundleComposer so the section reads as
   * part of the same design system while staying clearly distinct from FBT.
   */
  import type { Product, ProductImage, Variant } from '$lib/types';
  import ProductCard from '$lib/components/shop/ProductCard.svelte';
  import * as m from '$lib/paraglide/messages';

  let {
    items,
    kicker,
    heading
  }: {
    items: Product[];
    kicker?: string;
    heading?: string;
  } = $props();

  // Adapt the flattened ProductWithMeta row (default_variant_* + primary_image_url)
  // into ProductCard's image/variant props — same shape the /products grid uses.
  function imageOf(p: Product): ProductImage | undefined {
    if (!p.primary_image_url) return undefined;
    return {
      id: '',
      product_id: p.id,
      url: p.primary_image_url,
      thumbnail_url: p.primary_image_url,
      alt_text: p.name,
      sort_order: 0,
      is_primary: true
    };
  }

  function variantOf(p: Product): Variant | undefined {
    if (p.default_variant_price == null) return undefined;
    return {
      id: p.default_variant_id ?? '',
      product_id: p.id,
      sku: '',
      price: p.default_variant_price,
      compare_at_price: p.default_variant_compare_at_price ?? undefined,
      stock_qty: p.default_variant_stock_qty ?? 0,
      is_active: true
    };
  }
</script>

{#if items.length > 0}
  <section class="bg-white border-t border-ink-300/60">
    <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8 md:py-10">
      <p class="text-[11px] font-display font-semibold uppercase tracking-[0.18em] text-navy-500 mb-2">
        {kicker ?? m.product_detail_upsells_kicker()}
      </p>
      <h2 class="font-display text-xl md:text-2xl font-bold tracking-tight text-ink-900">
        {heading ?? m.product_detail_upsells_heading()}
      </h2>

      <ul class="mt-6 grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-x-4 gap-y-8 md:gap-x-6 md:gap-y-10">
        {#each items as p (p.id)}
          <li>
            <ProductCard product={p} image={imageOf(p)} variant={variantOf(p)} />
          </li>
        {/each}
      </ul>
    </div>
  </section>
{/if}
