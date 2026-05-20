<script lang="ts">
  import type { PageData } from './$types';
  import type { ProductImage } from '$lib/types';
  import { cartStore } from '$lib/stores/cart.svelte';
  import { isVideo, isStreamingVideo, getEmbedURL } from '$lib/media';
  import { page } from '$app/state';
  import { cubicOut } from 'svelte/easing';
  import * as m from '$lib/paraglide/messages';
  import Seo from '$lib/components/Seo.svelte';
  import { siteOrigin, snippet } from '$lib/seo';
  import WishlistButton from '$lib/components/shop/WishlistButton.svelte';
  import RecentlyViewed from '$lib/components/shop/RecentlyViewed.svelte';
  import BundleComposer from '$lib/components/shop/BundleComposer.svelte';
  import StickyAddToCart from '$lib/components/shop/StickyAddToCart.svelte';
  import TaobaoSelectionModal from '$lib/components/shop/TaobaoSelectionModal.svelte';
  import TaobaoImageBrowserModal from '$lib/components/shop/TaobaoImageBrowserModal.svelte';
  type BrowserItem = {
    kind: 'variant' | 'bundle';
    id: string;
    variantId: string;
    name: string;
    image: string | null;
    price: number;
    compareAtPrice: number | null;
    stockQty: number;
  };
  import { recentlyViewedStore } from '$lib/stores/recentlyViewed.svelte';
  import { trackViewItem, trackAddToCart } from '$lib/tracker';
  import MarkdownContent from '$lib/components/MarkdownContent.svelte';
  import ResponsiveImage from '$lib/components/ResponsiveImage.svelte';
  import { onMount } from 'svelte';

  // PDP main image is sticky on the left in a 4fr/3fr grid on lg+: max column
  // width ~720px there, full viewport on mobile. Eager + high priority for LCP.
  const PDP_MAIN_WIDTHS = [480, 640, 960, 1280];
  const PDP_MAIN_SIZES = '(min-width: 1024px) 720px, 100vw';
  // Gallery thumbnails are a fixed 64px square strip (w-16 h-16).
  const PDP_THUMB_WIDTHS = [160, 320];
  const PDP_THUMB_SIZES = '80px';

  let { data }: { data: PageData } = $props();

  // Resolve `?variant=` to an actual variant id. Accepts either id or SKU;
  // unknown values silently fall back to the first variant.
  function resolveVariantParam(param: string | null): string | undefined {
    if (!param) return undefined;
    return (
      data.variants.find((v) => v.id === param)?.id ??
      data.variants.find((v) => v.sku === param)?.id
    );
  }

  let selectedVariantID = $state<string | undefined>(
    resolveVariantParam(page.url.searchParams.get('variant'))
  );

  onMount(() => {
    recentlyViewedStore.init();
    recentlyViewedStore.push(data.product.id);
    // P3 #26 — fire view_item to GA4 / Meta Pixel (no-ops if neither configured)
    const v = data.variants.slice().sort((a, b) => a.price - b.price)[0];
    trackViewItem({
      id: data.product.id,
      name: data.product.name,
      price: v?.price ?? 0
    });
  });
  let activeImageID = $state<string | undefined>(undefined);

  type TabId = 'content' | 'howto' | 'surfaces';
  const availableTabs: TabId[] = (() => {
    const arr: TabId[] = [];
    if (data.product.description?.trim()) arr.push('content');
    if (data.product.how_to_use?.trim()) arr.push('howto');
    if ((data.product.compatible_surfaces ?? []).length > 0) arr.push('surfaces');
    return arr;
  })();
  let activeTab = $state<TabId>(availableTabs[0] ?? 'content');
  let tabButtons: Partial<Record<TabId, HTMLButtonElement>> = $state({});
  let mobileOpen = $state<Record<TabId, boolean>>(
    Object.fromEntries(availableTabs.map((t) => [t, true])) as Record<TabId, boolean>
  );

  // 4-cell strip between tabs section and BundleComposer. Empty slots
  // (no media_N_url) are dropped so e.g. a product with only 2 images
  // still renders a clean row instead of two ghost cells.
  const mediaStrip = $derived(
    [
      { url: data.product.media_1_url, webp: data.product.media_1_webp_url },
      { url: data.product.media_2_url, webp: data.product.media_2_webp_url },
      { url: data.product.media_3_url, webp: data.product.media_3_webp_url },
      { url: data.product.media_4_url, webp: data.product.media_4_webp_url }
    ].filter((m): m is { url: string; webp?: string | null } => !!m.url)
  );

  function onTabKeydown(e: KeyboardEvent) {
    if (availableTabs.length < 2) return;
    let nextIdx: number | null = null;
    const i = availableTabs.indexOf(activeTab);
    if (e.key === 'ArrowRight') nextIdx = (i + 1) % availableTabs.length;
    else if (e.key === 'ArrowLeft') nextIdx = (i - 1 + availableTabs.length) % availableTabs.length;
    else if (e.key === 'Home') nextIdx = 0;
    else if (e.key === 'End') nextIdx = availableTabs.length - 1;
    if (nextIdx == null) return;
    e.preventDefault();
    const next = availableTabs[nextIdx];
    activeTab = next;
    tabButtons[next]?.focus();
  }

  const selectedVariant = $derived(
    data.variants.find((v) => v.id === selectedVariantID) ?? data.variants[0]
  );

  // SEO derivations (computed once per render)
  const seoOrigin = $derived(siteOrigin(page.data.publicSettings));
  const seoCanonical = $derived(`${seoOrigin}/products/${data.product.slug}`);
  const seoHeroImage = $derived(data.images.find((i) => i.is_primary) ?? data.images[0]);
  const seoOgImage = $derived(
    seoHeroImage?.url
      ? (seoHeroImage.url.startsWith('http') ? seoHeroImage.url : `${seoOrigin}${seoHeroImage.url}`)
      : undefined
  );
  const seoDescription = $derived(
    snippet(data.product.excerpt || data.product.description || data.product.name)
  );
  const seoCheapestVariant = $derived(data.variants.slice().sort((a, b) => a.price - b.price)[0]);
  const seoJsonLd = $derived([
    {
      '@context': 'https://schema.org',
      '@type': 'Product',
      name: data.product.name,
      description: seoDescription,
      url: seoCanonical,
      ...(seoOgImage ? { image: seoOgImage } : {}),
      ...(seoCheapestVariant
        ? {
            offers: {
              '@type': 'Offer',
              price: seoCheapestVariant.price,
              priceCurrency: 'HKD',
              availability:
                seoCheapestVariant.stock_qty > 0
                  ? 'https://schema.org/InStock'
                  : 'https://schema.org/OutOfStock',
              url: seoCanonical
            }
          }
        : {})
    },
    {
      '@context': 'https://schema.org',
      '@type': 'BreadcrumbList',
      itemListElement: [
        { '@type': 'ListItem', position: 1, name: m.common_home(), item: seoOrigin },
        { '@type': 'ListItem', position: 2, name: m.common_products(), item: `${seoOrigin}/products` },
        ...(data.category
          ? [
              {
                '@type': 'ListItem',
                position: 3,
                name: data.category.name,
                item: `${seoOrigin}/products/category/${data.category.slug}`
              }
            ]
          : []),
        {
          '@type': 'ListItem',
          position: data.category ? 4 : 3,
          name: data.product.name,
          item: seoCanonical
        }
      ]
    }
  ]);

  function selectVariant(id: string, sku: string) {
    selectedVariantID = id;
    activeImageID = undefined;
    if (typeof window !== 'undefined') {
      const url = new URL(window.location.href);
      url.searchParams.set('variant', sku);
      history.replaceState(history.state, '', url);
    }
  }
  const activeImage = $derived<ProductImage | undefined>(
    data.images.find((i) => i.id === activeImageID) ??
    data.images.find((i) => i.variant_id != null && i.variant_id === selectedVariant?.id) ??
    data.images.find((i) => i.is_primary) ??
    data.images[0]
  );

  const imageCount = $derived(data.images.length);
  const activeIndex = $derived(
    Math.max(0, data.images.findIndex((i) => i.id === activeImage?.id))
  );

  let direction: 'next' | 'prev' = 'next';

  function goTo(index: number) {
    if (imageCount === 0) return;
    const wrapped = ((index % imageCount) + imageCount) % imageCount;
    if (wrapped === activeIndex) return;
    direction = wrapped > activeIndex ? 'next' : 'prev';
    activeImageID = data.images[wrapped].id;
  }
  const goPrev = () => { direction = 'prev'; goStep(-1); };
  const goNext = () => { direction = 'next'; goStep(1); };
  function goStep(step: number) {
    if (imageCount === 0) return;
    const wrapped = ((activeIndex + step) % imageCount + imageCount) % imageCount;
    activeImageID = data.images[wrapped].id;
  }

  function slide(_node: Element, { dir, duration = 280 }: { dir: number; duration?: number }) {
    return {
      duration,
      easing: cubicOut,
      css: (_t: number, u: number) => `transform: translateX(${u * dir * 100}%);`
    };
  }

  let touchStartX = 0;
  let touchStartY = 0;
  let touchActive = false;
  const SWIPE_THRESHOLD = 40;

  function onTouchStart(e: TouchEvent) {
    if (imageCount < 2) return;
    touchStartX = e.touches[0].clientX;
    touchStartY = e.touches[0].clientY;
    touchActive = true;
  }
  function onTouchEnd(e: TouchEvent) {
    if (!touchActive) return;
    touchActive = false;
    const dx = e.changedTouches[0].clientX - touchStartX;
    const dy = e.changedTouches[0].clientY - touchStartY;
    if (Math.abs(dx) > SWIPE_THRESHOLD && Math.abs(dx) > Math.abs(dy)) {
      if (dx < 0) goNext();
      else goPrev();
    }
  }
  function onTouchCancel() {
    touchActive = false;
  }

  let iframeUnlocked = $state(false);

  $effect(() => {
    activeImage?.id;
    iframeUnlocked = false;
  });

  let overlayStartT = 0;
  const TAP_MAX_MOVE = 10;
  const TAP_MAX_TIME = 350;

  function onOverlayTouchStart(e: TouchEvent) {
    overlayStartT = e.timeStamp;
    onTouchStart(e);
  }
  function onOverlayTouchEnd(e: TouchEvent) {
    const dx = e.changedTouches[0].clientX - touchStartX;
    const dy = e.changedTouches[0].clientY - touchStartY;
    const dt = e.timeStamp - overlayStartT;
    const isTap =
      Math.abs(dx) < TAP_MAX_MOVE &&
      Math.abs(dy) < TAP_MAX_MOVE &&
      dt < TAP_MAX_TIME;
    if (isTap) {
      iframeUnlocked = true;
      e.preventDefault();
    }
    onTouchEnd(e);
  }

  let qty = $state(1);
  let adding = $state(false);
  let added = $state(false);

  // DOM refs for the sticky-cart visibility observer (StickyAddToCart §4.8).
  let summaryEl = $state<HTMLDivElement | undefined>();
  let ctaEl = $state<HTMLDivElement | undefined>();

  const inStock = $derived((selectedVariant?.stock_qty ?? 0) > 0);
  const hasDiscount = $derived(
    selectedVariant?.compare_at_price != null &&
    selectedVariant.compare_at_price > selectedVariant.price
  );
  const discountPct = $derived(
    hasDiscount
      ? Math.round((1 - selectedVariant!.price / selectedVariant!.compare_at_price!) * 100)
      : 0
  );

  async function addToCart() {
    // Taobao layout intercepts the CTA — clicking the inline button opens
    // the selection modal instead of going straight to cart. Real
    // add-to-cart happens from inside the modal.
    if (data.useTaobaoLayout) {
      taobaoSelectionOpen = true;
      return;
    }
    if (!selectedVariant || !inStock) return;
    adding = true;
    try {
      await cartStore.add(selectedVariant.id, qty);
      // P3 #26 — analytics; safe no-op if no tracker configured
      trackAddToCart({
        id: data.product.id,
        name: data.product.name,
        price: selectedVariant.price,
        quantity: qty
      });
      added = true;
      setTimeout(() => (added = false), 2500);
    } finally {
      adding = false;
    }
  }

  // ── Taobao layout (modal-driven add-to-cart) ─────────────────────────────
  let taobaoSelectionOpen = $state(false);
  let taobaoBrowserOpen = $state(false);
  let taobaoBrowserStart = $state(0);

  // One BrowserItem per variant + per promo bundle, in the same order the
  // selection modal lists them. The image browser modal indexes into this
  // shared array so the slide that opens always matches the row clicked.
  const taobaoItems = $derived<BrowserItem[]>(
    !data.useTaobaoLayout
      ? []
      : [
          ...data.variants.map((v): BrowserItem => ({
            kind: 'variant',
            id: v.id,
            variantId: v.id,
            name: (v.name && v.name.trim()) || v.sku || data.product.name,
            image:
              v.image_url ??
              data.images.find((img) => img.variant_id === v.id)?.url ??
              data.images.find((img) => img.is_primary)?.url ??
              data.images[0]?.url ??
              null,
            price: v.price,
            compareAtPrice: v.compare_at_price ?? null,
            stockQty: v.stock_qty
          })),
          ...(data.promoBundles ?? []).map((b): BrowserItem => ({
            kind: 'bundle',
            id: b.bundle_product_id,
            variantId: b.variant_id,
            name: b.name,
            image: b.primary_image_url ?? null,
            price: b.price,
            compareAtPrice: b.compare_at_price ?? null,
            stockQty: b.stock_qty
          }))
        ]
  );

  function openTaobaoImageBrowser(index: number) {
    taobaoBrowserStart = index;
    taobaoBrowserOpen = true;
  }
</script>

<Seo
  title={m.product_detail_title({ name: data.product.name })}
  description={seoDescription}
  canonical={seoCanonical}
  image={seoOgImage}
  type="product"
  jsonLd={seoJsonLd}
/>

<!-- ── HERO ──────────────────────────────────────────────────────── -->
<div class="bg-white">
  <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 pt-6 pb-16">

    <!-- Breadcrumb -->
    <nav class="flex flex-wrap gap-2 items-center text-[11px] font-display uppercase tracking-[0.15em] text-ink-500 mb-10">
      <a href="/" class="hover:text-navy-500 transition-colors">{m.common_home()}</a>
      <span aria-hidden="true">/</span>
      <a href="/products" class="hover:text-navy-500 transition-colors">{m.common_products()}</a>
      {#if data.category}
        <span aria-hidden="true">/</span>
        <a href="/products/category/{data.category.slug}"
           class="hover:text-navy-500 transition-colors">{data.category.name}</a>
      {/if}
      <span aria-hidden="true">/</span>
      <span class="font-semibold text-navy-900 truncate max-w-[60vw]">{data.product.name}</span>
    </nav>

    <div class="grid grid-cols-1 lg:grid-cols-[4fr_3fr] gap-10 lg:gap-16 items-start">

      <!-- LEFT: Image Gallery -->
      <div class="flex flex-col gap-4">
        <div
          class="aspect-square rounded-lg overflow-hidden bg-paper relative group"
          ontouchstart={onTouchStart}
          ontouchend={onTouchEnd}
          ontouchcancel={onTouchCancel}
        >
          {#if activeImage}
            {#key activeImage.id}
              <div
                class="absolute inset-0"
                in:slide={{ dir: direction === 'next' ? 1 : -1 }}
                out:slide={{ dir: direction === 'next' ? -1 : 1 }}
              >
                {#if isStreamingVideo(activeImage) && getEmbedURL(activeImage)}
                  {#if activeImage.video_fit === 'cover'}
                    <iframe
                      src={getEmbedURL(activeImage) ?? ''}
                      title={activeImage.alt_text ?? data.product.name}
                      class="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 min-w-full min-h-full aspect-video {!iframeUnlocked ? '[@media(pointer:coarse)]:pointer-events-none' : ''}"
                      allow={activeImage.video_autoplay ? 'autoplay; encrypted-media; picture-in-picture' : 'encrypted-media; picture-in-picture'}
                      allowfullscreen
                      frameborder="0"
                    ></iframe>
                  {:else}
                    <iframe
                      src={getEmbedURL(activeImage) ?? ''}
                      title={activeImage.alt_text ?? data.product.name}
                      class="w-full h-full {!iframeUnlocked ? '[@media(pointer:coarse)]:pointer-events-none' : ''}"
                      allow={activeImage.video_autoplay ? 'autoplay; encrypted-media; picture-in-picture' : 'encrypted-media; picture-in-picture'}
                      allowfullscreen
                      frameborder="0"
                    ></iframe>
                  {/if}
                  {#if !iframeUnlocked}
                    <button
                      type="button"
                      aria-label={activeImage.video_autoplay
                        ? 'Tap to interact with video; swipe to change media'
                        : 'Tap to play video; swipe to change media'}
                      class="absolute inset-0 z-[5] hidden [@media(pointer:coarse)]:block bg-transparent
                             focus:outline-none focus-visible:ring-2 focus-visible:ring-white/60"
                      ontouchstart={onOverlayTouchStart}
                      ontouchend={onOverlayTouchEnd}
                    >
                      {#if !activeImage.video_autoplay}
                        <span aria-hidden="true" class="absolute inset-0 flex items-center justify-center">
                          <span class="w-16 h-16 rounded-full bg-black/45 backdrop-blur-sm flex items-center justify-center text-white">
                            <svg class="w-7 h-7 ml-1" fill="currentColor" viewBox="0 0 24 24">
                              <path d="M8 5v14l11-7z"/>
                            </svg>
                          </span>
                        </span>
                      {/if}
                    </button>
                  {/if}
                {:else if isVideo(activeImage)}
                  <video
                    src={activeImage.url}
                    autoplay muted loop playsinline preload="metadata"
                    aria-label={activeImage.alt_text ?? data.product.name}
                    class="w-full h-full {activeImage.video_fit === 'contain' ? 'object-contain' : 'object-cover'} transition-transform duration-700 group-hover:scale-[1.03]"
                  ></video>
                {:else}
                  <ResponsiveImage
                    src={activeImage.url}
                    alt={activeImage.alt_text ?? data.product.name}
                    widths={PDP_MAIN_WIDTHS}
                    sizes={PDP_MAIN_SIZES}
                    loading="eager"
                    fetchpriority="high"
                    class="w-full h-full object-cover transition-transform duration-700 group-hover:scale-[1.03]"
                  />
                {/if}
              </div>
            {/key}
          {:else}
            <div class="w-full h-full flex flex-col items-center justify-center gap-3 text-gray-300">
              <svg class="w-16 h-16" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1"
                  d="m2.25 15.75 5.159-5.159a2.25 2.25 0 0 1 3.182 0l5.159 5.159m-1.5-1.5
                     1.409-1.409a2.25 2.25 0 0 1 3.182 0l2.909 2.909m-18 3.75h16.5a1.5 1.5
                     0 0 0 1.5-1.5V6a1.5 1.5 0 0 0-1.5-1.5H3.75A1.5 1.5 0 0 0 2.25 6v12a1.5
                     1.5 0 0 0 1.5 1.5Zm10.5-11.25h.008v.008h-.008V8.25Z" />
              </svg>
            </div>
          {/if}

          {#if hasDiscount}
            <span class="absolute top-4 right-4 z-10 inline-flex items-center justify-center
                         w-12 h-12 bg-navy-500 text-white font-display font-bold text-sm rounded-sm tabular-nums">
              −{discountPct}%
            </span>
          {/if}

          {#if imageCount > 1}
            <button
              type="button"
              onclick={goPrev}
              aria-label={m.common_aria_previous_image()}
              class="hidden md:flex absolute left-3 top-1/2 z-10 -translate-y-1/2 w-10 h-10 items-center justify-center
                     rounded-full bg-white/90 backdrop-blur text-ink-900 shadow-card
                     opacity-0 group-hover:opacity-100 transition-opacity duration-200 ease-gy
                     hover:bg-white hover:text-navy-500"
            >
              <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7" />
              </svg>
            </button>
            <button
              type="button"
              onclick={goNext}
              aria-label={m.common_aria_next_image()}
              class="hidden md:flex absolute right-3 top-1/2 z-10 -translate-y-1/2 w-10 h-10 items-center justify-center
                     rounded-full bg-white/90 backdrop-blur text-ink-900 shadow-card
                     opacity-0 group-hover:opacity-100 transition-opacity duration-200 ease-gy
                     hover:bg-white hover:text-navy-500"
            >
              <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
              </svg>
            </button>
          {/if}
        </div>

        {#if imageCount > 1}
          <div class="flex justify-center gap-1.5 pt-1">
            {#each data.images as img, i}
              <button
                type="button"
                onclick={() => (activeImageID = img.id)}
                aria-label={m.common_aria_go_to_image({ n: i + 1 })}
                aria-current={activeIndex === i ? 'true' : undefined}
                class="h-1.5 rounded-full transition-all duration-300 ease-gy
                       {activeIndex === i ? 'w-6 bg-navy-500' : 'w-1.5 bg-ink-300 hover:bg-ink-500'}"
              ></button>
            {/each}
          </div>
        {/if}

        {#if data.images.length > 1}
          <div class="flex gap-2 overflow-x-auto pb-1">
            {#each data.images as img}
              <button
                onclick={() => (activeImageID = img.id)}
                class="relative flex-shrink-0 w-16 h-16 rounded-md overflow-hidden border-2 transition-all duration-200 ease-gy
                       {(activeImageID === img.id || (!activeImageID && img.is_primary))
                         ? 'opacity-100 border-navy-500'
                         : 'border-transparent opacity-50 hover:opacity-80'}"
              >
                {#if isVideo(img)}
                  {#if img.thumbnail_url}
                    <ResponsiveImage src={img.thumbnail_url} alt={img.alt_text ?? ''}
                                     widths={PDP_THUMB_WIDTHS} sizes={PDP_THUMB_SIZES}
                                     class="w-full h-full object-cover bg-black" />
                  {:else}
                    <div class="w-full h-full bg-black"></div>
                  {/if}
                  <span class="absolute inset-0 flex items-center justify-center pointer-events-none" aria-hidden="true">
                    <span class="p-1 rounded-full bg-black/50 text-white">
                      <svg class="w-3 h-3" fill="currentColor" viewBox="0 0 24 24"><path d="M8 5v14l11-7z"/></svg>
                    </span>
                  </span>
                {:else}
                  <ResponsiveImage src={img.url} alt={img.alt_text ?? ''}
                                   widths={PDP_THUMB_WIDTHS} sizes={PDP_THUMB_SIZES}
                                   class="w-full h-full object-cover" />
                {/if}
              </button>
            {/each}
          </div>
        {/if}
      </div>

      <!-- RIGHT: Product Info (sticky on lg+) -->
      <div class="flex flex-col gap-6 lg:pt-2 lg:sticky lg:top-24 lg:self-start"
           bind:this={summaryEl}>

        <!-- Category eyebrow chip -->
        {#if data.category}
          <a href="/products/category/{data.category.slug}"
             class="self-start inline-flex items-center gap-1 text-[11px] font-display font-semibold uppercase
                    tracking-[0.15em] text-navy-500 bg-paper hover:bg-cream
                    px-2.5 py-1 rounded-sm transition-colors">
            {data.category.name}
          </a>
        {/if}

        <!-- Name (compressed display, clamp 28-44px) -->
        <h1 class="font-display text-3xl md:text-4xl lg:text-5xl font-semibold tracking-tight leading-[1.05] text-ink-900">
          {data.product.name}
        </h1>

        <!-- Subtitle in navy -->
        {#if data.product.subtitle}
          <p class="text-base md:text-lg font-display font-medium text-navy-500 leading-snug -mt-3">
            {data.product.subtitle}
          </p>
        {/if}

        <!-- Price -->
        {#if selectedVariant}
          <div class="flex items-baseline gap-3 flex-wrap">
            <span class="font-display text-3xl md:text-4xl font-bold tabular-nums tracking-tight text-ink-900">
              HK${selectedVariant.price.toFixed(2)}
            </span>
            {#if hasDiscount}
              <span class="font-body text-base md:text-lg line-through tabular-nums text-ink-500">
                HK${selectedVariant.compare_at_price!.toFixed(2)}
              </span>
            {/if}
          </div>
        {/if}

        <!-- Excerpt (body) -->
        {#if data.product.excerpt}
          <p class="font-body text-base leading-[1.75] text-ink-900/85 max-w-md">
            {data.product.excerpt}
          </p>
        {/if}

        <div class="border-t border-ink-300/60"></div>

        <!-- Variant selector. In taobao layout the variant chips live inside
             the selection modal, so the inline selector is hidden here. -->
        {#if data.variants.length > 0 && data.product.kind !== 'bundle' && !data.useTaobaoLayout}
          <div>
            <p class="text-[11px] font-display font-semibold uppercase tracking-[0.18em] text-navy-500 mb-3">
              {data.variants.length > 1 ? m.product_detail_options_label_multi() : m.product_detail_options_label_single()}
            </p>
            <div class="flex flex-wrap gap-2">
              {#each data.variants as v}
                {@const isSelected = selectedVariant?.id === v.id}
                {@const isAvailable = v.stock_qty > 0}
                <button
                  onclick={() => selectVariant(v.id, v.sku)}
                  disabled={!isAvailable}
                  class="px-4 py-2.5 rounded-xl font-display text-sm font-bold border-2 transition-all duration-200 ease-gy
                         {isSelected
                           ? 'bg-navy-500 border-navy-500 text-white'
                           : isAvailable
                             ? 'border-ink-300 text-ink-900 hover:border-navy-500'
                             : 'opacity-40 cursor-not-allowed line-through border-ink-300 text-ink-500'}"
                >
                  {(() => {
                    const n = v.name?.trim();
                    if (!n) return v.sku;
                    return n.split(' / ').map((p) => {
                      const i = p.indexOf(':');
                      return i >= 0 ? p.slice(i + 1).trim() : p;
                    }).join(' / ');
                  })()}
                </button>
              {/each}
            </div>
          </div>
        {/if}

        <!-- Bundle contents (above qty + CTA) -->
        {#if data.product.kind === 'bundle' && data.bundleItems && data.bundleItems.length > 0}
          <div class="rounded-xl border border-ink-200 p-5">
            <p class="text-sm font-display font-semibold uppercase tracking-[0.18em] text-navy-500 mb-4">
              {m.product_detail_bundle_heading()}
            </p>
            <ul class="grid grid-cols-1 xl:grid-cols-2 gap-x-6 gap-y-5">
              {#each data.bundleItems as item}
                <li class="flex items-start gap-3">
                  {#if item.component_primary_image_url}
                    <ResponsiveImage
                      src={item.component_primary_image_url}
                      alt={item.component_product_name ?? ''}
                      widths={[80, 160]}
                      sizes="(min-width: 640px) 96px, 80px"
                      class="w-20 h-20 sm:w-24 sm:h-24 flex-shrink-0 object-contain rounded-md bg-white" />
                  {/if}
                  <div class="flex-1 min-w-0">
                    {#if item.component_product_slug}
                      <a href="/products/{item.component_product_slug}"
                         class="inline-flex items-center gap-1 font-display font-semibold text-sm text-ink-900 hover:text-navy-600 transition-colors">
                        <span>{item.display_name_override || item.component_product_name}</span>
                        {#if item.quantity > 1}<span class="text-ink-500 font-body font-normal">× {item.quantity}</span>{/if}
                        <svg class="w-3.5 h-3.5 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                          <path stroke-linecap="round" stroke-linejoin="round" d="M13.5 6H5.25A2.25 2.25 0 003 8.25v10.5A2.25 2.25 0 005.25 21h10.5A2.25 2.25 0 0018 18.75V10.5m-10.5 6L21 3m0 0h-5.25M21 3v5.25"/>
                        </svg>
                      </a>
                    {:else}
                      <p class="font-display font-semibold text-sm text-ink-900">
                        {item.display_name_override || item.component_product_name}
                        {#if item.quantity > 1}<span class="text-ink-500 font-normal">× {item.quantity}</span>{/if}
                      </p>
                    {/if}
                    {#if item.component_product_subtitle}
                      <p class="text-sm text-navy-600 mt-1">{item.component_product_subtitle}</p>
                    {/if}
                    {#if item.component_variant_name}
                      <p class="text-xs text-ink-500 mt-1.5">{item.component_variant_name}</p>
                    {/if}
                  </div>
                </li>
              {/each}
            </ul>
          </div>
        {/if}

        <!-- Qty + CTA. In taobao layout the qty stepper lives inside the
             selection modal, so the inline stepper is hidden here. -->
        <div class="flex gap-3" bind:this={ctaEl}>
          {#if !data.useTaobaoLayout}
            <div class="flex items-center border-2 border-ink-300 rounded-xl overflow-hidden bg-white">
              <button
                type="button"
                onclick={() => (qty = Math.max(1, qty - 1))}
                aria-label={m.common_aria_decrease_quantity()}
                disabled={qty <= 1}
                class="w-11 h-12 flex items-center justify-center text-ink-500 hover:text-navy-500 hover:bg-paper
                       disabled:opacity-40 disabled:cursor-not-allowed transition-colors text-lg"
              >−</button>
              <span class="w-10 text-center font-display text-sm font-bold text-ink-900 tabular-nums" aria-live="polite">{qty}</span>
              <button
                type="button"
                onclick={() => (qty = qty + 1)}
                aria-label={m.common_aria_increase_quantity()}
                class="w-11 h-12 flex items-center justify-center text-ink-500 hover:text-navy-500 hover:bg-paper
                       transition-colors text-lg"
              >+</button>
            </div>
          {/if}

          <button
            onclick={addToCart}
            disabled={!inStock || adding}
            class="flex-1 h-12 px-6 rounded-md font-display font-bold text-sm tracking-[0.1em] uppercase transition-all duration-200 ease-gy text-white
                   {!inStock
                     ? 'bg-ink-300 cursor-not-allowed'
                     : added
                       ? 'bg-success'
                       : 'bg-navy-500 hover:bg-navy-700 active:scale-[0.98]'}"
          >
            {#if !inStock}
              {m.product_detail_out_of_stock()}
            {:else if adding}
              {m.product_detail_adding()}
            {:else if added}
              {m.product_detail_added()}
            {:else}
              {m.product_detail_add_to_cart()}
            {/if}
          </button>
        </div>

        <!-- Stock + dispatch line -->
        {#if inStock && selectedVariant}
          <p class="text-xs font-body text-ink-500 -mt-2">
            <span class="text-success font-semibold">●</span>
            {data.product.kind === 'bundle'
              ? m.product_detail_sets_in_stock({ count: selectedVariant.stock_qty })
              : m.product_detail_units_in_stock({ count: selectedVariant.stock_qty })}
          </p>
        {/if}

        <WishlistButton productID={data.product.id} variant="full" class="w-full sm:w-auto" />

        <!-- Trust strip — three column promises -->
        <ul class="grid grid-cols-3 gap-4 pt-6">
          {#each [
            { icon: 'M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z', label: m.product_detail_trust_genuine() },
            { icon: 'M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4', label: m.product_detail_trust_shipping() },
            { icon: 'M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15', label: m.product_detail_trust_returns() }
          ] as t}
            <li class="flex flex-col items-center text-center gap-1.5">
              <svg class="w-5 h-5 text-navy-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d={t.icon} />
              </svg>
              <span class="text-[11px] font-display font-semibold uppercase tracking-[0.12em] text-ink-900">
                {t.label}
              </span>
            </li>
          {/each}
        </ul>

      </div>
    </div>
  </div>
</div>


<!-- ── SPECS STRIP — gyeon-project-design-system §4.5 ──────────── -->
<div class="bg-navy-900">
  <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
    <div class="grid grid-cols-2 md:grid-cols-4 gap-px bg-white/10">
      {#each [
        { icon: 'M9.75 3.104v5.714a2.25 2.25 0 0 1-.659 1.591L5 14.5M9.75 3.104c-.251.023-.501.05-.75.082m.75-.082a24.301 24.301 0 0 1 4.5 0m0 0v5.714c0 .597.237 1.17.659 1.591L19.8 15.3M14.25 3.186c.251.023.501.05.75.082M19.8 15.3l-1.57.393A9.065 9.065 0 0 1 12 15a9.065 9.065 0 0 0-6.23-.693L5 14.5m14.8.8 1.402 1.402c1.232 1.232.65 3.318-1.067 3.611A48.309 48.309 0 0 1 12 21c-2.773 0-5.491-.235-8.135-.687-1.718-.293-2.3-2.379-1.067-3.61L5 14.5',
          value: m.product_detail_specs_korean_rd(),
          caption: 'KOREAN R&D' },
        { icon: 'M9 12.75 11.25 15 15 9.75M21 12c0 1.268-.63 2.39-1.593 3.068a3.745 3.745 0 0 1-1.043 3.296 3.745 3.745 0 0 1-3.296 1.043A3.745 3.745 0 0 1 12 21c-1.268 0-2.39-.63-3.068-1.593a3.746 3.746 0 0 1-3.296-1.043 3.745 3.745 0 0 1-1.043-3.296A3.745 3.745 0 0 1 3 12c0-1.268.63-2.39 1.593-3.068a3.745 3.745 0 0 1 1.043-3.296 3.746 3.746 0 0 1 3.296-1.043A3.746 3.746 0 0 1 12 3c1.268 0 2.39.63 3.068 1.593a3.746 3.746 0 0 1 3.296 1.043 3.746 3.746 0 0 1 1.043 3.296A3.745 3.745 0 0 1 21 12Z',
          value: m.product_detail_specs_total_care(),
          caption: 'PROFESSIONAL CARE' },
        { icon: 'M12 6v6h4.5m4.5 0a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z',
          value: m.product_detail_specs_lasting(),
          caption: 'LONG-LASTING' },
        { icon: 'M3 3v1.5M3 21v-6m0 0 2.77-.693a9 9 0 0 1 6.208.682l.108.054a9 9 0 0 0 6.086.71l3.114-.732a48.524 48.524 0 0 1-.005-10.499l-3.11.732a9 9 0 0 1-6.085-.711l-.108-.054a9 9 0 0 0-6.208-.682L3 4.5M3 15V4.5',
          value: m.product_detail_specs_since_2012(),
          caption: 'SINCE 2012' }
      ] as spec}
        <div class="bg-navy-900 flex flex-col items-center gap-2 py-8 px-4 sm:py-10 sm:px-6 text-center">
          <svg class="w-7 h-7 text-amber-300 mb-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d={spec.icon} />
          </svg>
          <span class="font-display text-2xl md:text-3xl font-bold text-white">{spec.value}</span>
          <span class="text-[10px] font-display font-semibold uppercase tracking-[0.18em] text-white/40">{spec.caption}</span>
        </div>
      {/each}
    </div>
  </div>
</div>

<!-- ── HERO VIDEO (between specs strip and tabs) ──────────────── -->
{#if data.product.video_id}
  <div class="bg-white">
    <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-10 md:py-14">
      <div class="relative aspect-video max-w-5xl mx-auto overflow-hidden rounded-lg shadow-md bg-navy-900">
        <iframe
          src="https://www.youtube-nocookie.com/embed/{data.product.video_id}?rel=0"
          title={data.product.name}
          class="absolute inset-0 h-full w-full"
          allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture"
          allowfullscreen
          loading="lazy"
          referrerpolicy="strict-origin-when-cross-origin"
        ></iframe>
      </div>
    </div>
  </div>
{/if}

<!-- ── TABS (md+) / ACCORDION (mobile) — gyeon-project-design-system §4.6 -->
{#if availableTabs.length > 0}
  {@const tabLabels: Record<TabId, string> = {
    content:  m.product_detail_tab_content(),
    howto:    m.product_detail_tab_howto(),
    surfaces: m.product_detail_tab_surfaces()
  }}
  {@const allSurfaces = [
    { key: 'paint',   name: m.product_detail_surface_paint(),   icon: 'M7 21a4 4 0 01-4-4V5a2 2 0 012-2h4a2 2 0 012 2v12a4 4 0 01-4 4zm0 0h12a2 2 0 002-2v-4a2 2 0 00-2-2h-2.343M11 7.343l1.657-1.657a2 2 0 012.828 0l2.829 2.829a2 2 0 010 2.828l-8.486 8.485M7 17h.01' },
    { key: 'glass',   name: m.product_detail_surface_glass(),   icon: 'M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17H3a2 2 0 01-2-2V5a2 2 0 012-2h14a2 2 0 012 2v10a2 2 0 01-2 2h-2' },
    { key: 'wheels',  name: m.product_detail_surface_wheels(),  icon: 'M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z' },
    { key: 'leather', name: m.product_detail_surface_leather(), icon: 'M5 3v4M3 5h4M6 17v4m-2-2h4m5-16l2.286 6.857L21 12l-5.714 2.143L13 21l-2.286-6.857L5 12l5.714-2.143L13 3z' },
    { key: 'trim',    name: m.product_detail_surface_trim(),    icon: 'M4 5a1 1 0 011-1h14a1 1 0 011 1v2a1 1 0 01-1 1H5a1 1 0 01-1-1V5zM4 13a1 1 0 011-1h6a1 1 0 011 1v6a1 1 0 01-1 1H5a1 1 0 01-1-1v-6zM16 13a1 1 0 011-1h2a1 1 0 011 1v6a1 1 0 01-1 1h-2a1 1 0 01-1-1v-6z' },
    { key: 'fabric',  name: m.product_detail_surface_fabric(),  icon: 'M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 8a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4m6 6v10m6-2a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4' }
  ]}
  {@const selected = new Set(data.product.compatible_surfaces ?? [])}

  <div class="bg-white">
    <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-12 md:py-20">

      <!-- DESKTOP: horizontal tab strip with active navy underline -->
      <div class="hidden md:block">
        <div class="flex gap-0 border-b border-ink-300/60 mb-10" role="tablist" onkeydown={onTabKeydown}>
          {#each availableTabs as id}
            <button
              type="button"
              role="tab"
              id="pdp-tab-{id}"
              aria-selected={activeTab === id}
              aria-controls="pdp-panel-{id}"
              tabindex={activeTab === id ? 0 : -1}
              bind:this={tabButtons[id]}
              onclick={() => (activeTab = id)}
              class="relative px-6 py-4 font-display text-sm font-bold uppercase tracking-[0.12em] transition-colors
                     after:absolute after:left-0 after:right-0 after:-bottom-px after:h-0.5 after:bg-navy-500
                     after:transition-transform after:duration-300 after:ease-gy after:origin-center
                     {activeTab === id
                       ? 'text-navy-900 after:scale-x-100'
                       : 'text-ink-500 hover:text-ink-900 after:scale-x-0'}"
            >
              {tabLabels[id]}
            </button>
          {/each}
        </div>

        {#if activeTab === 'content'}
          <div role="tabpanel" id="pdp-panel-content" aria-labelledby="pdp-tab-content" tabindex="0"
               class={data.product.banner_1_url ? '' : 'max-w-2xl'}>
            {#if data.product.banner_1_url}
              <div class="grid md:grid-cols-[minmax(0,2fr)_minmax(0,3fr)] gap-8 items-start max-w-5xl">
                <img
                  src={data.product.banner_1_webp_url ?? data.product.banner_1_url}
                  alt=""
                  class="w-full h-auto rounded-lg"
                  loading="lazy"
                />
                <div class="font-body text-base leading-[1.75] text-ink-900/85 prose prose-sm max-w-none">
                  <MarkdownContent content={data.product.description} refs={data.shortcodeRefs} />
                </div>
              </div>
            {:else}
              <div class="font-body text-base leading-[1.75] text-ink-900/85 prose prose-sm max-w-none">
                <MarkdownContent content={data.product.description} refs={data.shortcodeRefs} />
              </div>
            {/if}
          </div>

        {:else if activeTab === 'howto'}
          <div role="tabpanel" id="pdp-panel-howto" aria-labelledby="pdp-tab-howto" tabindex="0"
               class={data.product.banner_2_url ? '' : 'max-w-2xl'}>
            {#if data.product.banner_2_url}
              <div class="grid md:grid-cols-[minmax(0,2fr)_minmax(0,3fr)] gap-8 items-start max-w-5xl">
                <img
                  src={data.product.banner_2_webp_url ?? data.product.banner_2_url}
                  alt=""
                  class="w-full h-auto rounded-lg"
                  loading="lazy"
                />
                <div class="font-body text-base leading-[1.75] text-ink-900/85 prose prose-sm max-w-none">
                  <MarkdownContent content={data.product.how_to_use} refs={data.shortcodeRefs} />
                </div>
              </div>
            {:else}
              <div class="font-body text-base leading-[1.75] text-ink-900/85 prose prose-sm max-w-none">
                <MarkdownContent content={data.product.how_to_use} refs={data.shortcodeRefs} />
              </div>
            {/if}
          </div>

        {:else if activeTab === 'surfaces'}
          <div class="grid grid-cols-3 lg:grid-cols-6 gap-3"
               role="tabpanel" id="pdp-panel-surfaces" aria-labelledby="pdp-tab-surfaces" tabindex="0">
            {#each allSurfaces.filter(s => selected.has(s.key)) as surface}
              <div class="flex flex-col items-center gap-3 p-5 rounded-lg border border-ink-300/60 hover:border-navy-500 hover:bg-paper transition-colors">
                <div class="w-10 h-10 rounded-md bg-navy-500 flex items-center justify-center">
                  <svg class="w-5 h-5 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d={surface.icon} />
                  </svg>
                </div>
                <span class="font-display text-sm font-semibold text-ink-900 text-center">{surface.name}</span>
              </div>
            {/each}
          </div>
        {/if}
      </div>

      <!-- MOBILE: accordion (one panel open at a time, controlled by activeTab) -->
      <div class="md:hidden divide-y divide-ink-300/60 border-y border-ink-300/60">
        {#each availableTabs as id}
          {@const expanded = mobileOpen[id]}
          <div>
            <button
              type="button"
              onclick={() => (mobileOpen[id] = !mobileOpen[id])}
              aria-expanded={expanded}
              aria-controls="pdp-acc-{id}"
              class="w-full flex items-center justify-between gap-3 py-4 text-left"
            >
              <span class="font-display text-sm font-bold uppercase tracking-[0.12em]
                           {expanded ? 'text-navy-900' : 'text-ink-900'}">
                {tabLabels[id]}
              </span>
              <svg class="w-4 h-4 text-ink-500 transition-transform duration-200 ease-gy {expanded ? 'rotate-180 text-navy-500' : ''}"
                   fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"/>
              </svg>
            </button>
            {#if expanded}
              <div id="pdp-acc-{id}" class="pb-5">
                {#if id === 'content'}
                  {#if data.product.banner_1_url}
                    <img
                      src={data.product.banner_1_webp_url ?? data.product.banner_1_url}
                      alt=""
                      class="w-full h-auto rounded-lg mb-4"
                      loading="lazy"
                    />
                  {/if}
                  <div class="font-body text-base leading-[1.75] text-ink-900/85 prose prose-sm max-w-none">
                    <MarkdownContent content={data.product.description} refs={data.shortcodeRefs} />
                  </div>
                {:else if id === 'howto'}
                  {#if data.product.banner_2_url}
                    <img
                      src={data.product.banner_2_webp_url ?? data.product.banner_2_url}
                      alt=""
                      class="w-full h-auto rounded-lg mb-4"
                      loading="lazy"
                    />
                  {/if}
                  <div class="font-body text-base leading-[1.75] text-ink-900/85 prose prose-sm max-w-none">
                    <MarkdownContent content={data.product.how_to_use} refs={data.shortcodeRefs} />
                  </div>
                {:else if id === 'surfaces'}
                  <div class="grid grid-cols-3 gap-3">
                    {#each allSurfaces.filter(s => selected.has(s.key)) as surface}
                      <div class="flex flex-col items-center gap-2 p-3 rounded-lg border border-ink-300/60">
                        <div class="w-9 h-9 rounded-md bg-navy-500 flex items-center justify-center">
                          <svg class="w-4 h-4 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d={surface.icon} />
                          </svg>
                        </div>
                        <span class="font-display text-xs font-semibold text-ink-900 text-center leading-tight">{surface.name}</span>
                      </div>
                    {/each}
                  </div>
                {/if}
              </div>
            {/if}
          </div>
        {/each}
      </div>
    </div>
  </div>
{/if}

<!-- ── MEDIA STRIP (4-cell row above BundleComposer) ──────────── -->
{#if mediaStrip.length > 0}
  <div class="bg-white">
    <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-10 md:py-14">
      <div class="grid grid-cols-2 md:grid-cols-4 gap-4">
        {#each mediaStrip as m}
          <div class="aspect-square overflow-hidden rounded-lg bg-ink-300/10">
            <img
              src={m.webp ?? m.url}
              alt=""
              class="w-full h-full object-cover"
              loading="lazy"
            />
          </div>
        {/each}
      </div>
    </div>
  </div>
{/if}

<!-- ── BUNDLE COMPOSER (replaces flat related row, §4.7) ───────── -->
<BundleComposer items={data.related} />

<!-- ── STICKY ADD-TO-CART BAR (mobile only, §4.8) ────────────── -->
<StickyAddToCart
  ctaEl={ctaEl}
  product={data.product}
  variant={selectedVariant}
  primaryImage={activeImage ?? data.images[0]}
  onAdd={addToCart}
  inStock={inStock}
  adding={adding}
  added={added}
/>

<RecentlyViewed excludeID={data.product.id} />

<!-- ── Taobao-layout modals ─────────────────────────────────────────── -->
{#if data.useTaobaoLayout}
  <TaobaoSelectionModal
    bind:open={taobaoSelectionOpen}
    product={data.product}
    variants={data.variants}
    images={data.images}
    promoBundles={data.promoBundles ?? []}
    onOpenImageBrowser={openTaobaoImageBrowser}
  />
  <TaobaoImageBrowserModal
    bind:open={taobaoBrowserOpen}
    items={taobaoItems}
    startIndex={taobaoBrowserStart}
    productId={data.product.id}
  />
{/if}
