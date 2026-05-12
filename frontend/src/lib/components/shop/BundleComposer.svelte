<script lang="ts">
  /*
   * "Complete the set" composer — gyeon-project-design-system §4.7.
   *
   * A row of mini cards that each act as toggleable add-to-cart proposals,
   * with a running total and a single "add all" CTA that batches the
   * selected variants into the cart in one go. Uses each related product's
   * `default_variant_id` (the cheapest variant, surfaced by the list endpoint).
   */
  import type { Product, ProductImage } from '$lib/types';
  import { cartStore } from '$lib/stores/cart.svelte';
  import { trackAddToCart } from '$lib/tracker';
  import * as m from '$lib/paraglide/messages';

  interface Item extends Product {
    primaryImage: ProductImage | null;
  }

  let { items }: { items: Item[] } = $props();

  // Selected by default = every item with a usable variant + a price > 0.
  // SvelteMap keeps id keys stable across re-renders.
  const initial = items.reduce<Record<string, boolean>>((acc, p) => {
    acc[p.id] = !!(p.default_variant_id && (p.min_price ?? 0) > 0);
    return acc;
  }, {});
  let selected = $state<Record<string, boolean>>(initial);

  const totalSale = $derived(
    items.reduce((sum, p) => selected[p.id] && p.min_price ? sum + p.min_price : sum, 0)
  );
  const totalRegular = $derived(
    items.reduce((sum, p) => selected[p.id] && (p.min_compare_at_price ?? p.min_price) ? sum + (p.min_compare_at_price ?? p.min_price ?? 0) : sum, 0)
  );
  const saved = $derived(Math.max(0, totalRegular - totalSale));
  const selectedCount = $derived(Object.values(selected).filter(Boolean).length);

  let adding = $state(false);
  let added = $state(false);

  async function addAll() {
    if (adding || selectedCount === 0) return;
    adding = true;
    try {
      for (const p of items) {
        if (!selected[p.id] || !p.default_variant_id) continue;
        await cartStore.add(p.default_variant_id, 1);
        trackAddToCart({
          id: p.id,
          name: p.name,
          price: p.min_price ?? 0,
          quantity: 1
        });
      }
      added = true;
      setTimeout(() => (added = false), 2500);
    } finally {
      adding = false;
    }
  }
</script>

{#if items.length > 0}
  <section class="bg-paper border-y border-ink-300/60">
    <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-12 md:py-16">
      <p class="text-[11px] font-display font-semibold uppercase tracking-[0.18em] text-navy-500 mb-2">
        {m.product_detail_related_kicker()}
      </p>
      <h2 class="font-display text-2xl md:text-3xl font-bold tracking-tight text-ink-900">
        {m.product_detail_related_heading()}
      </h2>
      <div class="mt-3 h-px w-12 bg-navy-500"></div>

      <div class="mt-8 grid lg:grid-cols-[1fr_auto] gap-8 lg:gap-12 items-start">

        <!-- Mini-card row -->
        <ul class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-4 md:gap-5">
          {#each items as p (p.id)}
            {@const enabled = !!(p.default_variant_id && (p.min_price ?? 0) > 0)}
            {@const checked = !!selected[p.id]}
            <li>
              <label
                class="group flex flex-col gap-3 p-3 bg-white rounded-lg border-2 transition-all duration-200 ease-gy cursor-pointer
                       {enabled ? '' : 'opacity-50 cursor-not-allowed'}
                       {checked ? 'border-navy-500' : 'border-ink-300/60 hover:border-navy-500'}">
                <div class="relative aspect-square bg-paper rounded-md overflow-hidden">
                  {#if p.primaryImage}
                    <img src={p.primaryImage.url} alt={p.primaryImage.alt_text ?? p.name}
                         class="w-full h-full object-cover transition-transform duration-500 ease-gy group-hover:scale-[1.04]" />
                  {/if}
                  <span class="absolute top-2 right-2 w-5 h-5 rounded-sm border-2 flex items-center justify-center transition-colors
                               {checked ? 'bg-navy-500 border-navy-500' : 'bg-white border-ink-300'}">
                    {#if checked}
                      <svg class="w-3 h-3 text-white" fill="none" stroke="currentColor" stroke-width="3" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7"/>
                      </svg>
                    {/if}
                  </span>
                </div>
                <div class="min-w-0">
                  <p class="font-display text-sm font-medium text-ink-500 line-clamp-2 group-hover:text-navy-500 transition-colors">
                    {p.name}
                  </p>
                  {#if p.min_price != null}
                    <p class="mt-1 font-display text-sm font-bold tabular-nums text-ink-900">
                      HK${p.min_price.toFixed(2)}
                    </p>
                  {/if}
                </div>
                <input
                  type="checkbox"
                  class="sr-only"
                  bind:checked={selected[p.id]}
                  disabled={!enabled}
                />
              </label>
            </li>
          {/each}
        </ul>

        <!-- Total + CTA -->
        <aside class="lg:sticky lg:top-24 bg-white p-5 rounded-lg border border-ink-300/60 lg:min-w-[260px]">
          <p class="text-[11px] font-display font-semibold uppercase tracking-[0.15em] text-ink-500">
            {m.bundle_composer_total_label()}
          </p>
          <p class="mt-1 font-display text-3xl font-bold tabular-nums text-ink-900">
            HK${totalSale.toFixed(2)}
          </p>
          {#if saved > 0}
            <p class="mt-1 text-sm font-display font-semibold text-success tabular-nums">
              {m.bundle_composer_saved({ amount: saved.toFixed(2) })}
            </p>
          {/if}
          <p class="mt-2 text-xs font-body text-ink-500">
            {m.bundle_composer_selected_count({ selected: selectedCount, total: items.length })}
          </p>
          <button
            type="button"
            onclick={addAll}
            disabled={adding || selectedCount === 0}
            class="mt-4 w-full h-11 px-5 rounded-md font-display font-bold text-sm uppercase tracking-[0.1em] text-white transition-all duration-200 ease-gy
                   {selectedCount === 0
                     ? 'bg-ink-300 cursor-not-allowed'
                     : added
                       ? 'bg-success'
                       : 'bg-navy-500 hover:bg-navy-700 active:scale-[0.98]'}"
          >
            {#if added}
              {m.bundle_composer_cta_added()}
            {:else if adding}
              {m.bundle_composer_cta_adding()}
            {:else}
              {m.bundle_composer_cta_idle()}
            {/if}
          </button>
        </aside>
      </div>
    </div>
  </section>
{/if}
