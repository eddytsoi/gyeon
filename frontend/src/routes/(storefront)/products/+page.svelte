<script lang="ts">
  import type { PageData } from './$types';
  import ProductCard from '$lib/components/shop/ProductCard.svelte';
  import * as m from '$lib/paraglide/messages';

  let { data }: { data: PageData } = $props();
</script>

<svelte:head>
  <title>{m.products_title()}</title>
</svelte:head>

<div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-10">
  <h1 class="text-3xl font-bold text-gray-900 mb-8">{m.products_heading_all()}</h1>

  {#if data.products.length === 0}
    <div class="text-center py-24 text-gray-400">
      <p class="text-xl">{m.products_empty()}</p>
    </div>
  {:else}
    <div class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-4 md:gap-6">
      {#each data.products as item}
        <ProductCard
          product={item.product}
          image={item.primaryImage}
          variant={item.cheapestVariant}
        />
      {/each}
    </div>

    <!-- Pagination -->
    <div class="mt-12 flex justify-center gap-4">
      {#if data.offset > 0}
        <a href="/products?offset={data.offset - data.limit}"
           class="px-6 py-2 border border-gray-300 rounded-full text-sm font-medium
                  text-gray-700 hover:border-gray-900 transition-colors">
          {m.common_previous_arrow()}
        </a>
      {/if}
      {#if data.products.length === data.limit}
        <a href="/products?offset={data.offset + data.limit}"
           class="px-6 py-2 border border-gray-300 rounded-full text-sm font-medium
                  text-gray-700 hover:border-gray-900 transition-colors">
          {m.common_next_arrow()}
        </a>
      {/if}
    </div>
  {/if}
</div>
