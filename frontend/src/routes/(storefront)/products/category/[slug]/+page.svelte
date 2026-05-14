<script lang="ts">
  import type { PageData } from './$types';
  import ProductCard from '$lib/components/shop/ProductCard.svelte';
  import ProductCardSkeleton from '$lib/components/shop/ProductCardSkeleton.svelte';
  import ResponsiveImage from '$lib/components/ResponsiveImage.svelte';
  import * as m from '$lib/paraglide/messages';
  import { getProductsFiltered, type ProductListFilters } from '$lib/api';
  import type { Product, ProductImage, Variant } from '$lib/types';

  let { data }: { data: PageData } = $props();

  const BATCH_SIZE = 6;

  let items = $state<Product[]>(data.products);
  let loadingMore = $state(false);
  let hasMore = $state(data.products.length < data.total);
  let sentinel = $state<HTMLDivElement | undefined>();
  let abortCtl: AbortController | null = null;

  // Reset when the slug changes (navigating between categories).
  $effect(() => {
    items = data.products;
    hasMore = data.products.length < data.total;
    abortCtl?.abort();
    abortCtl = null;
  });

  function currentFilters(): ProductListFilters {
    return {
      limit: BATCH_SIZE,
      offset: items.length,
      category: data.category.slug
    };
  }

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
      if (ctl.signal.aborted) return;
      items = [...items, ...next];
      if (next.length < BATCH_SIZE) hasMore = false;
    } catch (e: unknown) {
      if ((e as { name?: string })?.name !== 'AbortError') hasMore = false;
    } finally {
      if (!ctl.signal.aborted) loadingMore = false;
    }
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

  // Adapt list-row product into ProductCard's image/variant props — mirrors
  // the helpers on /products so we don't need N+1 variant/image fetches.
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

<svelte:head>
  <title>{m.products_category_title({ name: data.category.name })}</title>
</svelte:head>

<div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-10">
  <!-- Breadcrumbs -->
  <nav class="flex gap-2 items-center text-[11px] uppercase tracking-[0.15em] text-gray-400 mb-6">
    <a href="/" class="hover:text-gray-700 transition-colors">{m.common_home()}</a>
    <span>/</span>
    <a href="/products" class="hover:text-gray-700 transition-colors">{m.common_products()}</a>
    <span>/</span>
    <span class="font-semibold text-gray-700">{data.category.name}</span>
  </nav>

  {#if data.category.desktop_banner_url || data.category.mobile_banner_url}
    <div class="-mx-4 sm:mx-0 mb-8 rounded-none sm:rounded-2xl overflow-hidden">
      {#if data.category.mobile_banner_url}
        <ResponsiveImage src={data.category.mobile_banner_url} alt={data.category.name}
                         widths={[480, 768]} sizes="100vw"
                         loading="eager" fetchpriority="high"
                         class="w-full sm:hidden" />
      {/if}
      {#if data.category.desktop_banner_url}
        <ResponsiveImage src={data.category.desktop_banner_url} alt={data.category.name}
                         widths={[960, 1280, 1920]} sizes="100vw"
                         loading="eager" fetchpriority="high"
                         class="w-full hidden sm:block" />
      {/if}
    </div>
  {/if}

  <h1 class="text-3xl font-bold text-gray-900 mb-8">{data.category.name}</h1>

  <!-- Polite live announcement for screen readers as new batches append. -->
  <div class="sr-only" role="status" aria-live="polite">
    {#if loadingMore}
      {m.products_loading_more()}
    {:else if items.length > 0}
      {m.products_loaded_announcement({ shown: items.length, total: data.total })}
    {/if}
  </div>

  {#if items.length === 0 && !loadingMore}
    <div class="text-center py-24 text-gray-400">
      <p class="text-xl">{m.products_category_empty()}</p>
    </div>
  {:else}
    <ul class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-4 md:gap-6">
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
</div>
