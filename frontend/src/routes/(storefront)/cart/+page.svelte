<script lang="ts">
  import { cartStore } from '$lib/stores/cart.svelte';
  import RecentlyViewed from '$lib/components/shop/RecentlyViewed.svelte';
  import FreeShippingBanner from '$lib/components/shop/FreeShippingBanner.svelte';
  import ResponsiveImage from '$lib/components/ResponsiveImage.svelte';
  import * as m from '$lib/paraglide/messages';
  import type { PageData } from './$types';

  // publicSettings flows in from the (storefront) layout load.
  let { data }: { data: PageData } = $props();

  const freeShippingThreshold = $derived(() => {
    const raw = (data.publicSettings ?? []).find((s) => s.key === 'free_shipping_threshold_hkd')?.value;
    const n = raw ? Number(raw) : 0;
    return Number.isFinite(n) && n > 0 ? n : 0;
  });
  const shippingFree = $derived(
    freeShippingThreshold() > 0 && cartStore.subtotal >= freeShippingThreshold()
  );
</script>

<svelte:head>
  <title>{m.cart_title()}</title>
</svelte:head>

<div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-10">
  <h1 class="text-3xl font-bold text-gray-900 mb-6">{m.cart_heading()}</h1>

  <div class="mb-6">
    <FreeShippingBanner settings={data.publicSettings ?? []} showUnlocked />
  </div>

  {#if cartStore.loading && !cartStore.cart}
    <div class="text-center py-20 text-gray-400">{m.common_loading()}</div>

  {:else if !cartStore.cart || cartStore.cart.items.length === 0}
    <div class="text-center py-20">
      <p class="text-xl text-gray-400">{m.cart_empty()}</p>
      <a href="/products"
         class="mt-4 inline-block bg-gray-900 text-white font-medium px-8 py-3
                rounded-full hover:bg-gray-700 transition-colors">
        {m.cart_continue_shopping()}
      </a>
    </div>

  {:else}
    <div class="flex flex-col lg:flex-row gap-10">

      <!-- Items -->
      <div class="flex-1 flex flex-col gap-4">
        {#each cartStore.cart.items as item}
          <div class="flex items-center gap-4 bg-white rounded-2xl p-4 border border-gray-100">
            <a href="/products/{item.product_slug}?variant={encodeURIComponent(item.sku)}"
               class="w-16 h-16 rounded-lg bg-gray-50 flex-shrink-0 overflow-hidden block hover:opacity-80 transition-opacity">
              {#if item.image_url}
                <ResponsiveImage src={item.image_url} alt={item.product_name}
                                 widths={[160, 320]} sizes="64px"
                                 class="w-full h-full object-cover" />
              {/if}
            </a>

            <div class="flex-1 min-w-0">
              <a href="/products/{item.product_slug}?variant={encodeURIComponent(item.sku)}"
                 class="text-sm font-medium text-gray-900 truncate block hover:text-gray-600 transition-colors">
                {m.cart_item_label({ name: item.product_name, sku: item.sku })}
              </a>
              <p class="text-xs text-gray-400 mt-0.5">{m.cart_item_sku({ sku: item.sku })}</p>
            </div>

            <!-- Qty controls -->
            <div class="flex items-center border border-gray-200 rounded-lg overflow-hidden">
              <button
                type="button"
                onclick={() => cartStore.update(item.id, item.quantity - 1)}
                aria-label={m.common_aria_decrease_quantity()}
                disabled={item.quantity <= 1}
                class="w-8 h-8 flex items-center justify-center text-gray-500 hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed">−</button>
              <span class="w-8 text-center text-sm">{item.quantity}</span>
              <button
                type="button"
                onclick={() => cartStore.update(item.id, item.quantity + 1)}
                aria-label={m.common_aria_increase_quantity()}
                class="w-8 h-8 flex items-center justify-center text-gray-500 hover:bg-gray-50">+</button>
            </div>

            <button
              onclick={() => cartStore.remove(item.id)}
              class="p-2 text-gray-300 hover:text-red-400 transition-colors"
              aria-label={m.cart_aria_remove()}>
              <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none"
                   viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12"/>
              </svg>
            </button>
          </div>
        {/each}
      </div>

      <!-- Summary -->
      <div class="lg:w-72 flex-shrink-0">
        <div class="bg-white rounded-2xl border border-gray-100 p-6 flex flex-col gap-4 sticky top-24">
          <h2 class="font-semibold text-gray-900">{m.cart_summary_heading()}</h2>

          <div class="flex justify-between text-sm text-gray-600">
            <span>{m.cart_summary_items({ count: cartStore.itemCount })}</span>
            <span>HK${cartStore.subtotal.toFixed(2)}</span>
          </div>
          <div class="flex justify-between text-sm text-gray-600">
            <span>{m.cart_summary_shipping()}</span>
            <span class="whitespace-nowrap {shippingFree ? 'text-green-600' : 'text-gray-900'}">
              {shippingFree ? m.shipping_sf_free() : m.shipping_sf_cod()}
            </span>
          </div>
          <div class="border-t border-gray-100 pt-3 flex justify-between font-semibold text-gray-900">
            <span>{m.cart_summary_total()}</span>
            <span>HK${cartStore.subtotal.toFixed(2)}</span>
          </div>

          <a
            href="/checkout"
            class="w-full py-3 bg-gray-900 text-white font-semibold rounded-xl
                   hover:bg-gray-700 transition-colors text-center block">
            {m.cart_checkout()}
          </a>

          <a href="/products" class="text-center text-sm text-gray-400 hover:text-gray-700 transition-colors">
            {m.cart_continue_shopping_back()}
          </a>
        </div>
      </div>
    </div>
  {/if}
</div>

<RecentlyViewed />
