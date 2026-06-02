<script lang="ts">
  /*
   * The shared "grid of product cards, each with a quick-add button" surface.
   * Used for both the PDP up-sells ("buy this instead" alternatives) and the
   * cart cross-sells (complementary products) — see CartCrossSells. Each card
   * is a linked ProductCard (click through to the product) plus its own
   * "Add to Cart" button that one-click adds the product's default variant.
   * The kicker + heading shell mirrors BundleComposer (still used for the FBT
   * / "complete the set" section) so all suggestion sections read as one
   * design system.
   */
  import type { Product, ProductImage, Variant } from '$lib/types';
  import { cartStore } from '$lib/stores/cart.svelte';
  import { trackAddToCart } from '$lib/tracker';
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

  // Per-product transient state, keyed by product id.
  let adding = $state<Record<string, boolean>>({});
  let added = $state<Record<string, boolean>>({});

  function canAdd(p: Product): boolean {
    return !!(p.default_variant_id && (p.default_variant_stock_qty ?? 0) > 0 && p.purchasable !== false);
  }

  async function addOne(p: Product) {
    if (adding[p.id] || !p.default_variant_id || !canAdd(p)) return;
    adding[p.id] = true;
    try {
      await cartStore.add(p.default_variant_id, 1);
      trackAddToCart({ id: p.id, name: p.name, price: p.default_variant_price ?? 0, quantity: 1 });
      added[p.id] = true;
      setTimeout(() => (added[p.id] = false), 2500);
    } catch {
      // cartStore records the error; layout toast surfaces it. Swallow so the
      // rejection doesn't bubble as unhandled.
    } finally {
      adding[p.id] = false;
    }
  }

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
          {@const enabled = canAdd(p)}
          <li class="flex flex-col">
            <ProductCard product={p} image={imageOf(p)} variant={variantOf(p)} align="center" priceSize="lg" withVariantSuffix />
            <button
              type="button"
              onclick={() => addOne(p)}
              disabled={!enabled || adding[p.id]}
              class="mt-3 w-full max-w-[200px] mx-auto h-10 px-4 rounded-md font-display font-bold text-sm uppercase tracking-[0.1em] transition-all duration-200 ease-gy text-white
                     {!enabled
                       ? 'bg-ink-300 cursor-not-allowed'
                       : added[p.id]
                         ? 'bg-success'
                         : 'bg-navy-500 hover:bg-navy-700 active:scale-[0.98]'}"
            >
              {#if added[p.id]}
                {m.bundle_composer_cta_added()}
              {:else if adding[p.id]}
                {m.bundle_composer_cta_adding()}
              {:else}
                {m.bundle_composer_cta_idle()}
              {/if}
            </button>
          </li>
        {/each}
      </ul>
    </div>
  </section>
{/if}
