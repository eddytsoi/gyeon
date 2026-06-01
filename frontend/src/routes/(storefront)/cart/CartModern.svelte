<script lang="ts">
  import { cartStore } from '$lib/stores/cart.svelte';
  import { formatHKD } from '$lib/money';
  import RecentlyViewed from '$lib/components/shop/RecentlyViewed.svelte';
  import FreeShippingBanner from '$lib/components/shop/FreeShippingBanner.svelte';
  import PendingOrderBanner from '$lib/components/shop/PendingOrderBanner.svelte';
  import ResponsiveImage from '$lib/components/ResponsiveImage.svelte';
  import { productDisplayName } from '$lib/variant';
  import * as m from '$lib/paraglide/messages';
  import { resolveFreeShippingThreshold } from '$lib/shippingThreshold';
  import type { PageData } from './$types';

  // publicSettings flows in from the (storefront) layout load.
  let { data }: { data: PageData } = $props();

  const resolvedFreeShipping = $derived(
    resolveFreeShippingThreshold(data.publicSettings ?? [], data.customer?.role ?? null)
  );
  const freeShippingEnabled = $derived(resolvedFreeShipping.enabled);
  const freeShippingThreshold = $derived(() => resolvedFreeShipping.threshold);
  const shippingFree = $derived(
    freeShippingEnabled && freeShippingThreshold() > 0 && cartStore.subtotal >= freeShippingThreshold()
  );
</script>

<div class="max-w-2xl mx-auto px-4 sm:px-6 py-10">
  <h1 class="font-display font-bold uppercase tracking-tight text-2xl sm:text-3xl text-ink-900 mb-6">
    {m.cart_heading()}
  </h1>

  <PendingOrderBanner cartId={cartStore.cart?.id ?? null} />

  <div class="mb-6">
    <FreeShippingBanner settings={data.publicSettings ?? []} role={data.customer?.role ?? null} showUnlocked />
  </div>

  {#if cartStore.loading && !cartStore.cart}
    <div class="text-center py-20 text-ink-300">{m.common_loading()}</div>

  {:else if !cartStore.cart || cartStore.cart.items.length === 0}
    <div class="text-center py-20">
      <p class="text-lg text-ink-500">{m.cart_empty()}</p>
      <a href="/products"
         class="mt-6 inline-block bg-navy-500 hover:bg-navy-700 text-white
                font-display font-bold uppercase tracking-[0.12em] text-sm
                px-8 py-3 rounded-xl transition-colors">
        {m.cart_continue_shopping()}
      </a>
    </div>

  {:else}
    <!-- Receipt-style line items: one card, hairline dividers -->
    <div class="bg-white rounded-2xl border border-gray-100 shadow-card divide-y divide-gray-100 overflow-hidden">
      {#each cartStore.cart.items as item}
        <div class="px-4 sm:px-5 py-4">
          <div class="flex items-start gap-4">
            <a href="/products/{item.product_slug}?variant={encodeURIComponent(item.sku)}"
               class="w-16 h-16 sm:w-20 sm:h-20 rounded-lg bg-gray-50 shrink-0 overflow-hidden block hover:opacity-80 transition-opacity">
              {#if item.image_url}
                <ResponsiveImage src={item.image_url} alt={item.product_name}
                                 widths={[160, 320]} sizes="(min-width: 640px) 80px, 64px"
                                 class="w-full h-full object-cover" />
              {/if}
            </a>

            <div class="flex-1 min-w-0 flex flex-col gap-2">
              <div class="min-w-0">
                <a href="/products/{item.product_slug}?variant={encodeURIComponent(item.sku)}"
                   class="font-display font-medium uppercase leading-snug text-ink-900 line-clamp-2 hover:text-navy-500 transition-colors block">
                  {productDisplayName(item.product_name, item.variant_name)}
                </a>
                {#if item.product_subtitle}
                  <p class="text-xs text-ink-500 truncate mt-0.5">{item.product_subtitle}</p>
                {/if}
                <p class="text-xs text-ink-500 tabular-nums mt-0.5">
                  {formatHKD(item.price)} {m.cart_price_each()}
                </p>
              </div>

              <!-- Qty controls -->
              <div class="flex items-center border border-gray-200 rounded-lg overflow-hidden self-start">
                <button
                  type="button"
                  onclick={() => cartStore.update(item.id, item.quantity - 1)}
                  aria-label={m.common_aria_decrease_quantity()}
                  disabled={item.quantity <= 1}
                  class="w-8 h-8 flex items-center justify-center text-ink-500 hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed">−</button>
                <span class="w-8 text-center text-sm tabular-nums">{item.quantity}</span>
                <button
                  type="button"
                  onclick={() => cartStore.update(item.id, item.quantity + 1)}
                  aria-label={m.common_aria_increase_quantity()}
                  class="w-8 h-8 flex items-center justify-center text-ink-500 hover:bg-gray-50">+</button>
              </div>
            </div>

            <div class="shrink-0 flex flex-col items-end gap-3">
              <span class="font-display font-bold tabular-nums text-ink-900">
                {formatHKD(item.price * item.quantity)}
              </span>
              <button
                onclick={() => cartStore.remove(item.id)}
                class="p-1 text-ink-300 hover:text-alert transition-colors"
                aria-label={m.cart_aria_remove()}>
                <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none"
                     viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12"/>
                </svg>
              </button>
            </div>
          </div>

          {#if item.children?.length}
            <div class="mt-3 pt-3 border-t border-gray-100 flex flex-col gap-2 pl-20 sm:pl-24">
              {#each item.children as child}
                <a href="/products/{child.product_slug}?variant={encodeURIComponent(child.sku)}"
                   class="flex items-center gap-3 hover:opacity-80 transition-opacity">
                  <div class="w-10 h-10 rounded-md bg-gray-50 shrink-0 overflow-hidden">
                    {#if child.image_url}
                      <ResponsiveImage src={child.image_url} alt={child.product_name}
                                       widths={[80, 160]} sizes="40px"
                                       class="w-full h-full object-cover" />
                    {/if}
                  </div>
                  <p class="flex-1 min-w-0 text-xs text-ink-500 truncate uppercase">
                    {productDisplayName(child.product_name, child.variant_name)}
                  </p>
                  <span class="text-xs text-ink-300 tabular-nums">× {child.quantity}</span>
                </a>
              {/each}
            </div>
          {/if}
        </div>
      {/each}
    </div>

    <!-- Totals breakdown -->
    <div class="mt-6 flex flex-col gap-3">
      <div class="flex justify-between text-sm text-ink-500">
        <span>{m.cart_summary_items({ count: cartStore.itemCount })}</span>
        <span class="tabular-nums text-ink-900">{formatHKD(cartStore.subtotal)}</span>
      </div>
      <div class="flex justify-between text-sm text-ink-500">
        <span>{m.cart_summary_shipping()}</span>
        <span class="whitespace-nowrap {shippingFree ? 'text-success' : 'text-ink-900'}">
          {shippingFree ? m.shipping_sf_free() : m.shipping_sf_cod()}
        </span>
      </div>
      <div class="border-t border-gray-100 pt-3 flex justify-between items-baseline">
        <span class="font-display font-bold uppercase tracking-wide text-ink-900">{m.cart_summary_total()}</span>
        <span class="font-display font-bold text-lg tabular-nums text-ink-900">{formatHKD(cartStore.subtotal)}</span>
      </div>
    </div>

    <!-- Sticky bottom checkout bar — full-bleed within the column, stays
         tappable while scrolling a long cart at any screen size. -->
    <div class="sticky bottom-0 z-10 mt-6 -mx-4 sm:-mx-6 px-4 sm:px-6 pt-3 pb-4
                bg-white/95 backdrop-blur border-t border-gray-100">
      <div class="flex items-center gap-4">
        <div class="flex-1 min-w-0">
          <p class="text-xs text-ink-500">{m.cart_summary_total()}</p>
          <p class="font-display font-bold text-lg tabular-nums text-ink-900">{formatHKD(cartStore.subtotal)}</p>
        </div>
        <a href="/checkout"
           class="shrink-0 px-8 py-3 bg-navy-500 hover:bg-navy-700 text-white
                  font-display font-bold uppercase tracking-[0.12em] text-sm rounded-xl transition-colors">
          {m.cart_checkout()}
        </a>
      </div>
      <a href="/products"
         class="block text-center text-xs text-ink-500 hover:text-ink-900 transition-colors mt-2">
        {m.cart_continue_shopping_back()}
      </a>
    </div>
  {/if}
</div>

<RecentlyViewed />
