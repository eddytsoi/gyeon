<script lang="ts">
  /*
   * Compact "mini" variant of the PDP up-sells block. Instead of the full-width
   * UpsellGrid (a grid of vertical ProductCards), this renders a vertical stack
   * of horizontal cards — 120×120 image on the left; name / subtitle / price /
   * quick-add on the right — sized to sit in the PDP right column, in the gap
   * below the product info. Shows a heading only (no kicker). Reuses the same
   * one-click add-to-cart logic as UpsellGrid.
   */
  import type { Product } from '$lib/types';
  import { cartStore } from '$lib/stores/cart.svelte';
  import { trackAddToCart } from '$lib/tracker';
  import { formatHKD } from '$lib/money';
  import ResponsiveImage from '$lib/components/ResponsiveImage.svelte';
  import * as m from '$lib/paraglide/messages';

  let {
    items,
    heading
  }: {
    items: Product[];
    heading?: string;
  } = $props();

  // Per-product transient state, keyed by product id.
  let adding = $state<Record<string, boolean>>({});
  let added = $state<Record<string, boolean>>({});

  function canAdd(p: Product): boolean {
    return !!(p.default_variant_id && (p.default_variant_stock_qty ?? 0) > 0 && p.purchasable !== false);
  }

  function hasDiscount(p: Product): boolean {
    return p.default_variant_compare_at_price != null && p.default_variant_price != null
      && p.default_variant_compare_at_price > p.default_variant_price;
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
</script>

{#if items.length > 0}
  <section class="mt-10 lg:mt-12">
    <h2 class="font-display text-xl font-bold tracking-tight text-ink-900 mb-4">
      {heading ?? m.product_detail_upsells_heading()}
    </h2>

    <ul class="flex flex-col gap-5">
      {#each items as p (p.id)}
        {@const enabled = canAdd(p)}
        <li class="flex gap-4 items-start">
          <a href="/products/{p.slug}" class="group flex-shrink-0">
            <div class="w-[120px] h-[120px] rounded-lg overflow-hidden bg-white">
              {#if p.primary_image_url}
                <ResponsiveImage src={p.primary_image_url} alt={p.name}
                                 widths={[120, 240]} sizes="120px"
                                 class="w-full h-full object-cover transition-transform duration-500 ease-gy group-hover:scale-[1.04]" />
              {:else}
                <div class="w-full h-full flex items-center justify-center text-ink-300">
                  <svg xmlns="http://www.w3.org/2000/svg" class="h-10 w-10" aria-hidden="true" fill="none"
                       viewBox="0 0 24 24" stroke="currentColor" stroke-width="1">
                    <path stroke-linecap="round" stroke-linejoin="round"
                      d="m2.25 15.75 5.159-5.159a2.25 2.25 0 0 1 3.182 0l5.159 5.159m-1.5-1.5
                         1.409-1.409a2.25 2.25 0 0 1 3.182 0l2.909 2.909m-18 3.75h16.5a1.5 1.5
                         0 0 0 1.5-1.5V6a1.5 1.5 0 0 0-1.5-1.5H3.75A1.5 1.5 0 0 0 2.25 6v12a1.5
                         1.5 0 0 0 1.5 1.5Zm10.5-11.25h.008v.008h-.008V8.25Z" />
                  </svg>
                </div>
              {/if}
            </div>
          </a>

          <div class="flex-1 min-w-0 flex flex-col gap-1">
            <a href="/products/{p.slug}" class="group">
              <h3 class="font-display text-lg font-medium text-ink-500 line-clamp-2 group-hover:text-navy-500 transition-colors uppercase">
                {p.name}
              </h3>
              {#if p.subtitle}
                <p class="font-display text-[0.85rem] font-normal text-ink-900 line-clamp-1 tracking-wide uppercase">
                  {p.subtitle}
                </p>
              {/if}
            </a>

            {#if p.purchasable !== false && p.default_variant_price != null}
              <div class="mt-1 flex items-baseline gap-2">
                <span class="font-display tabular-nums text-ink-900 text-2xl font-medium">
                  {formatHKD(p.default_variant_price)}
                </span>
                {#if hasDiscount(p)}
                  <span class="text-sm font-body line-through tabular-nums text-ink-500">
                    {formatHKD(p.default_variant_compare_at_price!)}
                  </span>
                {/if}
              </div>
            {/if}

            <button
              type="button"
              onclick={() => addOne(p)}
              disabled={!enabled || adding[p.id]}
              class="mt-2 self-start h-10 px-4 rounded-md font-display font-bold text-sm uppercase tracking-[0.1em] transition-all duration-200 ease-gy text-white
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
          </div>
        </li>
      {/each}
    </ul>
  </section>
{/if}
