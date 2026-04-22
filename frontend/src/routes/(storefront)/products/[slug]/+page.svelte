<script lang="ts">
  import type { PageData } from './$types';
  import type { ProductImage } from '$lib/types';
  import { cartStore } from '$lib/stores/cart.svelte';

  let { data }: { data: PageData } = $props();

  // undefined initial state avoids Svelte 5 "state_referenced_locally" warning;
  // $derived handles the default fallback to first item / primary image.
  let selectedVariantID = $state<string | undefined>(undefined);
  let activeImageID = $state<string | undefined>(undefined);

  const selectedVariant = $derived(
    data.variants.find((v) => v.id === selectedVariantID) ?? data.variants[0]
  );
  const activeImage = $derived<ProductImage | undefined>(
    data.images.find((i) => i.id === activeImageID) ??
    data.images.find((i) => i.is_primary) ??
    data.images[0]
  );

  let qty = $state(1);
  let adding = $state(false);
  let added = $state(false);

  const inStock = $derived((selectedVariant?.stock_qty ?? 0) > 0);
  const hasDiscount = $derived(
    selectedVariant?.compare_at_price != null &&
    selectedVariant.compare_at_price > selectedVariant.price
  );

  async function addToCart() {
    if (!selectedVariant || !inStock) return;
    adding = true;
    try {
      await cartStore.add(selectedVariant.id, qty);
      added = true;
      setTimeout(() => (added = false), 2000);
    } finally {
      adding = false;
    }
  }
</script>

<svelte:head>
  <title>{data.product.name} — Gyeon</title>
</svelte:head>

<div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-10">
  <!-- Breadcrumb -->
  <nav class="text-sm text-gray-400 mb-6 flex gap-2 items-center">
    <a href="/" class="hover:text-gray-700">Home</a>
    <span>/</span>
    <a href="/products" class="hover:text-gray-700">Products</a>
    <span>/</span>
    <span class="text-gray-700">{data.product.name}</span>
  </nav>

  <div class="grid grid-cols-1 md:grid-cols-2 gap-10 lg:gap-16">

    <!-- Images -->
    <div class="flex flex-col gap-3">
      <div class="aspect-square rounded-2xl overflow-hidden bg-gray-50">
        {#if activeImage}
          <img src={activeImage.url} alt={activeImage.alt_text ?? data.product.name}
               class="w-full h-full object-cover" />
        {:else}
          <div class="w-full h-full flex items-center justify-center text-gray-200">
            <svg xmlns="http://www.w3.org/2000/svg" class="h-24 w-24" fill="none"
                 viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1"
                d="m2.25 15.75 5.159-5.159a2.25 2.25 0 0 1 3.182 0l5.159 5.159m-1.5-1.5
                   1.409-1.409a2.25 2.25 0 0 1 3.182 0l2.909 2.909m-18 3.75h16.5a1.5 1.5
                   0 0 0 1.5-1.5V6a1.5 1.5 0 0 0-1.5-1.5H3.75A1.5 1.5 0 0 0 2.25 6v12a1.5
                   1.5 0 0 0 1.5 1.5Zm10.5-11.25h.008v.008h-.008V8.25Z" />
            </svg>
          </div>
        {/if}
      </div>

      {#if data.images.length > 1}
        <div class="flex gap-2 overflow-x-auto pb-1">
          {#each data.images as img}
            <button
              onclick={() => activeImageID = img.id}
              class="flex-shrink-0 w-16 h-16 rounded-lg overflow-hidden border-2 transition-colors
                     {activeImageID === img.id ? 'border-gray-900' : 'border-transparent'}">
              <img src={img.url} alt={img.alt_text ?? ''} class="w-full h-full object-cover" />
            </button>
          {/each}
        </div>
      {/if}
    </div>

    <!-- Details -->
    <div class="flex flex-col gap-6">
      <div>
        <h1 class="text-3xl font-bold text-gray-900">{data.product.name}</h1>
        {#if data.product.description}
          <p class="mt-3 text-gray-600 leading-relaxed">{data.product.description}</p>
        {/if}
      </div>

      <!-- Price -->
      {#if selectedVariant}
        <div class="flex items-baseline gap-3">
          <span class="text-3xl font-bold text-gray-900">
            HK${selectedVariant.price.toFixed(2)}
          </span>
          {#if hasDiscount}
            <span class="text-xl text-gray-400 line-through">
              HK${selectedVariant.compare_at_price!.toFixed(2)}
            </span>
          {/if}
        </div>
      {/if}

      <!-- Variant selector -->
      {#if data.variants.length > 1}
        <div>
          <p class="text-sm font-medium text-gray-700 mb-2">Variant</p>
          <div class="flex flex-wrap gap-2">
            {#each data.variants as v}
              <button
                onclick={() => selectedVariantID = v.id}
                class="px-4 py-2 rounded-lg border text-sm font-medium transition-colors
                       {selectedVariantID === v.id
                         ? 'border-gray-900 bg-gray-900 text-white'
                         : 'border-gray-200 text-gray-700 hover:border-gray-400'}
                       {v.stock_qty === 0 ? 'opacity-40 cursor-not-allowed' : ''}">
                {v.sku}
              </button>
            {/each}
          </div>
        </div>
      {/if}

      <!-- Qty + Add to cart -->
      <div class="flex flex-col sm:flex-row gap-3">
        <div class="flex items-center border border-gray-200 rounded-xl overflow-hidden">
          <button onclick={() => qty = Math.max(1, qty - 1)}
                  class="w-11 h-11 flex items-center justify-center text-gray-500
                         hover:bg-gray-50 transition-colors">−</button>
          <span class="w-10 text-center text-sm font-medium">{qty}</span>
          <button onclick={() => qty = qty + 1}
                  class="w-11 h-11 flex items-center justify-center text-gray-500
                         hover:bg-gray-50 transition-colors">+</button>
        </div>

        <button
          onclick={addToCart}
          disabled={!inStock || adding}
          class="flex-1 py-3 px-6 rounded-xl font-semibold text-sm transition-all
                 {inStock
                   ? added
                     ? 'bg-green-600 text-white'
                     : 'bg-gray-900 text-white hover:bg-gray-700'
                   : 'bg-gray-100 text-gray-400 cursor-not-allowed'}">
          {#if !inStock}
            Out of Stock
          {:else if adding}
            Adding…
          {:else if added}
            ✓ Added to Cart
          {:else}
            Add to Cart
          {/if}
        </button>
      </div>

      {#if inStock && selectedVariant}
        <p class="text-xs text-gray-400">{selectedVariant.stock_qty} in stock</p>
      {/if}
    </div>
  </div>
</div>
