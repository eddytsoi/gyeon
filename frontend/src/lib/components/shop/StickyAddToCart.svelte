<script lang="ts">
  /*
   * Mobile sticky add-to-cart bar — gyeon-project-design-system §4.8.
   * Slides up from the bottom once the in-flow CTA has scrolled out of view.
   * Hidden on lg+ (desktop has the sticky summary column).
   */
  import type { Product, ProductImage, Variant } from '$lib/types';
  import ResponsiveImage from '$lib/components/ResponsiveImage.svelte';
  import { isVideo } from '$lib/media';
  import { formatHKD } from '$lib/money';

  let {
    ctaEl,
    product,
    variant,
    primaryImage,
    onAdd,
    inStock,
    adding,
    added,
    cannotPurchase = false,
    cannotPurchaseLabel = ''
  }: {
    ctaEl: HTMLElement | undefined;
    product: Product;
    variant?: Variant;
    primaryImage?: ProductImage;
    onAdd: () => Promise<void> | void;
    inStock: boolean;
    adding: boolean;
    added: boolean;
    /** Role can't buy from this product. Hides price; button shows the role label. */
    cannotPurchase?: boolean;
    cannotPurchaseLabel?: string;
  } = $props();

  let visible = $state(false);

  $effect(() => {
    if (!ctaEl || typeof IntersectionObserver === 'undefined') return;
    const io = new IntersectionObserver(
      (entries) => {
        // Bar shows once the in-flow CTA is no longer visible (scrolled past).
        const e = entries[0];
        visible = !e.isIntersecting;
      },
      { rootMargin: '0px 0px -20px 0px', threshold: 0 }
    );
    io.observe(ctaEl);
    return () => io.disconnect();
  });
</script>

<div
  class="lg:hidden fixed bottom-0 inset-x-0 z-30 bg-white border-t border-ink-300/60 px-4 py-3
         flex items-center gap-3 shadow-card-hover transition-transform duration-300 ease-gy
         {visible ? 'translate-y-0' : 'translate-y-full'}"
  aria-hidden={!visible}
>
  {#if primaryImage}
    {#if isVideo(primaryImage)}
      {#if primaryImage.thumbnail_url}
        <ResponsiveImage src={primaryImage.thumbnail_url}
                         alt={primaryImage.alt_text ?? product.name}
                         widths={[160, 320]} sizes="48px"
                         class="w-12 h-12 rounded-md object-cover bg-black" />
      {:else}
        <div class="w-12 h-12 rounded-md bg-black"></div>
      {/if}
    {:else}
      <ResponsiveImage src={primaryImage.url}
                       alt={primaryImage.alt_text ?? product.name}
                       widths={[160, 320]} sizes="48px"
                       class="w-12 h-12 rounded-md object-cover bg-paper" />
    {/if}
  {:else}
    <div class="w-12 h-12 rounded-md bg-paper"></div>
  {/if}

  <div class="flex-1 min-w-0">
    <p class="font-display text-lg font-medium text-ink-500 line-clamp-1 uppercase">{product.name}</p>
    {#if variant && !cannotPurchase}
      <!-- Proportional (not tabular) figures: matches the hero price — GT America
           Compressed's tabular digits read loose, and a single price needs no
           column alignment. -->
      <p class="font-display text-xl font-medium text-navy-500">
        {formatHKD(variant.price)}
      </p>
    {/if}
  </div>

  <button
    type="button"
    onclick={onAdd}
    disabled={!inStock || adding}
    class="flex-shrink-0 h-11 px-5 rounded-md font-display font-bold text-sm uppercase tracking-[0.1em] text-white transition-all duration-200 ease-gy
           {!inStock
             ? 'bg-ink-300 cursor-not-allowed'
             : added
               ? 'bg-success'
               : 'bg-navy-500 hover:bg-navy-700 active:scale-[0.98]'}"
  >
    {#if cannotPurchase}
      {cannotPurchaseLabel || '不可購買'}
    {:else}
      {added ? '已加入' : adding ? '加入中' : '加入'}
    {/if}
  </button>
</div>
