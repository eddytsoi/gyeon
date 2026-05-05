<script lang="ts">
  import type { PageData } from './$types';
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import ProductCard from '$lib/components/shop/ProductCard.svelte';
  import * as m from '$lib/paraglide/messages';
  import Seo from '$lib/components/Seo.svelte';
  import { siteOrigin } from '$lib/seo';

  let { data }: { data: PageData } = $props();

  let searchInput = $state(data.q);
  let minPriceInput = $state(data.minPrice != null ? String(data.minPrice) : '');
  let maxPriceInput = $state(data.maxPrice != null ? String(data.maxPrice) : '');
  let mobileFiltersOpen = $state(false);

  function navigate(updater: (params: URLSearchParams) => void) {
    const params = new URLSearchParams(page.url.searchParams);
    params.delete('offset');
    updater(params);
    goto(`/products${params.toString() ? '?' + params.toString() : ''}`,
      { keepFocus: true, noScroll: true });
  }

  let searchTimeout: ReturnType<typeof setTimeout> | undefined;
  function onSearchInput() {
    clearTimeout(searchTimeout);
    searchTimeout = setTimeout(() => {
      navigate(p => {
        if (searchInput) p.set('q', searchInput);
        else p.delete('q');
      });
    }, 300);
  }

  function onCategoryChange(slug: string) {
    navigate(p => {
      if (slug) p.set('category', slug);
      else p.delete('category');
    });
  }

  function onSortChange(sort: string) {
    navigate(p => {
      if (sort && sort !== 'new') p.set('sort', sort);
      else p.delete('sort');
    });
  }

  function applyPrice() {
    navigate(p => {
      if (minPriceInput) p.set('min_price', minPriceInput); else p.delete('min_price');
      if (maxPriceInput) p.set('max_price', maxPriceInput); else p.delete('max_price');
    });
  }

  function clearAll() {
    searchInput = '';
    minPriceInput = '';
    maxPriceInput = '';
    goto('/products', { noScroll: true });
  }

  const hasFilters = $derived(!!(data.q || data.category || data.minPrice != null || data.maxPrice != null || (data.sort && data.sort !== 'new')));
</script>

<Seo
  title={m.products_title()}
  description={m.products_meta_description()}
  canonical={`${siteOrigin(page.data.publicSettings)}/products`}
/>

<div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-10">
  <div class="flex items-center justify-between gap-4 mb-6">
    <h1 class="text-3xl font-bold text-gray-900">{m.products_heading_all()}</h1>
    <button class="md:hidden inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full border border-gray-300 text-sm"
            onclick={() => (mobileFiltersOpen = !mobileFiltersOpen)}>
      {m.products_filters_button()}
    </button>
  </div>

  <!-- Search bar -->
  <div class="relative mb-6">
    <svg class="absolute left-3.5 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400"
         fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
      <path stroke-linecap="round" stroke-linejoin="round"
            d="m21 21-4.34-4.34m0 0A7.5 7.5 0 1 0 6.075 6.075a7.5 7.5 0 0 0 10.585 10.585Z"/>
    </svg>
    <input type="text" bind:value={searchInput} oninput={onSearchInput}
           placeholder={m.products_search_placeholder()}
           class="w-full pl-10 pr-4 py-3 rounded-full border border-gray-200 bg-white text-sm
                  focus:outline-none focus:ring-2 focus:ring-gray-900 focus:border-transparent" />
  </div>

  <div class="grid grid-cols-1 md:grid-cols-[220px_1fr] gap-8">
    <!-- Filters -->
    <aside class="{mobileFiltersOpen ? 'block' : 'hidden'} md:block space-y-6">
      <!-- Category -->
      <div>
        <h3 class="text-xs font-semibold text-gray-500 uppercase tracking-wide mb-3">{m.products_filter_category()}</h3>
        <div class="space-y-1.5">
          <button onclick={() => onCategoryChange('')}
                  class="block w-full text-left text-sm px-2 py-1 rounded
                         {data.category === '' ? 'font-semibold text-gray-900' : 'text-gray-600 hover:text-gray-900'}">
            {m.products_filter_category_all()}
          </button>
          {#each data.categories.filter(c => c.is_active) as cat}
            <button onclick={() => onCategoryChange(cat.slug)}
                    class="block w-full text-left text-sm px-2 py-1 rounded
                           {data.category === cat.slug ? 'font-semibold text-gray-900' : 'text-gray-600 hover:text-gray-900'}">
              {cat.name}
            </button>
          {/each}
        </div>
      </div>

      <!-- Price -->
      <div>
        <h3 class="text-xs font-semibold text-gray-500 uppercase tracking-wide mb-3">{m.products_filter_price()}</h3>
        <div class="grid grid-cols-2 gap-2">
          <input type="number" min="0" step="1" bind:value={minPriceInput}
                 placeholder={m.products_filter_price_min()}
                 class="w-full px-2.5 py-1.5 rounded-lg border border-gray-200 text-sm
                        focus:outline-none focus:ring-2 focus:ring-gray-900" />
          <input type="number" min="0" step="1" bind:value={maxPriceInput}
                 placeholder={m.products_filter_price_max()}
                 class="w-full px-2.5 py-1.5 rounded-lg border border-gray-200 text-sm
                        focus:outline-none focus:ring-2 focus:ring-gray-900" />
        </div>
        <button onclick={applyPrice}
                class="mt-2 w-full text-xs font-medium text-gray-700 px-3 py-1.5 rounded-lg
                       border border-gray-200 hover:bg-gray-50">
          {m.products_filter_price_apply()}
        </button>
      </div>

      {#if hasFilters}
        <button onclick={clearAll}
                class="text-xs text-gray-500 hover:text-gray-900 underline underline-offset-2">
          {m.products_filter_clear_all()}
        </button>
      {/if}
    </aside>

    <!-- Results -->
    <div>
      <div class="flex items-center justify-between mb-4">
        <p class="text-sm text-gray-500">
          {data.products.length === 1 ? m.products_count_one({ count: data.products.length }) : m.products_count_many({ count: data.products.length })}
        </p>
        <select value={data.sort ?? 'new'}
                onchange={(e) => onSortChange((e.currentTarget as HTMLSelectElement).value)}
                class="px-3 py-1.5 rounded-lg border border-gray-200 bg-white text-sm
                       focus:outline-none focus:ring-2 focus:ring-gray-900">
          <option value="new">{m.products_sort_new()}</option>
          <option value="price_asc">{m.products_sort_price_asc()}</option>
          <option value="price_desc">{m.products_sort_price_desc()}</option>
          <option value="name">{m.products_sort_name()}</option>
        </select>
      </div>

      {#if data.products.length === 0}
        <div class="text-center py-24 text-gray-400">
          {#if data.q}
            <p class="text-base">{m.products_no_match({ query: data.q })}</p>
          {:else}
            <p class="text-base">{m.products_empty()}</p>
          {/if}
          {#if hasFilters}
            <button onclick={clearAll}
                    class="mt-4 text-sm text-gray-700 underline underline-offset-2">
              {m.products_filter_clear_all()}
            </button>
          {/if}
        </div>
      {:else}
        <div class="grid grid-cols-2 sm:grid-cols-3 gap-4 md:gap-6">
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
            <a href="/products?{new URLSearchParams({ ...Object.fromEntries(page.url.searchParams), offset: String(data.offset - data.limit) }).toString()}"
               class="px-6 py-2 border border-gray-300 rounded-full text-sm font-medium
                      text-gray-700 hover:border-gray-900 transition-colors">
              {m.common_previous_arrow()}
            </a>
          {/if}
          {#if data.products.length === data.limit}
            <a href="/products?{new URLSearchParams({ ...Object.fromEntries(page.url.searchParams), offset: String(data.offset + data.limit) }).toString()}"
               class="px-6 py-2 border border-gray-300 rounded-full text-sm font-medium
                      text-gray-700 hover:border-gray-900 transition-colors">
              {m.common_next_arrow()}
            </a>
          {/if}
        </div>
      {/if}
    </div>
  </div>
</div>
