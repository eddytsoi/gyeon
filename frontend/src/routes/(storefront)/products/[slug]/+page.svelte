<script lang="ts">
  import type { PageData } from './$types';
  import type { ProductImage } from '$lib/types';
  import { cartStore } from '$lib/stores/cart.svelte';
  import { isVideo, isStreamingVideo, getEmbedURL } from '$lib/media';
  import { page } from '$app/state';
  import { cubicOut } from 'svelte/easing';
  import * as m from '$lib/paraglide/messages';

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
  let activeImageID = $state<string | undefined>(undefined);
  let activeTab = $state<'content' | 'howto' | 'surfaces'>('content');

  function renderMarkdown(md: string): string {
    return md
      .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
      .replace(/^#### (.+)$/gm, '<h4 class="text-base font-bold mt-6 mb-1 text-gray-900">$1</h4>')
      .replace(/^### (.+)$/gm, '<h3 class="text-lg font-bold mt-7 mb-2 text-gray-900">$1</h3>')
      .replace(/^## (.+)$/gm, '<h2 class="text-xl font-bold mt-8 mb-2 text-gray-900">$1</h2>')
      .replace(/^# (.+)$/gm, '<h1 class="text-2xl font-bold mt-8 mb-3 text-gray-900">$1</h1>')
      .replace(/\*\*\*(.+?)\*\*\*/g, '<strong><em>$1</em></strong>')
      .replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
      .replace(/\*(.+?)\*/g, '<em>$1</em>')
      .replace(/`(.+?)`/g, '<code class="bg-gray-100 text-gray-800 px-1.5 py-0.5 rounded text-sm font-mono">$1</code>')
      .replace(/\[(.+?)\]\((.+?)\)/g, '<a href="$2" class="text-gray-900 underline underline-offset-2 hover:text-gray-600">$1</a>')
      .replace(/^> (.+)$/gm, '<blockquote class="border-l-4 border-gray-200 pl-4 italic text-gray-500 my-4">$1</blockquote>')
      .replace(/^- (.+)$/gm, '<li class="ml-5 list-disc mb-1">$1</li>')
      .replace(/^\d+\. (.+)$/gm, '<li class="ml-5 list-decimal mb-1">$1</li>')
      .replace(/^---$/gm, '<hr class="my-8 border-gray-100" />')
      .replace(/\n\n/g, '</p><p class="mb-5 leading-relaxed text-gray-700">')
      .replace(/\n/g, '<br />');
  }

  const selectedVariant = $derived(
    data.variants.find((v) => v.id === selectedVariantID) ?? data.variants[0]
  );

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

  let qty = $state(1);
  let adding = $state(false);
  let added = $state(false);

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
    if (!selectedVariant || !inStock) return;
    adding = true;
    try {
      await cartStore.add(selectedVariant.id, qty);
      added = true;
      setTimeout(() => (added = false), 2500);
    } finally {
      adding = false;
    }
  }
</script>

<svelte:head>
  <title>{m.product_detail_title({ name: data.product.name })}</title>
</svelte:head>

<!-- ── HERO ──────────────────────────────────────────────────────── -->
<div class="bg-white">
  <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 pt-6 pb-16">

    <!-- Breadcrumb -->
    <nav class="flex gap-2 items-center text-[11px] uppercase tracking-[0.15em] text-gray-400 mb-10">
      <a href="/" class="hover:text-gray-700 transition-colors">{m.common_home()}</a>
      <span>/</span>
      <a href="/products" class="hover:text-gray-700 transition-colors">{m.common_products()}</a>
      {#if data.category}
        <span>/</span>
        <a href="/products/category/{data.category.slug}"
           class="hover:text-gray-700 transition-colors">{data.category.name}</a>
      {/if}
      <span>/</span>
      <span class="font-semibold" style="color: rgb(25,37,63)">{data.product.name}</span>
    </nav>

    <div class="grid grid-cols-1 lg:grid-cols-[3fr_2fr] gap-12 lg:gap-20 items-start">

      <!-- LEFT: Image Gallery -->
      <div class="flex flex-col gap-4">
        <div
          class="aspect-[4/3] lg:aspect-[5/4] rounded-3xl overflow-hidden bg-gray-50 relative group border border-gray-100"
          ontouchstart={onTouchStart}
          ontouchend={onTouchEnd}
        >
          {#if activeImage}
            {#key activeImage.id}
              <div
                class="absolute inset-0"
                in:slide={{ dir: direction === 'next' ? 1 : -1 }}
                out:slide={{ dir: direction === 'next' ? -1 : 1 }}
              >
                {#if isStreamingVideo(activeImage) && getEmbedURL(activeImage)}
                  <iframe
                    src={getEmbedURL(activeImage) ?? ''}
                    title={activeImage.alt_text ?? data.product.name}
                    class="w-full h-full"
                    allow="autoplay; encrypted-media; picture-in-picture"
                    allowfullscreen
                    frameborder="0"
                  ></iframe>
                {:else if isVideo(activeImage)}
                  <video
                    src={activeImage.url}
                    autoplay muted loop playsinline preload="metadata"
                    aria-label={activeImage.alt_text ?? data.product.name}
                    class="w-full h-full object-cover transition-transform duration-700 group-hover:scale-[1.03]"
                  ></video>
                {:else}
                  <img
                    src={activeImage.url}
                    alt={activeImage.alt_text ?? data.product.name}
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
            <div class="absolute top-4 left-4 z-10 px-3 py-1.5 rounded-lg text-xs font-bold tracking-wide text-white"
                 style="background: rgb(51,73,119)">
              −{discountPct}%
            </div>
          {/if}

          {#if imageCount > 1}
            <button
              type="button"
              onclick={goPrev}
              aria-label="Previous image"
              class="hidden md:flex absolute left-3 top-1/2 z-10 -translate-y-1/2 w-10 h-10 items-center justify-center
                     rounded-full bg-white/80 backdrop-blur text-gray-700 shadow-sm
                     opacity-0 group-hover:opacity-100 transition-opacity duration-200
                     hover:bg-white hover:text-gray-900"
            >
              <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7" />
              </svg>
            </button>
            <button
              type="button"
              onclick={goNext}
              aria-label="Next image"
              class="hidden md:flex absolute right-3 top-1/2 z-10 -translate-y-1/2 w-10 h-10 items-center justify-center
                     rounded-full bg-white/80 backdrop-blur text-gray-700 shadow-sm
                     opacity-0 group-hover:opacity-100 transition-opacity duration-200
                     hover:bg-white hover:text-gray-900"
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
                aria-label={`Go to image ${i + 1}`}
                aria-current={activeIndex === i ? 'true' : undefined}
                class="h-1.5 rounded-full transition-all
                       {activeIndex === i ? 'w-6' : 'w-1.5 bg-gray-300 hover:bg-gray-400'}"
                style={activeIndex === i ? 'background: rgb(51,73,119)' : ''}
              ></button>
            {/each}
          </div>
        {/if}

        {#if data.images.length > 1}
          <div class="flex gap-2 overflow-x-auto pb-1">
            {#each data.images as img}
              <button
                onclick={() => (activeImageID = img.id)}
                class="relative flex-shrink-0 w-16 h-16 rounded-xl overflow-hidden border-2 transition-all
                       {(activeImageID === img.id || (!activeImageID && img.is_primary))
                         ? 'opacity-100'
                         : 'border-transparent opacity-50 hover:opacity-80'}"
                style={(activeImageID === img.id || (!activeImageID && img.is_primary))
                  ? 'border-color: rgb(51,73,119)'
                  : ''}
              >
                {#if isVideo(img)}
                  {#if img.thumbnail_url}
                    <img src={img.thumbnail_url} alt={img.alt_text ?? ''} class="w-full h-full object-cover bg-black" />
                  {:else}
                    <div class="w-full h-full bg-black"></div>
                  {/if}
                  <span class="absolute inset-0 flex items-center justify-center pointer-events-none" aria-hidden="true">
                    <span class="p-1 rounded-full bg-black/50 text-white">
                      <svg class="w-3 h-3" fill="currentColor" viewBox="0 0 24 24"><path d="M8 5v14l11-7z"/></svg>
                    </span>
                  </span>
                {:else}
                  <img src={img.url} alt={img.alt_text ?? ''} class="w-full h-full object-cover" />
                {/if}
              </button>
            {/each}
          </div>
        {/if}
      </div>

      <!-- RIGHT: Product Info -->
      <div class="flex flex-col gap-7 lg:pt-2">

        <!-- Category label -->
        {#if data.category}
          <span class="text-[11px] font-bold uppercase tracking-[0.25em]"
                style="color: rgb(51,73,119)">
            {data.category.name}
          </span>
        {/if}

        <!-- Name -->
        <h1 class="text-4xl lg:text-5xl font-black tracking-tight leading-[1.05] text-gray-900">
          {data.product.name}
        </h1>

        <!-- Excerpt -->
        {#if data.product.excerpt}
          <p class="text-gray-500 leading-relaxed text-sm max-w-md">
            {data.product.excerpt}
          </p>
        {/if}

        <!-- Price -->
        {#if selectedVariant}
          <div class="flex items-baseline gap-3">
            <span class="text-3xl font-bold tracking-tight text-gray-900">
              HK${selectedVariant.price.toFixed(2)}
            </span>
            {#if hasDiscount}
              <span class="text-lg text-gray-400 line-through">
                HK${selectedVariant.compare_at_price!.toFixed(2)}
              </span>
            {/if}
          </div>
        {/if}

        <div class="border-t border-gray-100"></div>

        <!-- Variant selector -->
        {#if data.variants.length > 0}
          <div>
            <p class="text-[11px] font-bold uppercase tracking-[0.2em] text-gray-400 mb-3">
              {data.variants.length > 1 ? m.product_detail_options_label_multi() : m.product_detail_options_label_single()}
            </p>
            <div class="flex flex-wrap gap-2">
              {#each data.variants as v}
                <button
                  onclick={() => selectVariant(v.id, v.sku)}
                  disabled={v.stock_qty === 0}
                  class="px-4 py-2.5 rounded-xl text-sm font-semibold border-2 transition-all
                         {v.stock_qty === 0 ? 'opacity-25 cursor-not-allowed line-through border-gray-200 text-gray-400' : ''}"
                  style={selectedVariant?.id === v.id
                    ? 'border-color: rgb(51,73,119); background: rgb(51,73,119); color: white'
                    : v.stock_qty > 0
                      ? 'border-color: #e5e7eb; color: #374151'
                      : ''}
                >
                  {v.sku}
                </button>
              {/each}
            </div>
          </div>
        {/if}

        <!-- Qty + CTA -->
        <div class="flex gap-3">
          <div class="flex items-center border-2 border-gray-200 rounded-xl overflow-hidden bg-white">
            <button
              onclick={() => (qty = Math.max(1, qty - 1))}
              class="w-11 h-11 flex items-center justify-center text-gray-400 hover:text-gray-900 hover:bg-gray-50 transition-colors text-lg"
            >−</button>
            <span class="w-10 text-center text-sm font-semibold text-gray-900">{qty}</span>
            <button
              onclick={() => (qty = qty + 1)}
              class="w-11 h-11 flex items-center justify-center text-gray-400 hover:text-gray-900 hover:bg-gray-50 transition-colors text-lg"
            >+</button>
          </div>

          <button
            onclick={addToCart}
            disabled={!inStock || adding}
            class="flex-1 py-3 px-6 rounded-xl font-bold text-sm tracking-widest uppercase transition-all text-white
                   {!inStock ? 'cursor-not-allowed opacity-40' : 'active:scale-[0.98]'}"
            style={inStock
              ? added
                ? 'background: #16a34a'
                : 'background: rgb(51,73,119)'
              : 'background: #9ca3af'}
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

        {#if inStock && selectedVariant}
          <p class="text-xs text-gray-400">{m.product_detail_units_in_stock({ count: selectedVariant.stock_qty })}</p>
        {/if}

        <!-- Trust strip -->
        <div class="flex flex-wrap gap-5 pt-1 border-t border-gray-100">
          {#each [
            { icon: 'M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z', label: m.product_detail_trust_genuine() },
            { icon: 'M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4', label: m.product_detail_trust_shipping() },
            { icon: 'M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15', label: m.product_detail_trust_returns() }
          ] as t}
            <div class="flex items-center gap-2 text-xs text-gray-500">
              <svg class="w-4 h-4 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24"
                   style="color: rgb(113,135,183)">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d={t.icon} />
              </svg>
              {t.label}
            </div>
          {/each}
        </div>

      </div>
    </div>
  </div>
</div>

<!-- ── BUNDLE CONTENTS ───────────────────────────────────────────── -->
{#if data.product.kind === 'bundle' && data.bundleItems && data.bundleItems.length > 0}
  <div class="bg-gray-50 border-t border-gray-100">
    <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-12">
      <p class="text-[11px] font-bold uppercase tracking-[0.25em] mb-2"
         style="color: rgb(113,135,183)">{m.product_detail_bundle_kicker()}</p>
      <h2 class="text-2xl font-black text-gray-900 mb-8">{m.product_detail_bundle_heading()}</h2>
      <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
        {#each data.bundleItems as item}
          <div class="flex items-center gap-4 bg-white rounded-2xl border border-gray-100 px-5 py-4">
            <div class="w-8 h-8 rounded-lg flex-shrink-0 flex items-center justify-center"
                 style="background: rgb(51,73,119)">
              <svg class="w-4 h-4 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
              </svg>
            </div>
            <div class="min-w-0">
              <p class="font-semibold text-gray-900 text-sm truncate">
                {item.display_name_override || item.component_product_name || item.component_sku}
              </p>
              {#if item.quantity > 1}
                <p class="text-xs text-gray-400">{m.product_detail_bundle_qty({ quantity: item.quantity })}</p>
              {/if}
            </div>
          </div>
        {/each}
      </div>
    </div>
  </div>
{/if}

<!-- ── SPECS STRIP ────────────────────────────────────────────────── -->
<div style="background: rgb(25,37,63)">
  <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
    <div class="grid grid-cols-2 md:grid-cols-4 divide-x divide-white/10">
      {#each [
        { icon: 'M19.428 15.428a2 2 0 00-1.022-.547l-2.387-.477a6 6 0 00-3.86.517l-.318.158a6 6 0 01-3.86.517L6.05 15.21a2 2 0 00-1.806.547M8 4h8l-1 1v5.172a2 2 0 00.586 1.414l5 5c1.26 1.26.367 3.414-1.415 3.414H4.828c-1.782 0-2.674-2.154-1.414-3.414l5-5A2 2 0 009 10.172V5L8 4z', label: m.product_detail_specs_variants(), value: data.variants.length > 0 ? (data.variants.length > 1 ? m.product_detail_specs_sizes_many({ count: data.variants.length }) : m.product_detail_specs_sizes_one({ count: data.variants.length })) : m.product_detail_specs_dash() },
        { icon: 'M9 12l2 2 4-4M7.835 4.697a3.42 3.42 0 001.946-.806 3.42 3.42 0 014.438 0 3.42 3.42 0 001.946.806 3.42 3.42 0 013.138 3.138 3.42 3.42 0 00.806 1.946 3.42 3.42 0 010 4.438 3.42 3.42 0 00-.806 1.946 3.42 3.42 0 01-3.138 3.138 3.42 3.42 0 00-1.946.806 3.42 3.42 0 01-4.438 0 3.42 3.42 0 00-1.946-.806 3.42 3.42 0 01-3.138-3.138 3.42 3.42 0 00-.806-1.946 3.42 3.42 0 010-4.438 3.42 3.42 0 00.806-1.946 3.42 3.42 0 013.138-3.138z', label: m.product_detail_specs_availability(), value: inStock ? m.product_detail_specs_in_stock() : m.product_detail_specs_out_of_stock() },
        { icon: 'M3 6l3 1m0 0l-3 9a5.002 5.002 0 006.001 0M6 7l3 9M6 7l6-2m6 2l3-1m-3 1l-3 9a5.002 5.002 0 006.001 0M18 7l3 9m-3-9l-6-2m0-2v2m0 16V5m0 16H9m3 0h3', label: m.product_detail_specs_coverage(), value: m.product_detail_specs_dash() },
        { icon: 'M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z', label: m.product_detail_specs_durability(), value: m.product_detail_specs_dash() }
      ] as spec}
        <div class="flex flex-col items-center gap-2 py-8 px-6 text-center">
          <svg class="w-5 h-5 text-white/50" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d={spec.icon} />
          </svg>
          <span class="text-xl font-bold text-white">{spec.value}</span>
          <span class="text-[11px] uppercase tracking-widest text-white/50">{spec.label}</span>
        </div>
      {/each}
    </div>
  </div>
</div>

<!-- ── TABS ───────────────────────────────────────────────────────── -->
<div class="bg-white">
  <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-16">

    <div class="flex gap-0 border-b border-gray-200 mb-10">
      {#each [
        { id: 'content',  label: m.product_detail_tab_content() },
        { id: 'howto',    label: m.product_detail_tab_howto() },
        { id: 'surfaces', label: m.product_detail_tab_surfaces() }
      ] as tab}
        <button
          onclick={() => (activeTab = tab.id as typeof activeTab)}
          class="px-6 py-4 text-sm font-semibold uppercase tracking-widest border-b-2 transition-colors"
          style={activeTab === tab.id
            ? 'border-color: rgb(51,73,119); color: rgb(25,37,63)'
            : 'border-color: transparent; color: #9ca3af'}
        >
          {tab.label}
        </button>
      {/each}
    </div>

    {#if activeTab === 'content'}
      <div class="max-w-2xl">
        {#if data.product.description}
          <div class="text-gray-700 text-base leading-relaxed">
            {@html `<p class="mb-5 leading-relaxed text-gray-700">${renderMarkdown(data.product.description)}</p>`}
          </div>
        {:else}
          <p class="text-gray-400">{m.product_detail_no_content()}</p>
        {/if}
      </div>

    {:else if activeTab === 'howto'}
      <div class="max-w-2xl flex flex-col gap-10">
        {#each [
          { step: '01', title: m.product_detail_howto_step1_title(), desc: m.product_detail_howto_step1_desc() },
          { step: '02', title: m.product_detail_howto_step2_title(), desc: m.product_detail_howto_step2_desc() },
          { step: '03', title: m.product_detail_howto_step3_title(), desc: m.product_detail_howto_step3_desc() }
        ] as s}
          <div class="flex gap-6 items-start">
            <span class="text-5xl font-black leading-none flex-shrink-0 w-14"
                  style="color: rgb(113,135,183); opacity: 0.6">{s.step}</span>
            <div class="pt-1">
              <h3 class="font-bold text-gray-900 mb-2">{s.title}</h3>
              <p class="text-sm text-gray-500 leading-relaxed">{s.desc}</p>
            </div>
          </div>
        {/each}
      </div>

    {:else if activeTab === 'surfaces'}
      <div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-6 gap-4">
        {#each [
          { name: m.product_detail_surface_paint(),   icon: 'M7 21a4 4 0 01-4-4V5a2 2 0 012-2h4a2 2 0 012 2v12a4 4 0 01-4 4zm0 0h12a2 2 0 002-2v-4a2 2 0 00-2-2h-2.343M11 7.343l1.657-1.657a2 2 0 012.828 0l2.829 2.829a2 2 0 010 2.828l-8.486 8.485M7 17h.01' },
          { name: m.product_detail_surface_glass(),   icon: 'M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17H3a2 2 0 01-2-2V5a2 2 0 012-2h14a2 2 0 012 2v10a2 2 0 01-2 2h-2' },
          { name: m.product_detail_surface_wheels(),  icon: 'M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z' },
          { name: m.product_detail_surface_leather(), icon: 'M5 3v4M3 5h4M6 17v4m-2-2h4m5-16l2.286 6.857L21 12l-5.714 2.143L13 21l-2.286-6.857L5 12l5.714-2.143L13 3z' },
          { name: m.product_detail_surface_trim(),    icon: 'M4 5a1 1 0 011-1h14a1 1 0 011 1v2a1 1 0 01-1 1H5a1 1 0 01-1-1V5zM4 13a1 1 0 011-1h6a1 1 0 011 1v6a1 1 0 01-1 1H5a1 1 0 01-1-1v-6zM16 13a1 1 0 011-1h2a1 1 0 011 1v6a1 1 0 01-1 1h-2a1 1 0 01-1-1v-6z' },
          { name: m.product_detail_surface_fabric(),  icon: 'M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 8a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4m6 6v10m6-2a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4' }
        ] as surface}
          <div class="flex flex-col items-center gap-3 p-5 rounded-2xl border border-gray-100 hover:border-gray-200 hover:bg-gray-50 transition-colors">
            <div class="w-10 h-10 rounded-xl flex items-center justify-center"
                 style="background: rgb(51,73,119)">
              <svg class="w-5 h-5 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d={surface.icon} />
              </svg>
            </div>
            <span class="text-sm font-semibold text-gray-700">{surface.name}</span>
          </div>
        {/each}
      </div>
    {/if}
  </div>
</div>

<!-- ── RELATED ────────────────────────────────────────────────────── -->
{#if data.related.length > 0}
  <div class="bg-gray-50 py-16 border-t border-gray-100">
    <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
      <div class="flex items-center justify-between mb-10">
        <div>
          <p class="text-[11px] font-bold uppercase tracking-[0.25em] mb-2"
             style="color: rgb(113,135,183)">{m.product_detail_related_kicker()}</p>
          <h2 class="text-2xl font-black text-gray-900">{m.product_detail_related_heading()}</h2>
        </div>
        <a href="/products" class="text-sm text-gray-400 hover:text-gray-700 transition-colors">
          {m.common_view_all_arrow()}
        </a>
      </div>

      <div class="grid grid-cols-2 md:grid-cols-4 gap-4">
        {#each data.related as p}
          <a href="/products/{p.slug}"
             class="group bg-white rounded-2xl overflow-hidden border border-gray-100 hover:border-gray-200 hover:shadow-md transition-all">
            <div class="aspect-square bg-gray-50 overflow-hidden group-hover:scale-[1.02] transition-transform duration-500">
              {#if p.primaryImage}
                <img
                  src={p.primaryImage.url}
                  alt={p.primaryImage.alt_text ?? p.name}
                  class="w-full h-full object-cover"
                />
              {:else}
                <div class="w-full h-full flex items-center justify-center text-gray-200">
                  <svg class="w-10 h-10" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1"
                      d="m2.25 15.75 5.159-5.159a2.25 2.25 0 0 1 3.182 0l5.159 5.159m-1.5-1.5 1.409-1.409a2.25 2.25 0 0 1 3.182 0l2.909 2.909m-18 3.75h16.5a1.5 1.5 0 0 0 1.5-1.5V6a1.5 1.5 0 0 0-1.5-1.5H3.75A1.5 1.5 0 0 0 2.25 6v12a1.5 1.5 0 0 0 1.5 1.5Zm10.5-11.25h.008v.008h-.008V8.25Z" />
                  </svg>
                </div>
              {/if}
            </div>
            <div class="p-4">
              <p class="text-sm font-semibold text-gray-900 group-hover:transition-colors leading-snug"
                 style="color: inherit"
                 onmouseenter={(e) => (e.currentTarget.style.color = 'rgb(51,73,119)')}
                 onmouseleave={(e) => (e.currentTarget.style.color = '')}>
                {p.name}
              </p>
            </div>
          </a>
        {/each}
      </div>
    </div>
  </div>
{/if}
