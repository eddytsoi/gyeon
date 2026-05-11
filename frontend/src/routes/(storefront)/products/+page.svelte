<script lang="ts">
  import type { PageData } from './$types';
  import { goto } from '$app/navigation';
  import { page } from '$app/state';
  import ProductCard from '$lib/components/shop/ProductCard.svelte';
  import ProductCardSkeleton from '$lib/components/shop/ProductCardSkeleton.svelte';
  import Eyebrow from '$lib/components/shop/Eyebrow.svelte';
  import * as m from '$lib/paraglide/messages';
  import Seo from '$lib/components/Seo.svelte';
  import { siteOrigin } from '$lib/seo';
  import { getProductsFiltered, type ProductListFilters } from '$lib/api';
  import type { Product, ProductImage, Variant } from '$lib/types';

  let { data }: { data: PageData } = $props();

  const BATCH_SIZE = 6;

  let searchInput = $state(data.q);
  let minPriceInput = $state(data.minPrice != null ? String(data.minPrice) : '');
  let maxPriceInput = $state(data.maxPrice != null ? String(data.maxPrice) : '');
  let mobileFiltersOpen = $state(false);

  let items = $state<Product[]>(data.products);
  let loadingMore = $state(false);
  let hasMore = $state(data.products.length < data.total);
  let sentinel = $state<HTMLDivElement | undefined>();
  let abortCtl: AbortController | null = null;

  // When SSR data changes (filter goto), reset list + drop any in-flight fetch.
  $effect(() => {
    items = data.products;
    hasMore = data.products.length < data.total;
    abortCtl?.abort();
    abortCtl = null;
  });

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

  function currentFilters(): ProductListFilters {
    return {
      limit: BATCH_SIZE,
      offset: items.length,
      search: data.q || undefined,
      category: data.category || undefined,
      minPrice: data.minPrice ?? undefined,
      maxPrice: data.maxPrice ?? undefined,
      sort: data.sort
    };
  }

  // True when the sentinel is still close enough to the viewport that another
  // batch should be loaded. After a fast scroll the user is sitting at the
  // page bottom and the IO callback only fires once on entry, so we re-check
  // post-append to keep loading until the document is long enough that the
  // sentinel sits comfortably below the viewport+rootMargin region.
  function shouldKeepLoading(): boolean {
    if (!sentinel) return false;
    const rect = sentinel.getBoundingClientRect();
    return rect.top - window.innerHeight < 600;
  }

  async function loadMore() {
    if (loadingMore || !hasMore) return;
    loadingMore = true;
    abortCtl?.abort();
    abortCtl = new AbortController();
    const ctl = abortCtl;
    try {
      const next = await getProductsFiltered(currentFilters(), { signal: ctl.signal });
      // Guard against a filter-reset that happened while we were awaiting.
      if (ctl.signal.aborted) return;
      items = [...items, ...next];
      if (next.length < BATCH_SIZE) hasMore = false;
    } catch (e: unknown) {
      if ((e as { name?: string })?.name !== 'AbortError') hasMore = false;
    } finally {
      if (!ctl.signal.aborted) loadingMore = false;
    }
    // After append: if the user already scrolled past where the sentinel
    // landed (or the sentinel is still within rootMargin), recurse on the
    // next frame. requestAnimationFrame ensures layout has settled.
    if (hasMore && !ctl.signal.aborted) {
      requestAnimationFrame(() => {
        if (hasMore && !loadingMore && shouldKeepLoading()) loadMore();
      });
    }
  }

  $effect(() => {
    if (!sentinel) return;
    const io = new IntersectionObserver(
      (entries) => { if (entries[0].isIntersecting) loadMore(); },
      { rootMargin: '600px' }
    );
    io.observe(sentinel);
    return () => {
      io.disconnect();
      abortCtl?.abort();
    };
  });

  // Adapt list-row product into ProductCard's image/variant props (kept
  // backward-compatible so the card component is unchanged for other consumers).
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
    if (p.min_price == null) return undefined;
    return {
      id: p.default_variant_id ?? '',
      product_id: p.id,
      sku: '',
      price: p.min_price,
      compare_at_price: p.min_compare_at_price ?? undefined,
      stock_qty: p.min_price_stock_qty ?? 0,
      is_active: true
    };
  }
</script>

<Seo
  title={m.products_title()}
  description={m.products_meta_description()}
  canonical={`${siteOrigin(page.data.publicSettings)}/products`}
/>

<div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-10 md:py-14">
  <div class="flex items-end justify-between gap-4 mb-8 md:mb-10">
    <header>
      <Eyebrow class="mb-2">Listing</Eyebrow>
      <h1 class="font-display text-[1.575rem] md:text-[2.1rem] lg:text-[2.625rem] font-bold tracking-tight text-ink-900 leading-none">
        {m.products_heading_all()}
      </h1>
      <p class="mt-3 text-sm text-ink-500" aria-live="polite">
        {data.total === 1 ? m.products_count_one({ count: data.total }) : m.products_count_many({ count: data.total })}
      </p>
      <div class="mt-4 h-px w-12 bg-navy-500"></div>
    </header>
    <button class="md:hidden inline-flex items-center gap-1.5 px-4 py-2 rounded-md border border-ink-300
                   text-[11px] font-display font-semibold uppercase tracking-[0.15em] text-ink-900
                   hover:border-navy-500 hover:text-navy-500 transition-colors"
            onclick={() => (mobileFiltersOpen = !mobileFiltersOpen)}
            aria-label={m.products_filters_button_aria()}
            aria-expanded={mobileFiltersOpen}
            aria-controls="storefront-product-filters">
      {m.products_filters_button()}
    </button>
  </div>

  <!-- Search bar -->
  <div class="relative mb-8">
    <svg class="absolute left-3.5 top-1/2 -translate-y-1/2 w-4 h-4 text-ink-500"
         aria-hidden="true"
         fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
      <path stroke-linecap="round" stroke-linejoin="round"
            d="m21 21-4.34-4.34m0 0A7.5 7.5 0 1 0 6.075 6.075a7.5 7.5 0 0 0 10.585 10.585Z"/>
    </svg>
    <input type="search" bind:value={searchInput} oninput={onSearchInput}
           placeholder={m.products_search_placeholder()}
           aria-label={m.products_search_aria()}
           class="w-full pl-10 pr-4 py-3 rounded-md border border-ink-300 bg-white text-sm font-body
                  focus:outline-none focus:ring-2 focus:ring-navy-300 focus:border-transparent" />
  </div>

  <div class="grid grid-cols-1 md:grid-cols-[220px_1fr] gap-8 lg:gap-12">
    <!-- Filters -->
    <aside id="storefront-product-filters"
           aria-labelledby="storefront-filters-heading"
           class="{mobileFiltersOpen ? 'block' : 'hidden'} md:block space-y-8">
      <h2 id="storefront-filters-heading" class="sr-only">{m.products_filters_section_heading()}</h2>
      <!-- Category -->
      <div>
        <h3 class="text-[11px] font-display font-semibold text-navy-500 uppercase tracking-[0.18em] mb-3">
          {m.products_filter_category()}
        </h3>
        <div class="space-y-1">
          <button onclick={() => onCategoryChange('')}
                  aria-pressed={data.category === ''}
                  class="block w-full text-left text-sm font-body py-1.5 transition-colors
                         {data.category === '' ? 'font-semibold text-navy-500' : 'text-ink-900/80 hover:text-navy-500'}">
            {m.products_filter_category_all()}
          </button>
          {#each data.categories.filter(c => c.is_active) as cat}
            <button onclick={() => onCategoryChange(cat.slug)}
                    aria-pressed={data.category === cat.slug}
                    class="block w-full text-left text-sm font-body py-1.5 transition-colors
                           {data.category === cat.slug ? 'font-semibold text-navy-500' : 'text-ink-900/80 hover:text-navy-500'}">
              {cat.name}
            </button>
          {/each}
        </div>
      </div>

      <!-- Price -->
      <div>
        <h3 class="text-[11px] font-display font-semibold text-navy-500 uppercase tracking-[0.18em] mb-3">
          {m.products_filter_price()}
        </h3>
        <div class="grid grid-cols-2 gap-2">
          <input type="number" min="0" step="1" bind:value={minPriceInput}
                 placeholder={m.products_filter_price_min()}
                 aria-label={m.products_filter_price_min_aria()}
                 class="w-full px-2.5 py-1.5 rounded-sm border border-ink-300 text-sm font-body
                        focus:outline-none focus:ring-2 focus:ring-navy-300" />
          <input type="number" min="0" step="1" bind:value={maxPriceInput}
                 placeholder={m.products_filter_price_max()}
                 aria-label={m.products_filter_price_max_aria()}
                 class="w-full px-2.5 py-1.5 rounded-sm border border-ink-300 text-sm font-body
                        focus:outline-none focus:ring-2 focus:ring-navy-300" />
        </div>
        <button onclick={applyPrice}
                class="mt-3 w-full text-[11px] font-display font-bold uppercase tracking-[0.15em]
                       text-navy-500 px-3 py-2 rounded-sm border border-navy-500
                       hover:bg-navy-500 hover:text-white transition-colors">
          {m.products_filter_price_apply()}
        </button>
      </div>

    </aside>

    <!-- Results -->
    <section aria-labelledby="storefront-results-heading">
      <h2 id="storefront-results-heading" class="sr-only">{m.products_results_section_heading()}</h2>

      <!-- Active filter chips + sort row (gyeon-project-design-system §3.2) -->
      <div class="flex flex-wrap items-center justify-between gap-3 py-4 mb-8">
        <ul class="flex flex-wrap items-center gap-2 min-h-[28px]">
          {#if data.category}
            {@const cat = data.categories.find((c) => c.slug === data.category)}
            <li>
              <button onclick={() => onCategoryChange('')}
                      class="inline-flex items-center gap-1.5 text-[11px] font-display font-semibold uppercase tracking-[0.15em]
                             text-ink-900 px-3 py-1.5 rounded-full bg-paper hover:bg-cream transition-colors">
                {cat?.name ?? data.category}
                <span aria-hidden="true" class="text-ink-500">×</span>
                <span class="sr-only">Remove filter</span>
              </button>
            </li>
          {/if}
          {#if data.q}
            <li>
              <button onclick={() => { searchInput = ''; navigate((p) => p.delete('q')); }}
                      class="inline-flex items-center gap-1.5 text-[11px] font-display font-semibold uppercase tracking-[0.15em]
                             text-ink-900 px-3 py-1.5 rounded-full bg-paper hover:bg-cream transition-colors">
                "{data.q}"
                <span aria-hidden="true" class="text-ink-500">×</span>
                <span class="sr-only">Remove search</span>
              </button>
            </li>
          {/if}
          {#if data.minPrice != null || data.maxPrice != null}
            <li>
              <button onclick={() => { minPriceInput = ''; maxPriceInput = ''; navigate((p) => { p.delete('min_price'); p.delete('max_price'); }); }}
                      class="inline-flex items-center gap-1.5 text-[11px] font-display font-semibold uppercase tracking-[0.15em]
                             text-ink-900 px-3 py-1.5 rounded-full bg-paper hover:bg-cream transition-colors tabular-nums">
                HK${data.minPrice ?? 0}–{data.maxPrice ?? '∞'}
                <span aria-hidden="true" class="text-ink-500">×</span>
                <span class="sr-only">Remove price range</span>
              </button>
            </li>
          {/if}
          {#if hasFilters}
            <li>
              <button onclick={clearAll}
                      class="text-[11px] font-display font-semibold uppercase tracking-[0.15em]
                             text-navy-500 hover:text-navy-700 underline underline-offset-4 px-1">
                {m.products_filter_clear_all()}
              </button>
            </li>
          {/if}
        </ul>
        <div class="flex items-center gap-3">
          <span class="text-[11px] font-display uppercase tracking-[0.15em] text-ink-500">Sort</span>
          <select value={data.sort ?? 'new'}
                  onchange={(e) => onSortChange((e.currentTarget as HTMLSelectElement).value)}
                  aria-label={m.products_sort_aria()}
                  class="font-display font-medium text-sm border-0 bg-transparent
                         focus:outline-none focus:ring-0 cursor-pointer text-ink-900">
            <option value="new">{m.products_sort_new()}</option>
            <option value="price_asc">{m.products_sort_price_asc()}</option>
            <option value="price_desc">{m.products_sort_price_desc()}</option>
            <option value="name">{m.products_sort_name()}</option>
          </select>
        </div>
      </div>

      <!-- Polite live announcement: screen readers hear when a new batch
           appended (items.length changed) or when "loading more" begins. -->
      <div class="sr-only" role="status" aria-live="polite">
        {#if loadingMore}
          {m.products_loading_more()}
        {:else if items.length > 0}
          {m.products_loaded_announcement({ shown: items.length, total: data.total })}
        {/if}
      </div>

      {#if items.length === 0 && !loadingMore}
        <div class="text-center py-24 text-ink-500">
          {#if data.q}
            <p class="text-base font-body">{m.products_no_match({ query: data.q })}</p>
          {:else}
            <p class="text-base font-body">{m.products_empty()}</p>
          {/if}
          {#if hasFilters}
            <button onclick={clearAll}
                    class="mt-4 text-sm text-navy-500 underline underline-offset-4">
              {m.products_filter_clear_all()}
            </button>
          {/if}
        </div>
      {:else}
        <ul class="grid grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-x-4 gap-y-10 md:gap-x-6 md:gap-y-12">
          {#each items as p, i (p.id)}
            <li>
              <ProductCard
                product={p}
                image={imageOf(p)}
                variant={variantOf(p)}
                loading={i < 3 ? 'eager' : 'lazy'}
                fetchpriority={i < 3 ? 'high' : 'auto'}
              />
            </li>
          {/each}
          {#if loadingMore}
            {#each Array(BATCH_SIZE) as _, i (`sk-${i}`)}
              <li><ProductCardSkeleton /></li>
            {/each}
          {/if}
        </ul>

        {#if hasMore}
          <div bind:this={sentinel} class="h-1 mt-8" aria-hidden="true"></div>
        {/if}
      {/if}
    </section>
  </div>
</div>
