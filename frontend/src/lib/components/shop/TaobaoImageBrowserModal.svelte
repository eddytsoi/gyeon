<script lang="ts">
  /**
   * Taobao-layout secondary image-browser modal (desktop-2 / mobile-2).
   *
   * Pure image carousel of all items the selection modal can pick — one
   * primary image per variant + per promo bundle. Each slide shows that
   * item's name, price (with strikethrough compare-at), and an add-to-cart
   * CTA. Closes on backdrop click or ESC; navigates via on-screen chevrons
   * and touch swipe.
   */
  import { cubicOut } from 'svelte/easing';
  import { cartStore } from '$lib/stores/cart.svelte';
  import { trackAddToCart } from '$lib/tracker';
  import ResponsiveImage from '$lib/components/ResponsiveImage.svelte';

  export type BrowserItem = {
    kind: 'variant' | 'bundle';
    id: string;
    variantId: string;
    name: string;
    image: string | null;
    price: number;
    compareAtPrice: number | null;
    stockQty: number;
  };

  let {
    open = $bindable(false),
    items,
    startIndex = 0,
    productId
  }: {
    open: boolean;
    items: BrowserItem[];
    startIndex?: number;
    productId: string;
  } = $props();

  let index = $state(0);
  // Reset to startIndex every time the modal is reopened so re-entering at
  // slide 4 doesn't get stuck on whatever slide the user last viewed.
  $effect(() => {
    if (open) {
      index = Math.min(Math.max(startIndex, 0), Math.max(items.length - 1, 0));
    }
  });

  let adding = $state(false);
  let added = $state(false);

  const current = $derived(items[index]);

  function close() { open = false; }
  function prev() { if (index > 0) index--; }
  function next() { if (index < items.length - 1) index++; }

  async function addToCart() {
    if (!current || current.stockQty <= 0 || adding) return;
    adding = true;
    try {
      await cartStore.add(current.variantId, 1);
      trackAddToCart({
        id: current.kind === 'variant' ? productId : current.id,
        name: current.name,
        price: current.price,
        quantity: 1
      });
      added = true;
      setTimeout(() => {
        added = false;
        open = false;
      }, 800);
    } finally {
      adding = false;
    }
  }

  function onKey(e: KeyboardEvent) {
    if (!open) return;
    if (e.key === 'Escape') close();
    else if (e.key === 'ArrowLeft') prev();
    else if (e.key === 'ArrowRight') next();
  }

  // Touch swipe — match the existing PDP gallery threshold (40px).
  let touchStartX = 0;
  function onTouchStart(e: TouchEvent) { touchStartX = e.touches[0].clientX; }
  function onTouchEnd(e: TouchEvent) {
    const dx = e.changedTouches[0].clientX - touchStartX;
    if (Math.abs(dx) < 40) return;
    if (dx > 0) prev(); else next();
  }

  $effect(() => {
    if (typeof document === 'undefined') return;
    if (open) {
      const prevOverflow = document.body.style.overflow;
      document.body.style.overflow = 'hidden';
      return () => { document.body.style.overflow = prevOverflow; };
    }
  });

  function fmtMoney(n: number): string {
    return `HK$${Math.round(n) === n ? n : n.toFixed(2)}`;
  }
</script>

<svelte:window onkeydown={onKey} />

{#if open && current}
  <div class="fixed inset-0 z-[300] bg-black/85 flex flex-col"
       role="dialog" aria-modal="true" aria-label="Product image gallery"
       onclick={close}
       onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') close(); }}
       tabindex="-1">
    <!-- Top bar: close + counter -->
    <div class="flex items-center justify-between px-4 sm:px-6 py-4 text-white">
      <button type="button" aria-label="Close" onclick={close}
              class="w-10 h-10 rounded-full bg-white/10 hover:bg-white/20 flex items-center justify-center">
        <svg class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M6 6l12 12M18 6l-12 12"/></svg>
      </button>
      <div class="text-base font-medium text-white/90 tabular-nums">{index + 1} / {items.length}</div>
      <span class="w-10 h-10"></span>
    </div>

    <!-- Image area — stop propagation so taps on the inner area don't dismiss. -->
    <button type="button"
            class="relative flex-1 flex items-center justify-center px-12 sm:px-16 cursor-default"
            onclick={(e) => e.stopPropagation()}
            aria-label="Image area"
            ontouchstart={onTouchStart}
            ontouchend={onTouchEnd}>
      {#if index > 0}
        <button type="button" aria-label="Previous" onclick={(e) => { e.stopPropagation(); prev(); }}
                class="absolute left-3 sm:left-6 top-1/2 -translate-y-1/2 w-10 h-10 rounded-full
                       bg-white/90 hover:bg-white text-ink-900 flex items-center justify-center shadow-card">
          <svg class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M15 18l-6-6 6-6"/></svg>
        </button>
      {/if}
      <div class="max-w-3xl w-full aspect-square bg-white rounded flex items-center justify-center overflow-hidden">
        {#if current.image}
          <ResponsiveImage src={current.image} alt={current.name}
                           widths={[480, 640, 960, 1280]} sizes="(min-width: 1024px) 720px, 100vw"
                           class="w-full h-full object-contain" />
        {/if}
      </div>
      {#if index < items.length - 1}
        <button type="button" aria-label="Next" onclick={(e) => { e.stopPropagation(); next(); }}
                class="absolute right-3 sm:right-6 top-1/2 -translate-y-1/2 w-10 h-10 rounded-full
                       bg-white/90 hover:bg-white text-ink-900 flex items-center justify-center shadow-card">
          <svg class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M9 6l6 6-6 6"/></svg>
        </button>
      {/if}
    </button>

    <!-- Name strip -->
    <div class="px-4 sm:px-6 pt-2 pb-3 text-center text-white text-base sm:text-lg font-medium line-clamp-2">
      {current.name}
    </div>

    <!-- Sticky footer (price + CTA). stopPropagation so taps don't close. -->
    <div class="border-t border-white/10 bg-ink-900/80 backdrop-blur px-4 sm:px-6 py-4 flex items-center gap-4"
         onclick={(e) => e.stopPropagation()}
         onkeydown={(e) => e.stopPropagation()}
         role="toolbar" tabindex="-1">
      <div class="text-white text-lg sm:text-xl font-bold tabular-nums flex items-baseline gap-2">
        {#if current.compareAtPrice != null && current.compareAtPrice > current.price}
          <span class="text-sm sm:text-base text-white/50 line-through">{fmtMoney(current.compareAtPrice)}</span>
        {/if}
        {fmtMoney(current.price)}
      </div>
      <button type="button" onclick={addToCart}
              disabled={current.stockQty <= 0 || adding}
              class="ml-auto px-6 sm:px-8 py-3 bg-navy-500 hover:bg-navy-700 text-white text-sm font-semibold rounded
                     disabled:opacity-50 disabled:cursor-not-allowed">
        {#if added}
          ✓ 已加入購物車
        {:else if adding}
          加入中…
        {:else if current.stockQty <= 0}
          缺貨
        {:else}
          加入購物車
        {/if}
      </button>
    </div>
  </div>
{/if}
